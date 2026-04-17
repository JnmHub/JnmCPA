package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_SQLiteStore(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
sqlite-store:
  path: "  ./data/cliproxy.db  "
  poll-interval-seconds:  9
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if got := cfg.SQLiteStore.Path; got != "./data/cliproxy.db" {
		t.Fatalf("Path = %q, want %q", got, "./data/cliproxy.db")
	}
	if got := cfg.SQLiteStore.PollIntervalSeconds; got != 9 {
		t.Fatalf("PollIntervalSeconds = %d, want %d", got, 9)
	}
}
