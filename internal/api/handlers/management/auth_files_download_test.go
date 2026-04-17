package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestDownloadAuthFile_ReturnsFile(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	fileName := "download-user.json"
	expected := []byte(`{"type":"codex"}`)
	if err := os.WriteFile(filepath.Join(authDir, fileName), expected, 0o600); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, nil)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files/download?name="+url.QueryEscape(fileName), nil)
	h.DownloadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected download status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got := rec.Body.Bytes(); string(got) != string(expected) {
		t.Fatalf("unexpected download content: %q", string(got))
	}
}

func TestDownloadAuthFile_RejectsPathSeparators(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, nil)

	for _, name := range []string{
		"../external/secret.json",
		`..\\external\\secret.json`,
		"nested/secret.json",
		`nested\\secret.json`,
	} {
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files/download?name="+url.QueryEscape(name), nil)
		h.DownloadAuthFile(ctx)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected %d for name %q, got %d with body %s", http.StatusBadRequest, name, rec.Code, rec.Body.String())
		}
	}
}

func TestDownloadAuthFile_FallsBackToRuntimeAuthMetadata(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	manager := coreauth.NewManager(nil, nil, nil)
	auth := &coreauth.Auth{
		ID:       "sqlite-user.json",
		FileName: "sqlite-user.json",
		Provider: "codex",
		Disabled: true,
		Metadata: map[string]any{
			"type":  "codex",
			"email": "download@example.com",
		},
		Attributes: map[string]string{
			"storage_backend": "sqlite",
		},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files/download?name="+url.QueryEscape(auth.FileName), nil)
	h.DownloadAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected download status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["type"] != "codex" {
		t.Fatalf("expected type codex, got %#v", payload["type"])
	}
	if payload["email"] != "download@example.com" {
		t.Fatalf("expected email download@example.com, got %#v", payload["email"])
	}
	if payload["disabled"] != true {
		t.Fatalf("expected disabled true, got %#v", payload["disabled"])
	}
}
