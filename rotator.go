package main

import (
	"sort"
	"sync"
	"time"
)

type KeyManager struct {
	mu              sync.Mutex
	cfg             *Config
	states          map[string]*KeyState
	cursor          int
	consecutiveUses int
}

type KeyState struct {
	RequestCount   int64         `json:"request_count"`
	SuccessCount   int64         `json:"success_count"`
	FailureCount   int64         `json:"failure_count"`
	LastUsedAt     *time.Time    `json:"last_used_at,omitempty"`
	LastSuccessAt  *time.Time    `json:"last_success_at,omitempty"`
	LastFailureAt  *time.Time    `json:"last_failure_at,omitempty"`
	CooldownUntil  *time.Time    `json:"cooldown_until,omitempty"`
	LastError      string        `json:"last_error,omitempty"`
	LastStatus     int           `json:"last_status,omitempty"`
	LastLatencyMS  int64         `json:"last_latency_ms,omitempty"`
	Usage          *ProjectUsage `json:"usage,omitempty"`
	UsageFetchedAt *time.Time    `json:"usage_fetched_at,omitempty"`
	UsageError     string        `json:"usage_error,omitempty"`
}

type KeyAttempt struct {
	Key HeliusKey
}

type KeyStatus struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	APIKeyMasked  string        `json:"api_key_masked"`
	ProjectID     string        `json:"project_id"`
	Enabled       bool          `json:"enabled"`
	Available     bool          `json:"available"`
	CooldownUntil *time.Time    `json:"cooldown_until,omitempty"`
	State         KeyState      `json:"state"`
	Usage         *ProjectUsage `json:"usage,omitempty"`
}

type ManagerStatus struct {
	ConfiguredKeys int         `json:"configured_keys"`
	EnabledKeys    int         `json:"enabled_keys"`
	AvailableKeys  int         `json:"available_keys"`
	Cursor         int         `json:"cursor"`
	StickyLimit    int         `json:"sticky_limit"`
	Keys           []KeyStatus `json:"keys"`
}

func NewKeyManager(cfg *Config) *KeyManager {
	m := &KeyManager{
		cfg:    cloneConfig(cfg),
		states: map[string]*KeyState{},
	}
	m.ensureStatesLocked()
	return m
}

func (m *KeyManager) Config() *Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneConfig(m.cfg)
}

func (m *KeyManager) UpdateConfig(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cloneConfig(cfg)
	m.ensureStatesLocked()
	if len(m.cfg.Helius.Keys) == 0 {
		m.cursor = 0
		m.consecutiveUses = 0
	} else if m.cursor >= len(m.cfg.Helius.Keys) {
		m.cursor = 0
		m.consecutiveUses = 0
	}
}

func (m *KeyManager) NextAttempts(now time.Time) []KeyAttempt {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureStatesLocked()
	if m.cfg == nil || len(m.cfg.Helius.Keys) == 0 {
		return nil
	}

	available := m.availableIndexesLocked(now)
	if len(available) == 0 {
		return nil
	}

	stickyLimit := normalizedStickyLimit(m.cfg.Routing.StickyRoundRobinLimit)
	selected := -1
	if m.cursor >= 0 && m.cursor < len(m.cfg.Helius.Keys) && m.indexAvailableLocked(m.cursor, now) && m.consecutiveUses < stickyLimit {
		selected = m.cursor
	} else {
		selected = m.nextAvailableAfterLocked(m.cursor, now)
		m.cursor = selected
		m.consecutiveUses = 0
	}
	if selected < 0 {
		return nil
	}

	m.consecutiveUses++
	ordered := []int{selected}
	for i := 1; i < len(m.cfg.Helius.Keys); i++ {
		idx := (selected + i) % len(m.cfg.Helius.Keys)
		if m.indexAvailableLocked(idx, now) {
			ordered = append(ordered, idx)
		}
	}

	attempts := make([]KeyAttempt, 0, len(ordered))
	for _, idx := range ordered {
		attempts = append(attempts, KeyAttempt{Key: m.cfg.Helius.Keys[idx]})
	}
	return attempts
}

func (m *KeyManager) MarkAttempt(id string, success bool, status int, message string, latency time.Duration, setCooldown bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureStatesLocked()
	state := m.stateLocked(id)
	now := time.Now().UTC()
	state.RequestCount++
	state.LastUsedAt = ptrTime(now)
	state.LastStatus = status
	state.LastLatencyMS = latency.Milliseconds()
	if success {
		state.SuccessCount++
		state.LastSuccessAt = ptrTime(now)
		state.LastError = ""
		state.UsageError = state.UsageError
		return
	}
	state.FailureCount++
	state.LastFailureAt = ptrTime(now)
	state.LastError = message
	if setCooldown {
		cooldown := defaultCooldownSeconds
		if m.cfg != nil && m.cfg.Routing.CooldownSeconds > 0 {
			cooldown = m.cfg.Routing.CooldownSeconds
		}
		until := now.Add(time.Duration(cooldown) * time.Second)
		state.CooldownUntil = ptrTime(until)
	}
}

func (m *KeyManager) MarkUsage(id string, usage *ProjectUsage, errMessage string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureStatesLocked()
	state := m.stateLocked(id)
	now := time.Now().UTC()
	state.UsageFetchedAt = ptrTime(now)
	state.UsageError = errMessage
	if usage != nil {
		state.Usage = usage
	}
}

func (m *KeyManager) KeysForUsage() []HeliusKey {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]HeliusKey, 0, len(m.cfg.Helius.Keys))
	for _, k := range m.cfg.Helius.Keys {
		out = append(out, k)
	}
	return out
}

func (m *KeyManager) Status(now time.Time) ManagerStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureStatesLocked()
	status := ManagerStatus{Cursor: m.cursor, StickyLimit: normalizedStickyLimit(m.cfg.Routing.StickyRoundRobinLimit)}
	if m.cfg == nil {
		return status
	}
	status.ConfiguredKeys = len(m.cfg.Helius.Keys)
	status.Keys = make([]KeyStatus, 0, len(m.cfg.Helius.Keys))
	for _, k := range m.cfg.Helius.Keys {
		state := m.stateValueLocked(k.ID)
		available := k.Enabled && !isCoolingDown(state.CooldownUntil, now)
		if k.Enabled {
			status.EnabledKeys++
		}
		if available {
			status.AvailableKeys++
		}
		status.Keys = append(status.Keys, KeyStatus{
			ID:            k.ID,
			Name:          k.Name,
			APIKeyMasked:  maskSecret(k.APIKey),
			ProjectID:     k.ProjectID,
			Enabled:       k.Enabled,
			Available:     available,
			CooldownUntil: state.CooldownUntil,
			State:         state,
			Usage:         state.Usage,
		})
	}
	sort.SliceStable(status.Keys, func(i, j int) bool { return status.Keys[i].Name < status.Keys[j].Name })
	return status
}

func (m *KeyManager) availableIndexesLocked(now time.Time) []int {
	indexes := []int{}
	for i := range m.cfg.Helius.Keys {
		if m.indexAvailableLocked(i, now) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m *KeyManager) indexAvailableLocked(idx int, now time.Time) bool {
	if idx < 0 || idx >= len(m.cfg.Helius.Keys) {
		return false
	}
	key := m.cfg.Helius.Keys[idx]
	if !key.Enabled {
		return false
	}
	state := m.stateLocked(key.ID)
	return !isCoolingDown(state.CooldownUntil, now)
}

func (m *KeyManager) nextAvailableAfterLocked(start int, now time.Time) int {
	if len(m.cfg.Helius.Keys) == 0 {
		return -1
	}
	for i := 1; i <= len(m.cfg.Helius.Keys); i++ {
		idx := (start + i) % len(m.cfg.Helius.Keys)
		if idx < 0 {
			idx += len(m.cfg.Helius.Keys)
		}
		if m.indexAvailableLocked(idx, now) {
			return idx
		}
	}
	return -1
}

func (m *KeyManager) ensureStatesLocked() {
	if m.states == nil {
		m.states = map[string]*KeyState{}
	}
	if m.cfg == nil {
		return
	}
	valid := map[string]struct{}{}
	for _, k := range m.cfg.Helius.Keys {
		valid[k.ID] = struct{}{}
		if _, ok := m.states[k.ID]; !ok {
			m.states[k.ID] = &KeyState{}
		}
	}
	for id := range m.states {
		if _, ok := valid[id]; !ok {
			delete(m.states, id)
		}
	}
}

func (m *KeyManager) stateLocked(id string) *KeyState {
	state := m.states[id]
	if state == nil {
		state = &KeyState{}
		m.states[id] = state
	}
	return state
}

func (m *KeyManager) stateValueLocked(id string) KeyState {
	state := m.stateLocked(id)
	clone := *state
	return clone
}

func normalizedStickyLimit(limit int) int {
	if limit <= 0 {
		return 1
	}
	return limit
}

func isCoolingDown(until *time.Time, now time.Time) bool {
	if until == nil {
		return false
	}
	return until.After(now)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
