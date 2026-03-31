package cliproxy

import (
	"context"
	"testing"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestServiceApplyCoreAuthRemoval_RemovesAuth(t *testing.T) {
	manager := coreauth.NewManager(nil, nil, nil)
	auth := &coreauth.Auth{
		ID:       "auth-remove",
		Provider: "claude",
		Metadata: map[string]any{"type": "claude"},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	svc := &Service{coreManager: manager}
	svc.applyCoreAuthRemoval(context.Background(), auth.ID)

	if _, ok := manager.GetByID(auth.ID); ok {
		t.Fatalf("expected auth %q to be removed", auth.ID)
	}
}
