package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestResetAllAuthRetryTimes_ClearsCooldownAndRegistrySuspension(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	cfg := &config.Config{}
	manager.SetConfig(cfg)

	nextRetry := time.Now().Add(30 * time.Minute)
	auth := &coreauth.Auth{
		ID:             "auth-reset-all",
		FileName:       "auth-reset-all.json",
		Provider:       "codex",
		Status:         coreauth.StatusError,
		StatusMessage:  "rate limited",
		Unavailable:    true,
		NextRetryAfter: nextRetry,
		Quota: coreauth.QuotaState{
			Exceeded:      true,
			Reason:        "quota",
			NextRecoverAt: nextRetry,
			BackoffLevel:  2,
		},
		Attributes: map[string]string{
			"path": "/tmp/auth-reset-all.json",
		},
		ModelStates: map[string]*coreauth.ModelState{
			"gpt-5": {
				Status:         coreauth.StatusError,
				StatusMessage:  "rate limited",
				Unavailable:    true,
				NextRetryAfter: nextRetry,
				Quota: coreauth.QuotaState{
					Exceeded:      true,
					Reason:        "quota",
					NextRecoverAt: nextRetry,
					BackoffLevel:  3,
				},
			},
		},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	reg := registry.GetGlobalRegistry()
	reg.RegisterClient(auth.ID, "codex", []*registry.ModelInfo{{ID: "gpt-5"}})
	reg.SuspendClientModel(auth.ID, "gpt-5", "manual")
	if models := reg.GetAvailableModelsByProvider("codex"); len(models) != 0 {
		t.Fatalf("expected suspended model to be unavailable before reset, got %d models", len(models))
	}
	t.Cleanup(func() {
		reg.UnregisterClient(auth.ID)
	})

	h := NewHandlerWithoutConfigFilePath(cfg, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/retry/reset-all", nil)

	h.ResetAllAuthRetryTimes(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Status string `json:"status"`
		Reset  int    `json:"reset"`
		Total  int    `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected status ok, got %q", payload.Status)
	}
	if payload.Reset != 1 || payload.Total != 1 {
		t.Fatalf("expected reset=1 total=1, got reset=%d total=%d", payload.Reset, payload.Total)
	}

	updated, ok := manager.GetByID(auth.ID)
	if !ok {
		t.Fatalf("expected auth %q to still exist", auth.ID)
	}
	if updated.Unavailable {
		t.Fatalf("expected auth unavailable=false after reset")
	}
	if !updated.NextRetryAfter.IsZero() {
		t.Fatalf("expected auth next retry cleared, got %v", updated.NextRetryAfter)
	}
	if updated.Quota.Exceeded || !updated.Quota.NextRecoverAt.IsZero() || updated.Quota.BackoffLevel != 0 {
		t.Fatalf("expected auth quota cleared, got %#v", updated.Quota)
	}

	state := updated.ModelStates["gpt-5"]
	if state == nil {
		t.Fatalf("expected gpt-5 state to remain present")
	}
	if state.Unavailable {
		t.Fatalf("expected model state unavailable=false after reset")
	}
	if !state.NextRetryAfter.IsZero() {
		t.Fatalf("expected model state next retry cleared, got %v", state.NextRetryAfter)
	}
	if state.Quota.Exceeded || !state.Quota.NextRecoverAt.IsZero() || state.Quota.BackoffLevel != 0 {
		t.Fatalf("expected model quota cleared, got %#v", state.Quota)
	}

	models := reg.GetAvailableModelsByProvider("codex")
	if len(models) != 1 || models[0].ID != "gpt-5" {
		t.Fatalf("expected gpt-5 to be available again after reset, got %#v", models)
	}
}

func TestResetAllAuthRetryTimes_RespectsFilterQuery(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	cfg := &config.Config{}
	manager.SetConfig(cfg)

	nextRetry := time.Now().Add(15 * time.Minute)
	auths := []*coreauth.Auth{
		{
			ID:             "auth-reset-codex",
			FileName:       "auth-reset-codex.json",
			Provider:       "codex",
			Unavailable:    true,
			NextRetryAfter: nextRetry,
			Attributes: map[string]string{
				"path": "/tmp/auth-reset-codex.json",
			},
		},
		{
			ID:             "auth-reset-claude",
			FileName:       "auth-reset-claude.json",
			Provider:       "claude",
			Unavailable:    true,
			NextRetryAfter: nextRetry,
			Attributes: map[string]string{
				"path": "/tmp/auth-reset-claude.json",
			},
		},
	}
	for _, auth := range auths {
		if _, err := manager.Register(context.Background(), auth); err != nil {
			t.Fatalf("register auth %s: %v", auth.ID, err)
		}
	}

	h := NewHandlerWithoutConfigFilePath(cfg, manager)

	body := bytes.NewBufferString(`{"query":{"provider":"codex"}}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/retry/reset-all", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.ResetAllAuthRetryTimes(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	codexAuth, _ := manager.GetByID("auth-reset-codex")
	if codexAuth == nil || !codexAuth.NextRetryAfter.IsZero() {
		t.Fatalf("expected codex auth retry time to be reset, got %+v", codexAuth)
	}
	claudeAuth, _ := manager.GetByID("auth-reset-claude")
	if claudeAuth == nil || claudeAuth.NextRetryAfter.IsZero() {
		t.Fatalf("expected claude auth retry time to remain set, got %+v", claudeAuth)
	}
}
