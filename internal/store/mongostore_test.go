package store

import (
	"context"
	"encoding/json"
	"testing"

	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestMongoStoreNormalizeAuthForStorage_UsesRelativePathID(t *testing.T) {
	store := &MongoStore{}
	store.SetBaseDir("/data/auths")

	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Attributes: map[string]string{
			"path": "/data/auths/nested/token.json",
		},
	}

	normalized, err := store.normalizeAuthForStorage(auth)
	if err != nil {
		t.Fatalf("normalize auth: %v", err)
	}

	if got := normalized.ID; got != "nested/token.json" {
		t.Fatalf("unexpected normalized id: got %q want %q", got, "nested/token.json")
	}
	if got := normalized.FileName; got != "token.json" {
		t.Fatalf("unexpected normalized file name: got %q want %q", got, "token.json")
	}
	if got := normalized.Attributes["storage_backend"]; got != "mongo" {
		t.Fatalf("expected storage_backend=mongo, got %q", got)
	}
}

func TestMongoStoreNormalizeAuthForStorage_DerivesIDFromProviderAndEmail(t *testing.T) {
	store := &MongoStore{}

	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Metadata: map[string]any{
			"email": "mongo@example.com",
		},
	}

	normalized, err := store.normalizeAuthForStorage(auth)
	if err != nil {
		t.Fatalf("normalize auth: %v", err)
	}

	if got := normalized.ID; got != "codex-mongo@example.com.json" {
		t.Fatalf("unexpected derived id: got %q want %q", got, "codex-mongo@example.com.json")
	}
	if got := normalized.FileName; got != "codex-mongo@example.com.json" {
		t.Fatalf("unexpected derived file name: got %q want %q", got, "codex-mongo@example.com.json")
	}
}

func TestMongoStoreDecodeDocument_RestoresAuthFields(t *testing.T) {
	store := &MongoStore{}

	payloadAuth := &cliproxyauth.Auth{
		ID:       "nested/token.json",
		Provider: "codex",
		Metadata: map[string]any{
			"email": "mongo@example.com",
		},
	}
	payload, err := json.Marshal(payloadAuth)
	if err != nil {
		t.Fatalf("marshal auth payload: %v", err)
	}

	decoded, err := store.decodeDocument(&mongoAuthDocument{
		ID:       "nested/token.json",
		FileName: "nested/token.json",
		Provider: "codex",
		Payload:  payload,
	})
	if err != nil {
		t.Fatalf("decode document: %v", err)
	}

	if decoded == nil {
		t.Fatalf("expected decoded auth, got nil")
	}
	if got := decoded.ID; got != "nested/token.json" {
		t.Fatalf("unexpected decoded id: got %q want %q", got, "nested/token.json")
	}
	if got := decoded.FileName; got != "nested/token.json" {
		t.Fatalf("unexpected decoded file name: got %q want %q", got, "nested/token.json")
	}
	if got := decoded.Label; got != "mongo@example.com" {
		t.Fatalf("unexpected decoded label: got %q want %q", got, "mongo@example.com")
	}
	if got := decoded.Attributes["storage_backend"]; got != "mongo" {
		t.Fatalf("expected decoded storage_backend=mongo, got %q", got)
	}
}

func TestMongoStoreChangeFromEvent_ConvertsUpsertAndDeleteEvents(t *testing.T) {
	store := &MongoStore{}

	payloadAuth := &cliproxyauth.Auth{
		ID:       "stream/token.json",
		Provider: "codex",
		Metadata: map[string]any{
			"email": "mongo@example.com",
		},
	}
	payload, err := json.Marshal(payloadAuth)
	if err != nil {
		t.Fatalf("marshal auth payload: %v", err)
	}

	upsertChange, err := store.changeFromEvent(context.Background(), &mongoAuthChangeEvent{
		OperationType: "insert",
		FullDocument: &mongoAuthDocument{
			ID:       "stream/token.json",
			FileName: "stream/token.json",
			Provider: "codex",
			Payload:  payload,
		},
		DocumentKey: struct {
			ID string `bson:"_id"`
		}{ID: "stream/token.json"},
	})
	if err != nil {
		t.Fatalf("convert upsert event: %v", err)
	}
	if upsertChange == nil {
		t.Fatalf("expected upsert change, got nil")
	}
	if upsertChange.Action != sdkAuth.AuthChangeActionUpsert {
		t.Fatalf("unexpected upsert action: %+v", upsertChange)
	}
	if upsertChange.Auth == nil || upsertChange.Auth.ID != "stream/token.json" {
		t.Fatalf("unexpected upsert auth: %+v", upsertChange)
	}
	if got := upsertChange.Auth.Attributes["storage_backend"]; got != "mongo" {
		t.Fatalf("expected upsert auth storage_backend=mongo, got %q", got)
	}

	deleteChange, err := store.changeFromEvent(context.Background(), &mongoAuthChangeEvent{
		OperationType: "delete",
		DocumentKey: struct {
			ID string `bson:"_id"`
		}{ID: "stream/token.json"},
	})
	if err != nil {
		t.Fatalf("convert delete event: %v", err)
	}
	if deleteChange == nil {
		t.Fatalf("expected delete change, got nil")
	}
	if deleteChange.Action != sdkAuth.AuthChangeActionDelete || deleteChange.ID != "stream/token.json" {
		t.Fatalf("unexpected delete change: %+v", deleteChange)
	}
}

func TestMongoStoreResolveDeleteID_NormalizesAgainstBaseDir(t *testing.T) {
	store := &MongoStore{}
	store.SetBaseDir("/data/auths")

	got := store.resolveDeleteID("/data/auths/nested/token.json")
	if got != "nested/token.json" {
		t.Fatalf("unexpected normalized delete id: got %q want %q", got, "nested/token.json")
	}
}
