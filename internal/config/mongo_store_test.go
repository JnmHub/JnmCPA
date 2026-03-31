package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigOptional_MongoStore(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	configYAML := []byte(`
mongo-store:
  uri: "  mongodb://127.0.0.1:27018  "
  database: "  cliproxy  "
  collection: "  auth_store  "
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadConfigOptional(configPath, false)
	if err != nil {
		t.Fatalf("LoadConfigOptional() error = %v", err)
	}

	if got := cfg.MongoStore.URI; got != "mongodb://127.0.0.1:27018" {
		t.Fatalf("URI = %q, want %q", got, "mongodb://127.0.0.1:27018")
	}
	if got := cfg.MongoStore.Database; got != "cliproxy" {
		t.Fatalf("Database = %q, want %q", got, "cliproxy")
	}
	if got := cfg.MongoStore.Collection; got != "auth_store" {
		t.Fatalf("Collection = %q, want %q", got, "auth_store")
	}
}
