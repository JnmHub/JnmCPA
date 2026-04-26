package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/runtime/executor"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	"github.com/tidwall/gjson"
)

type codexBrokenCompletedMock struct {
	path string
	body []byte
}

func newCodexBrokenCompletedServer(t *testing.T, responseID string, createdAt int64, totalTokens int64) (*httptest.Server, *codexBrokenCompletedMock) {
	t.Helper()

	mock := &codexBrokenCompletedMock{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		mock.body = append([]byte(nil), body...)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"` + responseID + `","model":"gpt-5.4","created_at":` + int64ToString(createdAt) + `,"output":[]}}`,
			`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"msg_` + responseID + `_0","type":"message","status":"in_progress","content":[{"type":"output_text","text":""}],"role":"assistant"}}`,
			`data: {"type":"response.output_text.delta","item_id":"msg_` + responseID + `_0","output_index":0,"delta":"OK"}`,
			`data: {"type":"response.output_text.done","item_id":"msg_` + responseID + `_0","output_index":0,"text":"OK"}`,
			`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"msg_` + responseID + `_0","type":"message","status":"completed","content":[{"type":"output_text","text":"OK"}],"role":"assistant"}}`,
			`data: {"type":"response.completed","response":{"id":"` + responseID + `","model":"gpt-5.4","created_at":` + int64ToString(createdAt) + `,"status":"completed","usage":{"input_tokens":5,"output_tokens":2,"total_tokens":` + int64ToString(totalTokens) + `},"output":[]}}`,
			`data: [DONE]`,
			"",
		}, "\n"))
	}))
	return server, mock
}

func registerCodexTestAuth(t *testing.T, manager *coreauth.Manager, baseURL string, authID string, modelID string) {
	t.Helper()

	auth := &coreauth.Auth{
		ID:       authID,
		Provider: "codex",
		Status:   coreauth.StatusActive,
		Attributes: map[string]string{
			"base_url": baseURL,
			"api_key":  "test-token",
		},
		Metadata: map[string]any{"email": authID + "@example.com"},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("manager.Register(%s): %v", authID, err)
	}
	registry.GetGlobalRegistry().RegisterClient(auth.ID, auth.Provider, []*registry.ModelInfo{{ID: modelID}})
	t.Cleanup(func() {
		registry.GetGlobalRegistry().UnregisterClient(auth.ID)
	})
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func TestOpenAIChatCompletions_NonStreamCodexBrokenCompletedOutputStillReturnsContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server, mock := newCodexBrokenCompletedServer(t, "resp_handler_chat", 1777190500, 7)
	defer server.Close()

	manager := coreauth.NewManager(nil, nil, nil)
	manager.RegisterExecutor(executor.NewCodexExecutor(&config.Config{}))
	registerCodexTestAuth(t, manager, server.URL, "auth-codex-chat", "gpt-5.4")

	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, manager)
	h := NewOpenAIAPIHandler(base)
	router := gin.New()
	router.POST("/v1/chat/completions", h.ChatCompletions)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.4","messages":[{"role":"user","content":"Reply with exactly OK"}]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if mock.path != "/responses" {
		t.Fatalf("upstream path = %q, want %q", mock.path, "/responses")
	}
	if got := gjson.GetBytes(mock.body, "stream").Bool(); !got {
		t.Fatalf("expected upstream stream=true body=%s", string(mock.body))
	}
	if got := gjson.Get(resp.Body.String(), "choices.0.message.content").String(); got != "OK" {
		t.Fatalf("choices[0].message.content = %q, want %q body=%s", got, "OK", resp.Body.String())
	}
	if got := gjson.Get(resp.Body.String(), "usage.total_tokens").Int(); got != 7 {
		t.Fatalf("usage.total_tokens = %d, want 7 body=%s", got, resp.Body.String())
	}
}

func TestOpenAIResponses_NonStreamCodexBrokenCompletedOutputStillReturnsOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server, mock := newCodexBrokenCompletedServer(t, "resp_handler_responses", 1777190600, 9)
	defer server.Close()

	manager := coreauth.NewManager(nil, nil, nil)
	manager.RegisterExecutor(executor.NewCodexExecutor(&config.Config{}))
	registerCodexTestAuth(t, manager, server.URL, "auth-codex-responses", "gpt-5.4")

	base := handlers.NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, manager)
	h := NewOpenAIResponsesAPIHandler(base)
	router := gin.New()
	router.POST("/v1/responses", h.Responses)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.4","input":"Reply with exactly OK"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if mock.path != "/responses" {
		t.Fatalf("upstream path = %q, want %q", mock.path, "/responses")
	}
	if got := gjson.GetBytes(mock.body, "stream").Bool(); !got {
		t.Fatalf("expected upstream stream=true body=%s", string(mock.body))
	}
	if got := gjson.Get(resp.Body.String(), "output.0.content.0.text").String(); got != "OK" {
		t.Fatalf("output[0].content[0].text = %q, want %q body=%s", got, "OK", resp.Body.String())
	}
	if got := gjson.Get(resp.Body.String(), "usage.total_tokens").Int(); got != 9 {
		t.Fatalf("usage.total_tokens = %d, want 9 body=%s", got, resp.Body.String())
	}
}
