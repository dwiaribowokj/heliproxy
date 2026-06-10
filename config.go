package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultHost             = "0.0.0.0"
	defaultPort             = 18081
	defaultRPCBaseURL       = "https://mainnet.helius-rpc.com/"
	defaultRestBaseURL      = "https://api.helius.xyz"
	defaultAdminBaseURL     = "https://admin-api.helius.xyz/v0"
	defaultStickyLimit      = 3
	defaultCooldownSeconds  = 60
	defaultTimeoutSeconds   = 30
	defaultMaxBodyBytes     = 32 << 20
	defaultConfigFileName   = "config.yaml"
	generatedKeyByteLength  = 24
	heliusKeyIDByteLength   = 8
	maskedSecretPlaceholder = "********"
)

type Config struct {
	Server  ServerConfig  `yaml:"server" json:"server"`
	Auth    AuthConfig    `yaml:"auth" json:"auth"`
	Helius  HeliusConfig  `yaml:"helius" json:"helius"`
	Routing RoutingConfig `yaml:"routing" json:"routing"`
}

type ServerConfig struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

type AuthConfig struct {
	ClientKeys []string `yaml:"client_keys" json:"client_keys"`
	AdminKeys  []string `yaml:"admin_keys" json:"admin_keys"`
}

type HeliusConfig struct {
	RPCBaseURL   string      `yaml:"rpc_base_url" json:"rpc_base_url"`
	RestBaseURL  string      `yaml:"rest_base_url" json:"rest_base_url"`
	AdminBaseURL string      `yaml:"admin_base_url" json:"admin_base_url"`
	Keys         []HeliusKey `yaml:"keys" json:"keys"`
}

type HeliusKey struct {
	ID        string `yaml:"id" json:"id"`
	Name      string `yaml:"name" json:"name"`
	APIKey    string `yaml:"api_key" json:"api_key,omitempty"`
	ProjectID string `yaml:"project_id" json:"project_id"`
	Enabled   bool   `yaml:"enabled" json:"enabled"`
}

type RoutingConfig struct {
	StickyRoundRobinLimit int `yaml:"sticky_round_robin_limit" json:"sticky_round_robin_limit"`
	CooldownSeconds       int `yaml:"cooldown_seconds" json:"cooldown_seconds"`
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds" json:"request_timeout_seconds"`
	MaxBodyBytes          int `yaml:"max_body_bytes" json:"max_body_bytes"`
}

type ConfigStore struct {
	Path string
}

type ConfigLoadResult struct {
	Config        *Config
	Created       bool
	GeneratedAuth bool
}

type PublicConfig struct {
	Server  ServerConfig       `json:"server"`
	Auth    AuthConfig         `json:"auth"`
	Helius  PublicHeliusConfig `json:"helius"`
	Routing RoutingConfig      `json:"routing"`
	Meta    PublicConfigMeta   `json:"meta"`
}

type PublicHeliusConfig struct {
	RPCBaseURL   string            `json:"rpc_base_url"`
	RestBaseURL  string            `json:"rest_base_url"`
	AdminBaseURL string            `json:"admin_base_url"`
	Keys         []PublicHeliusKey `json:"keys"`
}

type PublicHeliusKey struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	APIKeyMasked string `json:"api_key_masked"`
	ProjectID    string `json:"project_id"`
	Enabled      bool   `json:"enabled"`
}

type PublicConfigMeta struct {
	ConfigPath string `json:"config_path"`
}

func resolveConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("HELIPROXY_CONFIG")); v != "" {
		return v
	}
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		dataDir = strings.TrimSpace(os.Getenv("HELIPROXY_DATA_DIR"))
	}
	if dataDir == "" {
		dataDir = "./data"
	}
	return filepath.Join(dataDir, defaultConfigFileName)
}

func NewConfigStore(path string) *ConfigStore {
	return &ConfigStore{Path: path}
}

func (s *ConfigStore) LoadOrCreate() (*ConfigLoadResult, error) {
	if s.Path == "" {
		return nil, errors.New("config path is empty")
	}

	data, err := os.ReadFile(s.Path)
	if err == nil {
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
		applyConfigDefaults(&cfg)
		changed := normalizeConfig(&cfg)
		if err := validateConfig(&cfg); err != nil {
			return nil, err
		}
		if changed {
			if err := s.Save(&cfg); err != nil {
				return nil, err
			}
		}
		return &ConfigLoadResult{Config: &cfg}, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg, generatedAuth, err := defaultConfigFromEnv()
	if err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	if err := s.Save(cfg); err != nil {
		return nil, err
	}
	return &ConfigLoadResult{Config: cfg, Created: true, GeneratedAuth: generatedAuth}, nil
}

func (s *ConfigStore) Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	applyConfigDefaults(cfg)
	normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config temp: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func defaultConfigFromEnv() (*Config, bool, error) {
	clientKeys := splitEnvList("HELIPROXY_CLIENT_KEYS", "HELIPROXY_CLIENT_KEY")
	adminKeys := splitEnvList("HELIPROXY_ADMIN_KEYS", "HELIPROXY_ADMIN_KEY")
	generatedAuth := false
	if len(clientKeys) == 0 {
		key, err := randomToken(generatedKeyByteLength)
		if err != nil {
			return nil, false, err
		}
		clientKeys = []string{key}
		generatedAuth = true
	}
	if len(adminKeys) == 0 {
		key, err := randomToken(generatedKeyByteLength)
		if err != nil {
			return nil, false, err
		}
		adminKeys = []string{key}
		generatedAuth = true
	}

	heliusKeys := splitEnvList("HELIUS_API_KEYS", "HELIUS_API_KEY")
	projectIDs := splitEnvList("HELIUS_PROJECT_IDS", "HELIUS_PROJECT_ID")
	names := splitEnvList("HELIUS_KEY_NAMES", "HELIUS_KEY_NAME")
	keys := make([]HeliusKey, 0, len(heliusKeys))
	for i, apiKey := range heliusKeys {
		id, err := randomHeliusKeyID()
		if err != nil {
			return nil, false, err
		}
		name := fmt.Sprintf("helius-%d", i+1)
		if i < len(names) && names[i] != "" {
			name = names[i]
		}
		projectID := ""
		if i < len(projectIDs) {
			projectID = projectIDs[i]
		}
		keys = append(keys, HeliusKey{ID: id, Name: name, APIKey: apiKey, ProjectID: projectID, Enabled: true})
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: envString("HELIPROXY_HOST", defaultHost),
			Port: envInt([]string{"HELIPROXY_PORT", "PORT"}, defaultPort),
		},
		Auth: AuthConfig{
			ClientKeys: clientKeys,
			AdminKeys:  adminKeys,
		},
		Helius: HeliusConfig{
			RPCBaseURL:   envString("HELIUS_RPC_URL", defaultRPCBaseURL),
			RestBaseURL:  envString("HELIUS_REST_URL", defaultRestBaseURL),
			AdminBaseURL: envString("HELIUS_ADMIN_URL", defaultAdminBaseURL),
			Keys:         keys,
		},
		Routing: RoutingConfig{
			StickyRoundRobinLimit: envInt([]string{"HELIPROXY_STICKY_LIMIT"}, defaultStickyLimit),
			CooldownSeconds:       envInt([]string{"HELIPROXY_COOLDOWN_SECONDS"}, defaultCooldownSeconds),
			RequestTimeoutSeconds: envInt([]string{"HELIPROXY_REQUEST_TIMEOUT_SECONDS"}, defaultTimeoutSeconds),
			MaxBodyBytes:          envInt([]string{"HELIPROXY_MAX_BODY_BYTES"}, defaultMaxBodyBytes),
		},
	}
	applyConfigDefaults(cfg)
	normalizeConfig(cfg)
	return cfg, generatedAuth, nil
}

func applyConfigDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.Server.Host) == "" {
		cfg.Server.Host = defaultHost
	}
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = defaultPort
	}
	if strings.TrimSpace(cfg.Helius.RPCBaseURL) == "" {
		cfg.Helius.RPCBaseURL = defaultRPCBaseURL
	}
	if strings.TrimSpace(cfg.Helius.RestBaseURL) == "" {
		cfg.Helius.RestBaseURL = defaultRestBaseURL
	}
	if strings.TrimSpace(cfg.Helius.AdminBaseURL) == "" {
		cfg.Helius.AdminBaseURL = defaultAdminBaseURL
	}
	if cfg.Routing.StickyRoundRobinLimit <= 0 {
		cfg.Routing.StickyRoundRobinLimit = defaultStickyLimit
	}
	if cfg.Routing.CooldownSeconds <= 0 {
		cfg.Routing.CooldownSeconds = defaultCooldownSeconds
	}
	if cfg.Routing.RequestTimeoutSeconds <= 0 {
		cfg.Routing.RequestTimeoutSeconds = defaultTimeoutSeconds
	}
	if cfg.Routing.MaxBodyBytes <= 0 {
		cfg.Routing.MaxBodyBytes = defaultMaxBodyBytes
	}
}

func normalizeConfig(cfg *Config) bool {
	changed := false
	cfg.Server.Host = strings.TrimSpace(cfg.Server.Host)
	cfg.Helius.RPCBaseURL = strings.TrimSpace(cfg.Helius.RPCBaseURL)
	cfg.Helius.RestBaseURL = strings.TrimRight(strings.TrimSpace(cfg.Helius.RestBaseURL), "/")
	cfg.Helius.AdminBaseURL = strings.TrimRight(strings.TrimSpace(cfg.Helius.AdminBaseURL), "/")
	cfg.Auth.ClientKeys = compactUniqueStrings(cfg.Auth.ClientKeys)
	cfg.Auth.AdminKeys = compactUniqueStrings(cfg.Auth.AdminKeys)

	seenIDs := map[string]struct{}{}
	keys := cfg.Helius.Keys[:0]
	for i := range cfg.Helius.Keys {
		k := cfg.Helius.Keys[i]
		k.ID = strings.TrimSpace(k.ID)
		k.Name = strings.TrimSpace(k.Name)
		k.APIKey = strings.TrimSpace(k.APIKey)
		k.ProjectID = strings.TrimSpace(k.ProjectID)
		if k.APIKey == "" {
			changed = true
			continue
		}
		if k.ID == "" {
			id, err := randomHeliusKeyID()
			if err == nil {
				k.ID = id
				changed = true
			}
		}
		if _, exists := seenIDs[k.ID]; exists {
			id, err := randomHeliusKeyID()
			if err == nil {
				k.ID = id
				changed = true
			}
		}
		seenIDs[k.ID] = struct{}{}
		if k.Name == "" {
			k.Name = fmt.Sprintf("helius-%d", len(keys)+1)
			changed = true
		}
		keys = append(keys, k)
	}
	cfg.Helius.Keys = keys
	return changed
}

func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	if len(cfg.Auth.ClientKeys) == 0 {
		return fmt.Errorf("auth.client_keys must contain at least one key")
	}
	if len(cfg.Auth.AdminKeys) == 0 {
		return fmt.Errorf("auth.admin_keys must contain at least one key")
	}
	if _, err := url.ParseRequestURI(cfg.Helius.RPCBaseURL); err != nil {
		return fmt.Errorf("helius.rpc_base_url is invalid: %w", err)
	}
	if _, err := url.ParseRequestURI(cfg.Helius.RestBaseURL); err != nil {
		return fmt.Errorf("helius.rest_base_url is invalid: %w", err)
	}
	if _, err := url.ParseRequestURI(cfg.Helius.AdminBaseURL); err != nil {
		return fmt.Errorf("helius.admin_base_url is invalid: %w", err)
	}
	if cfg.Routing.StickyRoundRobinLimit <= 0 {
		return fmt.Errorf("routing.sticky_round_robin_limit must be positive")
	}
	if cfg.Routing.CooldownSeconds <= 0 {
		return fmt.Errorf("routing.cooldown_seconds must be positive")
	}
	if cfg.Routing.RequestTimeoutSeconds <= 0 {
		return fmt.Errorf("routing.request_timeout_seconds must be positive")
	}
	if cfg.Routing.MaxBodyBytes <= 0 {
		return fmt.Errorf("routing.max_body_bytes must be positive")
	}
	for _, k := range cfg.Helius.Keys {
		if strings.TrimSpace(k.ID) == "" {
			return fmt.Errorf("helius key id cannot be empty")
		}
		if strings.TrimSpace(k.APIKey) == "" {
			return fmt.Errorf("helius key %q api_key cannot be empty", k.Name)
		}
	}
	return nil
}

func mergeConfigUpdate(current *Config, incoming *Config) (*Config, error) {
	if incoming == nil {
		return nil, errors.New("incoming config is nil")
	}
	merged := cloneConfig(incoming)
	applyConfigDefaults(merged)

	oldByID := map[string]HeliusKey{}
	if current != nil {
		for _, k := range current.Helius.Keys {
			oldByID[k.ID] = k
		}
	}
	for i := range merged.Helius.Keys {
		k := &merged.Helius.Keys[i]
		k.ID = strings.TrimSpace(k.ID)
		k.APIKey = strings.TrimSpace(k.APIKey)
		if old, ok := oldByID[k.ID]; ok && (k.APIKey == "" || strings.Contains(k.APIKey, "*")) {
			k.APIKey = old.APIKey
		}
		if k.ID == "" {
			id, err := randomHeliusKeyID()
			if err != nil {
				return nil, err
			}
			k.ID = id
		}
	}
	normalizeConfig(merged)
	if err := validateConfig(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func cloneConfig(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.Auth.ClientKeys = append([]string(nil), cfg.Auth.ClientKeys...)
	out.Auth.AdminKeys = append([]string(nil), cfg.Auth.AdminKeys...)
	out.Helius.Keys = append([]HeliusKey(nil), cfg.Helius.Keys...)
	return &out
}

func publicConfig(cfg *Config, configPath string) PublicConfig {
	pub := PublicConfig{
		Server:  cfg.Server,
		Auth:    AuthConfig{ClientKeys: append([]string(nil), cfg.Auth.ClientKeys...), AdminKeys: append([]string(nil), cfg.Auth.AdminKeys...)},
		Routing: cfg.Routing,
		Helius: PublicHeliusConfig{
			RPCBaseURL:   cfg.Helius.RPCBaseURL,
			RestBaseURL:  cfg.Helius.RestBaseURL,
			AdminBaseURL: cfg.Helius.AdminBaseURL,
			Keys:         make([]PublicHeliusKey, 0, len(cfg.Helius.Keys)),
		},
		Meta: PublicConfigMeta{ConfigPath: configPath},
	}
	for _, k := range cfg.Helius.Keys {
		pub.Helius.Keys = append(pub.Helius.Keys, PublicHeliusKey{
			ID:           k.ID,
			Name:         k.Name,
			APIKeyMasked: maskSecret(k.APIKey),
			ProjectID:    k.ProjectID,
			Enabled:      k.Enabled,
		})
	}
	return pub
}

func splitEnvList(keys ...string) []string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return splitList(v)
		}
	}
	return nil
}

func splitList(v string) []string {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == '\n' || r == ';' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func compactUniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(keys []string, fallback int) int {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			parsed, err := strconv.Atoi(v)
			if err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return fallback
}

func randomHeliusKeyID() (string, error) {
	return randomToken(heliusKeyIDByteLength)
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func maskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 8 {
		return maskedSecretPlaceholder
	}
	return secret[:4] + strings.Repeat("*", 8) + secret[len(secret)-4:]
}

func timeoutDuration(cfg *Config) time.Duration {
	seconds := defaultTimeoutSeconds
	if cfg != nil && cfg.Routing.RequestTimeoutSeconds > 0 {
		seconds = cfg.Routing.RequestTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}
