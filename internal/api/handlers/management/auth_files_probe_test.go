package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type probeTimeoutExecutor struct {
	executeCalls atomic.Int32
}

func (e *probeTimeoutExecutor) Identifier() string { return "codex" }

func (e *probeTimeoutExecutor) Execute(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (coreexecutor.Response, error) {
	e.executeCalls.Add(1)
	return coreexecutor.Response{}, &url.Error{
		Op:  http.MethodPost,
		URL: "https://chatgpt.com/backend-api/codex/responses",
		Err: context.DeadlineExceeded,
	}
}

func (e *probeTimeoutExecutor) ExecuteStream(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (*coreexecutor.StreamResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (e *probeTimeoutExecutor) Refresh(context.Context, *coreauth.Auth) (*coreauth.Auth, error) {
	return nil, fmt.Errorf("not implemented")
}

func (e *probeTimeoutExecutor) CountTokens(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, fmt.Errorf("not implemented")
}

func (e *probeTimeoutExecutor) HttpRequest(context.Context, *coreauth.Auth, *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestListAuthFiles_WithProbe_PopulatesNormalizedTimeoutFields(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	authDir := t.TempDir()
	authPath := filepath.Join(authDir, "probe-timeout.json")
	if err := os.WriteFile(authPath, []byte(`{"type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write auth fixture: %v", err)
	}

	manager := coreauth.NewManager(nil, nil, nil)
	executor := &probeTimeoutExecutor{}
	manager.RegisterExecutor(executor)

	auth := &coreauth.Auth{
		ID:       "probe-timeout",
		FileName: "probe-timeout.json",
		Provider: "codex",
		Attributes: map[string]string{
			"path": authPath,
		},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files?probe=1", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got := executor.executeCalls.Load(); got != 1 {
		t.Fatalf("expected 1 probe execute call, got %d", got)
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(payload.Files))
	}

	entry := payload.Files[0]
	if got := entry["status_code"]; got != float64(http.StatusRequestTimeout) {
		t.Fatalf("expected synthetic status_code %d, got %#v", http.StatusRequestTimeout, got)
	}
	if got := entry["error_code"]; got != "timeout" {
		t.Fatalf("expected error_code timeout, got %#v", got)
	}
	if got, _ := entry["error_message"].(string); got == "" {
		t.Fatalf("expected error_message to be populated, got %#v", entry["error_message"])
	}
}
