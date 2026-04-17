package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestCountAuthFiles(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)

	seedAuthFileRecord(t, manager, authDir, &coreauth.Auth{
		ID:       "alpha",
		FileName: "alpha.json",
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"path": filepath.Join(authDir, "alpha.json"),
		},
		LastResult: &coreauth.ResultSnapshot{
			Success:    false,
			StatusCode: http.StatusUnauthorized,
			Code:       "token_revoked",
			Message:    "token revoked",
		},
	})
	seedAuthFileRecord(t, manager, authDir, &coreauth.Auth{
		ID:       "beta",
		FileName: "beta.json",
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Disabled: true,
		Attributes: map[string]string{
			"path": filepath.Join(authDir, "beta.json"),
		},
		LastResult: &coreauth.ResultSnapshot{
			Success:    true,
			StatusCode: http.StatusOK,
		},
	})
	seedAuthFileRecord(t, manager, authDir, &coreauth.Auth{
		ID:       "gamma",
		FileName: "gamma.json",
		Provider: "claude",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"path": filepath.Join(authDir, "gamma.json"),
		},
		LastResult: &coreauth.ResultSnapshot{
			Success:    false,
			StatusCode: http.StatusUnauthorized,
			Code:       "expired",
			Message:    "expired",
		},
	})

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/auth-files/count?provider=codex",
		nil,
	)

	h.CountAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Total        int            `json:"total"`
		TypeCounts   map[string]int `json:"type_counts"`
		Enabled      int            `json:"enabled"`
		Disabled     int            `json:"disabled"`
		Usable       int            `json:"usable"`
		Cooling      int            `json:"cooling"`
		ProblemCount int            `json:"problem_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode count payload: %v", err)
	}

	if payload.Total != 2 {
		t.Fatalf("expected total 2, got %d", payload.Total)
	}
	if payload.Enabled != 1 {
		t.Fatalf("expected enabled 1, got %d", payload.Enabled)
	}
	if payload.Disabled != 1 {
		t.Fatalf("expected disabled 1, got %d", payload.Disabled)
	}
	if payload.ProblemCount != 1 {
		t.Fatalf("expected problem_count 1, got %d", payload.ProblemCount)
	}
	if got := payload.TypeCounts["all"]; got != 3 {
		t.Fatalf("expected type_counts.all 3, got %d", got)
	}
	if got := payload.TypeCounts["codex"]; got != 2 {
		t.Fatalf("expected type_counts.codex 2, got %d", got)
	}
}
