package auth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeResultError_Timeout(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  http.MethodPost,
		URL: "https://chatgpt.com/backend-api/codex/responses",
		Err: context.DeadlineExceeded,
	}

	normalized := normalizeResultError(err)
	if normalized == nil {
		t.Fatal("expected normalized error")
	}
	if normalized.Code != "timeout" {
		t.Fatalf("expected timeout code, got %q", normalized.Code)
	}
	if normalized.HTTPStatus != http.StatusRequestTimeout {
		t.Fatalf("expected synthetic status %d, got %d", http.StatusRequestTimeout, normalized.HTTPStatus)
	}
	if !strings.Contains(strings.ToLower(normalized.Message), "deadline exceeded") {
		t.Fatalf("expected timeout message to be preserved, got %q", normalized.Message)
	}
}

func TestNormalizeResultError_ExtractsStructuredDetails(t *testing.T) {
	t.Parallel()

	err := &Error{
		HTTPStatus: http.StatusBadRequest,
		Message:    `{"error":{"type":"unsupported_parameter","message":"Unsupported parameter: background"}}`,
	}

	normalized := normalizeResultError(err)
	if normalized == nil {
		t.Fatal("expected normalized error")
	}
	if normalized.Code != "unsupported_parameter" {
		t.Fatalf("expected unsupported_parameter code, got %q", normalized.Code)
	}
	if normalized.Message != "Unsupported parameter: background" {
		t.Fatalf("expected normalized message, got %q", normalized.Message)
	}
	if normalized.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected http status %d, got %d", http.StatusBadRequest, normalized.HTTPStatus)
	}
}

func TestNormalizeResultError_EOFMapsToBadGateway(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  http.MethodPost,
		URL: "https://chatgpt.com/backend-api/codex/responses",
		Err: io.EOF,
	}

	normalized := normalizeResultError(err)
	if normalized == nil {
		t.Fatal("expected normalized error")
	}
	if normalized.Code != "upstream_closed" {
		t.Fatalf("expected upstream_closed code, got %q", normalized.Code)
	}
	if normalized.HTTPStatus != http.StatusBadGateway {
		t.Fatalf("expected synthetic status %d, got %d", http.StatusBadGateway, normalized.HTTPStatus)
	}
	if !strings.Contains(strings.ToLower(normalized.Message), "eof") {
		t.Fatalf("expected eof message to be preserved, got %q", normalized.Message)
	}
}
