package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	_ "github.com/router-for-me/CLIProxyAPI/v6/internal/translator"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
	"github.com/tidwall/gjson"
)

func TestCodexExecutorExecute_MockBrokenCompletedOutputStillReturnsChatContent(t *testing.T) {
	var gotPath string
	var gotBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, strings.Join([]string{
			`data: {"type":"response.created","response":{"id":"resp_mock_http","model":"gpt-5.4","created_at":1777190400,"output":[]}}`,
			`data: {"type":"response.output_item.added","output_index":0,"item":{"id":"msg_resp_mock_http_0","type":"message","status":"in_progress","content":[{"type":"output_text","text":""}],"role":"assistant"}}`,
			`data: {"type":"response.output_text.delta","item_id":"msg_resp_mock_http_0","output_index":0,"delta":"OK"}`,
			`data: {"type":"response.output_text.done","item_id":"msg_resp_mock_http_0","output_index":0,"text":"OK"}`,
			`data: {"type":"response.output_item.done","output_index":0,"item":{"id":"msg_resp_mock_http_0","type":"message","status":"completed","content":[{"type":"output_text","text":"OK"}],"role":"assistant"}}`,
			`data: {"type":"response.completed","response":{"id":"resp_mock_http","model":"gpt-5.4","created_at":1777190400,"status":"completed","usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7},"output":[]}}`,
			`data: [DONE]`,
			"",
		}, "\n"))
	}))
	defer server.Close()

	executor := NewCodexExecutor(&config.Config{})
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Attributes: map[string]string{
			"base_url": server.URL,
			"api_key":  "test-token",
		},
	}

	resp, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "gpt-5.4",
		Payload: []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"Reply with exactly OK"}]}`),
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if gotPath != "/responses" {
		t.Fatalf("path = %q, want %q", gotPath, "/responses")
	}
	if got := gjson.GetBytes(gotBody, "stream").Bool(); !got {
		t.Fatalf("expected upstream stream=true body=%s", string(gotBody))
	}
	if got := gjson.GetBytes(resp.Payload, "choices.0.message.content").String(); got != "OK" {
		t.Fatalf("choices[0].message.content = %q, want %q body=%s", got, "OK", string(resp.Payload))
	}
	if got := gjson.GetBytes(resp.Payload, "usage.total_tokens").Int(); got != 7 {
		t.Fatalf("usage.total_tokens = %d, want 7 body=%s", got, string(resp.Payload))
	}
}

func TestCodexWebsocketsExecutorExecute_MockBrokenCompletedOutputStillReturnsChatContent(t *testing.T) {
	var gotPath string
	var gotRequest []byte

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if !websocket.IsWebSocketUpgrade(r) {
			http.Error(w, "upgrade required", http.StatusUpgradeRequired)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade error: %v", err)
		}
		defer conn.Close()

		_, reqPayload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage error: %v", err)
		}
		gotRequest = append([]byte(nil), reqPayload...)

		events := []string{
			`{"type":"response.created","response":{"id":"resp_mock_ws","model":"gpt-5.4","created_at":1777190401,"output":[]}}`,
			`{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_resp_mock_ws_0","type":"message","status":"in_progress","content":[{"type":"output_text","text":""}],"role":"assistant"}}`,
			`{"type":"response.output_text.delta","item_id":"msg_resp_mock_ws_0","output_index":0,"delta":"OK"}`,
			`{"type":"response.output_text.done","item_id":"msg_resp_mock_ws_0","output_index":0,"text":"OK"}`,
			`{"type":"response.output_item.done","output_index":0,"item":{"id":"msg_resp_mock_ws_0","type":"message","status":"completed","content":[{"type":"output_text","text":"OK"}],"role":"assistant"}}`,
			`{"type":"response.completed","response":{"id":"resp_mock_ws","model":"gpt-5.4","created_at":1777190401,"status":"completed","usage":{"input_tokens":6,"output_tokens":2,"total_tokens":8},"output":[]}}`,
		}
		for _, event := range events {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(event)); err != nil {
				t.Fatalf("WriteMessage error: %v", err)
			}
		}
	}))
	defer server.Close()

	executor := NewCodexWebsocketsExecutor(&config.Config{})
	auth := &cliproxyauth.Auth{
		Provider: "codex",
		Attributes: map[string]string{
			"base_url": server.URL,
			"api_key":  "test-token",
		},
	}

	resp, err := executor.Execute(context.Background(), auth, cliproxyexecutor.Request{
		Model:   "gpt-5.4",
		Payload: []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"Reply with exactly OK"}]}`),
	}, cliproxyexecutor.Options{
		SourceFormat: sdktranslator.FromString("openai"),
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if gotPath != "/responses" {
		t.Fatalf("path = %q, want %q", gotPath, "/responses")
	}
	if got := gjson.GetBytes(gotRequest, "type").String(); got != "response.create" {
		t.Fatalf("request type = %q, want %q body=%s", got, "response.create", string(gotRequest))
	}
	if got := gjson.GetBytes(gotRequest, "stream").Bool(); !got {
		t.Fatalf("expected websocket upstream stream=true body=%s", string(gotRequest))
	}
	if got := gjson.GetBytes(resp.Payload, "choices.0.message.content").String(); got != "OK" {
		t.Fatalf("choices[0].message.content = %q, want %q body=%s", got, "OK", string(resp.Payload))
	}
	if got := gjson.GetBytes(resp.Payload, "usage.total_tokens").Int(); got != 8 {
		t.Fatalf("usage.total_tokens = %d, want 8 body=%s", got, string(resp.Payload))
	}
}
