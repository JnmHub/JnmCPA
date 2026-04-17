package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestSQLiteStoreSaveListDelete(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cliproxy.db")

	store, err := NewSQLiteStore(ctx, SQLiteStoreConfig{
		Path:         dbPath,
		PollInterval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{
			"email": "sqlite-save@example.com",
		},
		Attributes: map[string]string{
			"path": "/tmp/sqlite-save.json",
		},
	}

	location, err := store.Save(ctx, auth)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if location == "" {
		t.Fatalf("Save() location is empty")
	}
	if auth.ID == "" {
		t.Fatalf("Save() did not assign auth ID")
	}

	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() len = %d, want 1", len(list))
	}
	if got := list[0].Attributes["storage_backend"]; got != "sqlite" {
		t.Fatalf("storage_backend = %q, want %q", got, "sqlite")
	}
	if got := list[0].ID; got != auth.ID {
		t.Fatalf("listed auth ID = %q, want %q", got, auth.ID)
	}

	if err := store.Delete(ctx, auth.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	list, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List() after delete len = %d, want 0", len(list))
	}
}

func TestSQLiteStoreSubscribeAuthChanges(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "cliproxy.db")

	observer, err := NewSQLiteStore(ctx, SQLiteStoreConfig{
		Path:         dbPath,
		PollInterval: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore(observer) error = %v", err)
	}
	defer func() {
		_ = observer.Close()
	}()

	writer, err := NewSQLiteStore(ctx, SQLiteStoreConfig{
		Path:         dbPath,
		PollInterval: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore(writer) error = %v", err)
	}
	defer func() {
		_ = writer.Close()
	}()

	changes := make(chan sdkAuth.AuthChange, 8)
	subscribeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_ = observer.SubscribeAuthChanges(subscribeCtx, func(change sdkAuth.AuthChange) {
			changes <- change
		})
	}()
	time.Sleep(150 * time.Millisecond)

	auth := &cliproxyauth.Auth{
		ID:       "sqlite-subscribe.json",
		Provider: "codex",
		FileName: "sqlite-subscribe.json",
		Metadata: map[string]any{
			"email": "sqlite-subscribe@example.com",
		},
	}

	if _, err := writer.Save(ctx, auth); err != nil {
		t.Fatalf("writer.Save() error = %v", err)
	}

	upsert := awaitAuthChange(t, changes, sdkAuth.AuthChangeActionUpsert)
	if upsert.ID != auth.ID {
		t.Fatalf("upsert ID = %q, want %q", upsert.ID, auth.ID)
	}
	if upsert.Auth == nil || upsert.Auth.ID != auth.ID {
		t.Fatalf("upsert auth = %#v, want auth %q", upsert.Auth, auth.ID)
	}

	if err := writer.Delete(ctx, auth.ID); err != nil {
		t.Fatalf("writer.Delete() error = %v", err)
	}

	deletion := awaitAuthChange(t, changes, sdkAuth.AuthChangeActionDelete)
	if deletion.ID != auth.ID {
		t.Fatalf("delete ID = %q, want %q", deletion.ID, auth.ID)
	}
}

func awaitAuthChange(t *testing.T, changes <-chan sdkAuth.AuthChange, want sdkAuth.AuthChangeAction) sdkAuth.AuthChange {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case change := <-changes:
			if change.Action == want {
				return change
			}
		case <-deadline:
			t.Fatalf("timed out waiting for auth change action %q", want)
		}
	}
}
