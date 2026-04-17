package management

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestExportAuthFiles_ReturnsFilteredZip(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(authDir, "alpha.json"), []byte(`{"type":"codex","email":"alpha@example.com"}`), 0o600); err != nil {
		t.Fatalf("write alpha: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	records := []*coreauth.Auth{
		{
			ID:       "alpha.json",
			FileName: "alpha.json",
			Provider: "codex",
			Attributes: map[string]string{
				"path": filepath.Join(authDir, "alpha.json"),
			},
			Metadata: map[string]any{"type": "codex", "email": "alpha@example.com"},
		},
		{
			ID:       "beta.json",
			FileName: "beta.json",
			Provider: "claude",
			Attributes: map[string]string{
				"storage_backend": "sqlite",
			},
			Metadata: map[string]any{"type": "claude", "email": "beta@example.com"},
		},
	}
	for _, auth := range records {
		if _, err := manager.Register(context.Background(), auth); err != nil {
			t.Fatalf("register auth %s: %v", auth.ID, err)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files/export?provider="+url.QueryEscape("claude"), nil)

	h.ExportAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected export status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("expected 1 exported file, got %d", len(zr.File))
	}
	if zr.File[0].Name != "beta.json" {
		t.Fatalf("expected beta.json in zip, got %q", zr.File[0].Name)
	}

	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open zip entry: %v", err)
	}
	defer rc.Close()
	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read zip entry: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		t.Fatalf("decode exported payload: %v", err)
	}
	if payload["type"] != "claude" {
		t.Fatalf("expected exported type claude, got %#v", payload["type"])
	}
	if payload["email"] != "beta@example.com" {
		t.Fatalf("expected exported email beta@example.com, got %#v", payload["email"])
	}
}
