package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestListAuthFiles_AppliesServerPaginationAndFilters(t *testing.T) {
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
		"/v0/management/auth-files?provider=codex&page=2&page_size=1&sort=az",
		nil,
	)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Files      []map[string]any `json:"files"`
		Total      int              `json:"total"`
		Page       int              `json:"page"`
		PageSize   int              `json:"page_size"`
		TotalPages int              `json:"total_pages"`
		TypeCounts map[string]int   `json:"type_counts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}

	if payload.Total != 2 {
		t.Fatalf("expected total 2, got %d", payload.Total)
	}
	if payload.Page != 2 {
		t.Fatalf("expected page 2, got %d", payload.Page)
	}
	if payload.PageSize != 1 {
		t.Fatalf("expected page_size 1, got %d", payload.PageSize)
	}
	if payload.TotalPages != 2 {
		t.Fatalf("expected total_pages 2, got %d", payload.TotalPages)
	}
	if got := payload.TypeCounts["all"]; got != 3 {
		t.Fatalf("expected type_counts.all 3, got %d", got)
	}
	if got := payload.TypeCounts["codex"]; got != 2 {
		t.Fatalf("expected type_counts.codex 2, got %d", got)
	}
	if got := payload.TypeCounts["claude"]; got != 1 {
		t.Fatalf("expected type_counts.claude 1, got %d", got)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(payload.Files))
	}
	if got, _ := payload.Files[0]["name"].(string); got != "beta.json" {
		t.Fatalf("expected second page to contain beta.json, got %#v", payload.Files[0]["name"])
	}

	filteredRec := httptest.NewRecorder()
	filteredCtx, _ := gin.CreateTestContext(filteredRec)
	filteredCtx.Request = httptest.NewRequest(
		http.MethodGet,
		"/v0/management/auth-files?problem_only=1&status_code=401&search=gam*",
		nil,
	)
	h.ListAuthFiles(filteredCtx)

	if filteredRec.Code != http.StatusOK {
		t.Fatalf("expected filtered list status %d, got %d with body %s", http.StatusOK, filteredRec.Code, filteredRec.Body.String())
	}

	var filteredPayload struct {
		Files []map[string]any `json:"files"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(filteredRec.Body.Bytes(), &filteredPayload); err != nil {
		t.Fatalf("decode filtered payload: %v", err)
	}
	if filteredPayload.Total != 1 || len(filteredPayload.Files) != 1 {
		t.Fatalf("expected exactly one filtered file, got total=%d len=%d", filteredPayload.Total, len(filteredPayload.Files))
	}
	if got, _ := filteredPayload.Files[0]["name"].(string); got != "gamma.json" {
		t.Fatalf("expected filtered result gamma.json, got %#v", filteredPayload.Files[0]["name"])
	}
}

func TestDeleteAuthFile_ByServerFilters(t *testing.T) {
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
	h.tokenStore = &memoryAuthStore{}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(
		http.MethodDelete,
		"/v0/management/auth-files?provider=codex&problem_only=1&status_code=401&all=1",
		nil,
	)
	h.DeleteAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Deleted int `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode delete payload: %v", err)
	}
	if payload.Deleted != 1 {
		t.Fatalf("expected deleted 1, got %d", payload.Deleted)
	}

	if _, err := os.Stat(filepath.Join(authDir, "alpha.json")); !os.IsNotExist(err) {
		t.Fatalf("expected alpha.json to be removed, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(authDir, "beta.json")); err != nil {
		t.Fatalf("expected beta.json to remain, stat err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(authDir, "gamma.json")); err != nil {
		t.Fatalf("expected gamma.json to remain, stat err: %v", err)
	}
}

func seedAuthFileRecord(t *testing.T, manager *coreauth.Manager, authDir string, record *coreauth.Auth) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(authDir, record.FileName), []byte(`{"type":"`+record.Provider+`"}`), 0o600); err != nil {
		t.Fatalf("write auth file %s: %v", record.FileName, err)
	}
	if _, err := manager.Register(context.Background(), record); err != nil {
		t.Fatalf("register auth %s: %v", record.ID, err)
	}
}
