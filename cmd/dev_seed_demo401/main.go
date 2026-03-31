package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	internalstore "github.com/router-for-me/CLIProxyAPI/v6/internal/store"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "path to config file")
	flag.Parse()

	cfg, err := internalconfig.LoadConfig(*configPath)
	if err != nil {
		panic(err)
	}
	if strings.TrimSpace(cfg.MongoStore.URI) == "" {
		panic("mongo-store.uri is empty in config")
	}
	if strings.TrimSpace(cfg.MongoStore.Database) == "" {
		panic("mongo-store.database is empty in config")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	store, err := internalstore.NewMongoStore(ctx, internalstore.MongoStoreConfig{
		URI:        cfg.MongoStore.URI,
		Database:   cfg.MongoStore.Database,
		Collection: cfg.MongoStore.Collection,
	})
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = store.Close(context.Background())
	}()

	repoRoot, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	viewJSONPath := filepath.Join(repoRoot, "tmp.test-401-codex.json")
	viewRaw, err := os.ReadFile(viewJSONPath)
	if err != nil {
		panic(err)
	}
	viewMetadata := map[string]any{}
	if err := json.Unmarshal(viewRaw, &viewMetadata); err != nil {
		panic(err)
	}

	now := time.Now().UTC()
	mockBaseURL := "http://127.0.0.1:19191"
	records := []*coreauth.Auth{
		{
			ID:       "tmp.test-401-proof-codex.json",
			FileName: "tmp.test-401-proof-codex.json",
			Provider: "codex",
			Prefix:   "demo401proof",
			Label:    "demo401-proof@example.com",
			Status:   coreauth.StatusActive,
			Metadata: map[string]any{
				"type":  "codex",
				"email": "demo401-proof@example.com",
				"note":  "DEMO 401 proof account",
			},
			Attributes: map[string]string{
				"api_key":  "demo401-proof-invalid-api-key",
				"base_url": mockBaseURL,
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:       "tmp.test-401-codex.json",
			FileName: "tmp.test-401-codex.json",
			Provider: "codex",
			Prefix:   "demo401view",
			Label:    "demo401-view@example.com",
			Status:   coreauth.StatusActive,
			Metadata: viewMetadata,
			Attributes: map[string]string{
				"api_key":  "demo401-view-invalid-api-key",
				"base_url": mockBaseURL,
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	for _, record := range records {
		if _, err := store.Save(ctx, record); err != nil {
			panic(err)
		}
		fmt.Printf("seeded %s with prefix %s\n", record.FileName, record.Prefix)
	}
}
