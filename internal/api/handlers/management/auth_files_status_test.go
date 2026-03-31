package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestListAuthFiles_IncludesStructuredErrorFields(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	record := &coreauth.Auth{
		ID:            "auth-with-error",
		FileName:      "auth-with-error.json",
		Provider:      "codex",
		Status:        coreauth.StatusError,
		StatusMessage: "Unsupported parameter: background",
		Unavailable:   true,
		Attributes: map[string]string{
			"path": "/tmp/auth-with-error.json",
		},
		LastError: &coreauth.Error{
			Code:       "unsupported_parameter",
			Message:    "Unsupported parameter: background",
			HTTPStatus: http.StatusBadRequest,
			Retryable:  false,
		},
		Metadata: map[string]any{
			"type": "codex",
		},
	}
	if _, err := manager.Register(context.Background(), record); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(payload.Files))
	}

	entry := payload.Files[0]
	if got := entry["status_code"]; got != float64(http.StatusBadRequest) {
		t.Fatalf("expected status_code %d, got %#v", http.StatusBadRequest, got)
	}
	if nextRetry, exists := entry["next_retry_after"]; !exists {
		t.Fatalf("expected next_retry_after key to be present")
	} else if nextRetry != nil {
		t.Fatalf("expected next_retry_after to be null, got %#v", nextRetry)
	}
	if got := entry["usable"]; got != true {
		t.Fatalf("expected usable true, got %#v", got)
	}
	if got := entry["cooling"]; got != false {
		t.Fatalf("expected cooling false, got %#v", got)
	}
	if got := entry["error_code"]; got != "unsupported_parameter" {
		t.Fatalf("expected error_code unsupported_parameter, got %#v", got)
	}
	if got := entry["error_message"]; got != "Unsupported parameter: background" {
		t.Fatalf("expected error_message to be propagated, got %#v", got)
	}

	lastError, ok := entry["last_error"].(map[string]any)
	if !ok {
		t.Fatalf("expected last_error object, got %#v", entry["last_error"])
	}
	if got := lastError["http_status"]; got != float64(http.StatusBadRequest) {
		t.Fatalf("expected last_error.http_status %d, got %#v", http.StatusBadRequest, got)
	}
	if got := lastError["code"]; got != "unsupported_parameter" {
		t.Fatalf("expected last_error.code unsupported_parameter, got %#v", got)
	}
	if got := lastError["message"]; got != "Unsupported parameter: background" {
		t.Fatalf("expected last_error.message to match, got %#v", got)
	}
}

func TestListAuthFiles_IncludesLastSuccessfulStatusCode(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	record := &coreauth.Auth{
		ID:       "auth-success",
		FileName: "auth-success.json",
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"path": "/tmp/auth-success.json",
		},
		LastResult: &coreauth.ResultSnapshot{
			Success:    true,
			StatusCode: http.StatusOK,
			UpdatedAt:  time.Now().UTC(),
		},
		Metadata: map[string]any{
			"type": "codex",
		},
	}
	if _, err := manager.Register(context.Background(), record); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: t.TempDir()}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}
	if len(payload.Files) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(payload.Files))
	}

	entry := payload.Files[0]
	if got := entry["status_code"]; got != float64(http.StatusOK) {
		t.Fatalf("expected status_code %d, got %#v", http.StatusOK, got)
	}
	if nextRetry, exists := entry["next_retry_after"]; !exists {
		t.Fatalf("expected next_retry_after key to be present")
	} else if nextRetry != nil {
		t.Fatalf("expected next_retry_after to be null, got %#v", nextRetry)
	}
	if got := entry["usable"]; got != true {
		t.Fatalf("expected usable true, got %#v", got)
	}
	if got := entry["cooling"]; got != false {
		t.Fatalf("expected cooling false, got %#v", got)
	}

	lastResult, ok := entry["last_result"].(map[string]any)
	if !ok {
		t.Fatalf("expected last_result object, got %#v", entry["last_result"])
	}
	if got := lastResult["success"]; got != true {
		t.Fatalf("expected last_result.success true, got %#v", got)
	}
	if got := lastResult["status_code"]; got != float64(http.StatusOK) {
		t.Fatalf("expected last_result.status_code %d, got %#v", http.StatusOK, got)
	}
	if _, exists := entry["last_error"]; exists {
		t.Fatalf("did not expect last_error on successful auth entry, got %#v", entry["last_error"])
	}
}

func TestListAuthFiles_IncludesUsageFlagsForCoolingAndDisabledStates(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	manager := coreauth.NewManager(nil, nil, nil)
	authDir := t.TempDir()
	coolingPath := filepath.Join(authDir, "auth-cooling.json")
	disabledPath := filepath.Join(authDir, "auth-disabled.json")
	if err := os.WriteFile(coolingPath, []byte(`{"type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write cooling auth file: %v", err)
	}
	if err := os.WriteFile(disabledPath, []byte(`{"type":"codex"}`), 0o600); err != nil {
		t.Fatalf("write disabled auth file: %v", err)
	}

	coolingRetryAt := time.Now().UTC().Add(30 * time.Minute)
	records := []*coreauth.Auth{
		{
			ID:             "auth-cooling",
			FileName:       "auth-cooling.json",
			Provider:       "codex",
			Status:         coreauth.StatusError,
			StatusMessage:  "quota exhausted",
			Unavailable:    true,
			NextRetryAfter: coolingRetryAt,
			Attributes: map[string]string{
				"path": coolingPath,
			},
			Metadata: map[string]any{
				"type": "codex",
			},
		},
		{
			ID:       "auth-disabled",
			FileName: "auth-disabled.json",
			Provider: "codex",
			Status:   coreauth.StatusDisabled,
			Disabled: true,
			Attributes: map[string]string{
				"path": disabledPath,
			},
			Metadata: map[string]any{
				"type": "codex",
			},
		},
	}
	for _, record := range records {
		if _, err := manager.Register(context.Background(), record); err != nil {
			t.Fatalf("register auth %s: %v", record.ID, err)
		}
	}

	h := NewHandlerWithoutConfigFilePath(&config.Config{AuthDir: authDir}, manager)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files", nil)
	h.ListAuthFiles(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d with body %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Files []map[string]any `json:"files"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list payload: %v", err)
	}
	if len(payload.Files) != 2 {
		t.Fatalf("expected 2 file entries, got %d", len(payload.Files))
	}

	entriesByName := make(map[string]map[string]any, len(payload.Files))
	for _, entry := range payload.Files {
		name, _ := entry["name"].(string)
		entriesByName[name] = entry
	}

	coolingEntry := entriesByName["auth-cooling.json"]
	if coolingEntry == nil {
		t.Fatalf("expected auth-cooling.json entry to be present")
	}
	if got := coolingEntry["usable"]; got != false {
		t.Fatalf("expected cooling auth usable false, got %#v", got)
	}
	if got := coolingEntry["cooling"]; got != true {
		t.Fatalf("expected cooling auth cooling true, got %#v", got)
	}
	if nextRetry, ok := coolingEntry["next_retry_after"].(string); !ok || nextRetry == "" {
		t.Fatalf("expected cooling auth next_retry_after string, got %#v", coolingEntry["next_retry_after"])
	}

	disabledEntry := entriesByName["auth-disabled.json"]
	if disabledEntry == nil {
		t.Fatalf("expected auth-disabled.json entry to be present")
	}
	if got := disabledEntry["usable"]; got != false {
		t.Fatalf("expected disabled auth usable false, got %#v", got)
	}
	if got := disabledEntry["cooling"]; got != false {
		t.Fatalf("expected disabled auth cooling false, got %#v", got)
	}
	if nextRetry, exists := disabledEntry["next_retry_after"]; !exists {
		t.Fatalf("expected disabled auth next_retry_after key to be present")
	} else if nextRetry != nil {
		t.Fatalf("expected disabled auth next_retry_after null, got %#v", nextRetry)
	}
}
