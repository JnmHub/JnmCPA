package management

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type probeSingleExecutor struct {
	mu          sync.Mutex
	streamModel string
	streamErr   error
}

func (e *probeSingleExecutor) Identifier() string { return "codex" }

func (e *probeSingleExecutor) Execute(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, fmt.Errorf("not implemented")
}

func (e *probeSingleExecutor) ExecuteStream(_ context.Context, _ *coreauth.Auth, req coreexecutor.Request, _ coreexecutor.Options) (*coreexecutor.StreamResult, error) {
	e.mu.Lock()
	e.streamModel = req.Model
	err := e.streamErr
	e.mu.Unlock()
	if err != nil {
		return nil, err
	}

	chunks := make(chan coreexecutor.StreamChunk, 1)
	chunks <- coreexecutor.StreamChunk{Payload: []byte("data: ok\n\n")}
	close(chunks)
	return &coreexecutor.StreamResult{Chunks: chunks}, nil
}

func (e *probeSingleExecutor) Refresh(context.Context, *coreauth.Auth) (*coreauth.Auth, error) {
	return nil, fmt.Errorf("not implemented")
}

func (e *probeSingleExecutor) CountTokens(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, fmt.Errorf("not implemented")
}

func (e *probeSingleExecutor) HttpRequest(context.Context, *coreauth.Auth, *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func (e *probeSingleExecutor) lastModel() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.streamModel
}

func registerProbeSingleAuth(t *testing.T, manager *coreauth.Manager) *coreauth.Auth {
	t.Helper()

	auth := &coreauth.Auth{
		ID:       "probe-single-auth",
		FileName: "probe-single.json",
		Provider: "codex",
		Attributes: map[string]string{
			"path": "/tmp/probe-single.json",
		},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}
	return auth
}

func TestProbeAuthFile_ReturnsSelectedModelAndStatusCode(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	cfg := &config.Config{
		AuthAutoDelete: config.AuthAutoDeleteConfig{
			Unauthorized: true,
			RateLimited:  true,
		},
	}
	manager.SetConfig(cfg)

	executor := &probeSingleExecutor{}
	manager.RegisterExecutor(executor)
	registerProbeSingleAuth(t, manager)

	h := NewHandlerWithoutConfigFilePath(cfg, manager)

	body := bytes.NewBufferString(`{"name":"probe-single.json"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/probe", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.ProbeAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["success"]; got != true {
		t.Fatalf("expected success=true, got %#v", got)
	}
	if got := payload["deleted"]; got != false {
		t.Fatalf("expected deleted=false, got %#v", got)
	}
	if got := payload["status_code"]; got != float64(http.StatusOK) {
		t.Fatalf("expected status_code=%d, got %#v", http.StatusOK, got)
	}
	if got := payload["probe_model"]; got != "gpt-5" {
		t.Fatalf("expected probe_model gpt-5, got %#v", got)
	}
	if got := payload["probe_model_source"]; got != "default" {
		t.Fatalf("expected probe_model_source default, got %#v", got)
	}
	if got := executor.lastModel(); got != "gpt-5" {
		t.Fatalf("expected executor model gpt-5, got %q", got)
	}
}

func TestProbeAuthFile_SkipsExcludedDefaultModel(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	cfg := &config.Config{}
	manager.SetConfig(cfg)

	executor := &probeSingleExecutor{}
	manager.RegisterExecutor(executor)

	auth := &coreauth.Auth{
		ID:       "probe-skip-excluded",
		FileName: "probe-skip-excluded.json",
		Provider: "codex",
		Attributes: map[string]string{
			"path":            "/tmp/probe-skip-excluded.json",
			"excluded_models": "gpt-5",
		},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient(auth.ID, "codex", []*registry.ModelInfo{
		{ID: "gpt-5"},
		{ID: "gpt-4.1"},
	})
	t.Cleanup(func() {
		reg.UnregisterClient(auth.ID)
	})

	h := NewHandlerWithoutConfigFilePath(cfg, manager)

	body := bytes.NewBufferString(`{"name":"probe-skip-excluded.json"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/probe", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.ProbeAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["probe_model"]; got != "gpt-4.1" {
		t.Fatalf("expected probe_model gpt-4.1, got %#v", got)
	}
	if got := payload["probe_model_source"]; got != "registry" {
		t.Fatalf("expected probe_model_source registry, got %#v", got)
	}
	if got := executor.lastModel(); got != "gpt-4.1" {
		t.Fatalf("expected executor model gpt-4.1, got %q", got)
	}
}

func TestProbeAuthFile_UsesConfiguredProbeModel(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	cfg := &config.Config{
		AuthProbeModels: map[string]string{
			"codex": "gpt-4.1",
		},
	}
	manager.SetConfig(cfg)

	executor := &probeSingleExecutor{}
	manager.RegisterExecutor(executor)
	registerProbeSingleAuth(t, manager)

	h := NewHandlerWithoutConfigFilePath(cfg, manager)

	body := bytes.NewBufferString(`{"name":"probe-single.json"}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/probe", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.ProbeAuthFile(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["probe_model"]; got != "gpt-4.1" {
		t.Fatalf("expected probe_model gpt-4.1, got %#v", got)
	}
	if got := payload["probe_model_source"]; got != "config" {
		t.Fatalf("expected probe_model_source config, got %#v", got)
	}
	if got := executor.lastModel(); got != "gpt-4.1" {
		t.Fatalf("expected executor model gpt-4.1, got %q", got)
	}
}

func TestProbeAuthFile_RespectsUnauthorizedAutoDeleteConfig(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		autoDelete401 bool
		expectDeleted bool
		expectExists  bool
	}{
		{
			name:          "disabled",
			autoDelete401: false,
			expectDeleted: false,
			expectExists:  true,
		},
		{
			name:          "enabled",
			autoDelete401: true,
			expectDeleted: true,
			expectExists:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &memoryAuthStore{}
			manager := coreauth.NewManager(store, nil, nil)
			cfg := &config.Config{
				AuthAutoDelete: config.AuthAutoDeleteConfig{
					Unauthorized: tt.autoDelete401,
					RateLimited:  false,
				},
			}
			manager.SetConfig(cfg)

			executor := &probeSingleExecutor{
				streamErr: &coreauth.Error{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "token_revoked",
					Message:    "token revoked",
				},
			}
			manager.RegisterExecutor(executor)
			registerProbeSingleAuth(t, manager)

			h := NewHandlerWithoutConfigFilePath(cfg, manager)

			body := bytes.NewBufferString(`{"name":"probe-single.json"}`)
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/probe", body)
			ctx.Request.Header.Set("Content-Type", "application/json")

			h.ProbeAuthFile(ctx)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if got := payload["success"]; got != false {
				t.Fatalf("expected success=false, got %#v", got)
			}
			if got := payload["deleted"]; got != tt.expectDeleted {
				t.Fatalf("expected deleted=%v, got %#v", tt.expectDeleted, got)
			}
			if got := payload["status_code"]; got != float64(http.StatusUnauthorized) {
				t.Fatalf("expected status_code=%d, got %#v", http.StatusUnauthorized, got)
			}
			if got := payload["error_code"]; got != "token_revoked" {
				t.Fatalf("expected error_code token_revoked, got %#v", got)
			}

			_, exists := manager.GetByID("probe-single-auth")
			if exists != tt.expectExists {
				t.Fatalf("expected auth exists=%v, got %v", tt.expectExists, exists)
			}
		})
	}
}
