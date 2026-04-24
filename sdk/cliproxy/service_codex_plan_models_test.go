package cliproxy

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

func TestRegisterModelsForAuth_CodexPaidPlansIncludeGPT55(t *testing.T) {
	tests := []struct {
		name string
		plan string
	}{
		{name: "team", plan: "team"},
		{name: "plus", plan: "plus"},
		{name: "pro", plan: "pro"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{cfg: &config.Config{}}
			auth := &coreauth.Auth{
				ID:       "codex-" + tt.plan + "-auth",
				Provider: "codex",
				Status:   coreauth.StatusActive,
				Attributes: map[string]string{
					"plan_type": tt.plan,
				},
			}

			modelRegistry := registry.GetGlobalRegistry()
			modelRegistry.UnregisterClient(auth.ID)
			t.Cleanup(func() {
				modelRegistry.UnregisterClient(auth.ID)
			})

			service.registerModelsForAuth(auth)

			models := modelRegistry.GetModelsForClient(auth.ID)
			if len(models) == 0 {
				t.Fatalf("expected models to be registered for codex %s plan", tt.plan)
			}
			if !containsModelID(models, "gpt-5.5") {
				t.Fatalf("expected codex %s plan models to include gpt-5.5", tt.plan)
			}
		})
	}
}

func containsModelID(models []*ModelInfo, id string) bool {
	for _, model := range models {
		if model != nil && model.ID == id {
			return true
		}
	}
	return false
}
