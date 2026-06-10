package main

import (
	"testing"
	"time"
)

func TestRotatorStickyOne(t *testing.T) {
	m := NewKeyManager(testConfigWithKeys(3, 1))
	choices := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		attempts := m.NextAttempts(time.Now())
		if len(attempts) == 0 {
			t.Fatal("no attempts")
		}
		choices = append(choices, attempts[0].Key.Name)
	}
	want := []string{"key-1", "key-2", "key-3", "key-1", "key-2"}
	for i := range want {
		if choices[i] != want[i] {
			t.Fatalf("choice %d = %s, want %s; all=%v", i, choices[i], want[i], choices)
		}
	}
}

func TestRotatorStickyThree(t *testing.T) {
	m := NewKeyManager(testConfigWithKeys(2, 3))
	choices := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		attempts := m.NextAttempts(time.Now())
		if len(attempts) == 0 {
			t.Fatal("no attempts")
		}
		choices = append(choices, attempts[0].Key.Name)
	}
	want := []string{"key-1", "key-1", "key-1", "key-2", "key-2", "key-2", "key-1", "key-1"}
	for i := range want {
		if choices[i] != want[i] {
			t.Fatalf("choice %d = %s, want %s; all=%v", i, choices[i], want[i], choices)
		}
	}
}

func TestRotatorSkipsDisabledAndCooldown(t *testing.T) {
	cfg := testConfigWithKeys(3, 1)
	cfg.Helius.Keys[0].Enabled = false
	m := NewKeyManager(cfg)
	m.MarkAttempt(cfg.Helius.Keys[1].ID, false, 429, "rate limited", time.Millisecond, true)
	attempts := m.NextAttempts(time.Now())
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d, want 1", len(attempts))
	}
	if attempts[0].Key.Name != "key-3" {
		t.Fatalf("selected %s, want key-3", attempts[0].Key.Name)
	}
}

func TestRotatorConcurrentSelection(t *testing.T) {
	m := NewKeyManager(testConfigWithKeys(5, 2))
	errCh := make(chan error, 100)
	for i := 0; i < 100; i++ {
		go func() {
			attempts := m.NextAttempts(time.Now())
			if len(attempts) == 0 {
				errCh <- errNoAttempts{}
				return
			}
			errCh <- nil
		}()
	}
	for i := 0; i < 100; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

type errNoAttempts struct{}

func (errNoAttempts) Error() string { return "no attempts" }

func testConfigWithKeys(count, sticky int) *Config {
	cfg := &Config{
		Server: ServerConfig{Host: "127.0.0.1", Port: defaultPort},
		Auth: AuthConfig{
			ClientKeys: []string{"client"},
			AdminKeys:  []string{"admin"},
		},
		Helius: HeliusConfig{
			RPCBaseURL:   defaultRPCBaseURL,
			RestBaseURL:  defaultRestBaseURL,
			AdminBaseURL: defaultAdminBaseURL,
		},
		Routing: RoutingConfig{
			StickyRoundRobinLimit: sticky,
			CooldownSeconds:       defaultCooldownSeconds,
			RequestTimeoutSeconds: defaultTimeoutSeconds,
			MaxBodyBytes:          defaultMaxBodyBytes,
		},
	}
	for i := 0; i < count; i++ {
		cfg.Helius.Keys = append(cfg.Helius.Keys, HeliusKey{
			ID:      string(rune('a' + i)),
			Name:    "key-" + string(rune('1'+i)),
			APIKey:  "helius-" + string(rune('1'+i)),
			Enabled: true,
		})
	}
	return cfg
}
