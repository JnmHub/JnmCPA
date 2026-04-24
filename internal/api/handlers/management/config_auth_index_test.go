package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher/synthesizer"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestGetGeminiKeys_IncludesStableAuthIndex(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	idGen := synthesizer.NewStableIDGenerator()
	authID, _ := idGen.Next("gemini:apikey", "shared-key", "https://gemini.example.com")

	manager := cliproxyauth.NewManager(nil, nil, nil)
	auth := &cliproxyauth.Auth{
		ID:       authID,
		Provider: "gemini",
	}
	expectedIndex := auth.EnsureIndex()
	if _, errRegister := manager.Register(context.Background(), auth); errRegister != nil {
		t.Fatalf("register auth failed: %v", errRegister)
	}

	h := &Handler{
		cfg: &config.Config{
			GeminiKey: []config.GeminiKey{
				{APIKey: "shared-key", BaseURL: "https://gemini.example.com"},
			},
		},
		authManager: manager,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v0/management/gemini-api-key", nil)

	h.GetGeminiKeys(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload struct {
		GeminiKeys []struct {
			APIKey    string `json:"api-key"`
			BaseURL   string `json:"base-url"`
			AuthIndex string `json:"auth-index"`
		} `json:"gemini-api-key"`
	}
	if errDecode := json.Unmarshal(rec.Body.Bytes(), &payload); errDecode != nil {
		t.Fatalf("decode response failed: %v", errDecode)
	}
	if len(payload.GeminiKeys) != 1 {
		t.Fatalf("gemini keys len = %d, want 1", len(payload.GeminiKeys))
	}
	if payload.GeminiKeys[0].AuthIndex != expectedIndex {
		t.Fatalf("auth-index = %q, want %q", payload.GeminiKeys[0].AuthIndex, expectedIndex)
	}
}
