package auth

import (
	"context"
	"net/http"
	"testing"
)

func TestManager_MarkResult_DeletesAuthOnUnauthorized(t *testing.T) {
	store := &countingStore{}
	manager := NewManager(store, nil, nil)

	auth := &Auth{
		ID:       "auth-401",
		Provider: "claude",
		Metadata: map[string]any{"type": "claude"},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: auth.Provider,
		Model:    "claude-sonnet",
		Success:  false,
		Error: &Error{
			HTTPStatus: http.StatusUnauthorized,
			Message:    "unauthorized",
		},
	})

	if _, ok := manager.GetByID(auth.ID); ok {
		t.Fatalf("expected auth %q to be removed after 401", auth.ID)
	}
	if got := len(manager.List()); got != 0 {
		t.Fatalf("expected no auths after 401 deletion, got %d", got)
	}
	if got := store.deleteCount.Load(); got != 1 {
		t.Fatalf("expected 1 Delete call, got %d", got)
	}
}

func TestManager_MarkResult_DeletesAuthOnTooManyRequests(t *testing.T) {
	store := &countingStore{}
	manager := NewManager(store, nil, nil)

	auth := &Auth{
		ID:       "auth-429",
		Provider: "codex",
		Metadata: map[string]any{"type": "codex"},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: auth.Provider,
		Model:    "gpt-5",
		Success:  false,
		Error: &Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    "quota exhausted",
		},
	})

	if _, ok := manager.GetByID(auth.ID); ok {
		t.Fatalf("expected auth %q to be removed after 429", auth.ID)
	}
	if got := len(manager.List()); got != 0 {
		t.Fatalf("expected no auths after 429 deletion, got %d", got)
	}
	if got := store.deleteCount.Load(); got != 1 {
		t.Fatalf("expected 1 Delete call, got %d", got)
	}
}

func TestManager_Delete_WithSkipPersist_RemovesOnlyRuntimeState(t *testing.T) {
	store := &countingStore{}
	manager := NewManager(store, nil, nil)

	auth := &Auth{
		ID:       "auth-skip-delete",
		Provider: "gemini",
		Metadata: map[string]any{"type": "gemini"},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	if err := manager.Delete(WithSkipPersist(context.Background()), auth.ID); err != nil {
		t.Fatalf("delete auth: %v", err)
	}

	if _, ok := manager.GetByID(auth.ID); ok {
		t.Fatalf("expected auth %q to be removed from runtime state", auth.ID)
	}
	if got := store.deleteCount.Load(); got != 0 {
		t.Fatalf("expected no store Delete calls with skip persist, got %d", got)
	}
}

func TestManager_MarkResult_StoresSuccessfulHTTPStatus(t *testing.T) {
	manager := NewManager(nil, nil, nil)

	auth := &Auth{
		ID:       "auth-success-status",
		Provider: "codex",
		Metadata: map[string]any{"type": "codex"},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: auth.Provider,
		Model:    "gpt-5",
		Success:  true,
	})

	stored, ok := manager.GetByID(auth.ID)
	if !ok {
		t.Fatalf("expected auth %q to remain registered", auth.ID)
	}
	if stored.LastResult == nil {
		t.Fatalf("expected LastResult to be recorded")
	}
	if !stored.LastResult.Success {
		t.Fatalf("expected LastResult.Success to be true")
	}
	if stored.LastResult.StatusCode != http.StatusOK {
		t.Fatalf("expected LastResult.StatusCode %d, got %d", http.StatusOK, stored.LastResult.StatusCode)
	}
}
