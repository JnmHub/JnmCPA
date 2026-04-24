package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiCLIEndpointDisabledReturns404(t *testing.T) {
	server := newTestServer(t)
	server.cfg.EnableGeminiCLIEndpoint = false

	req := httptest.NewRequest(http.MethodPost, "/v1internal:generateContent", strings.NewReader(`{}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "127.0.0.1"

	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d want %d; body=%s", rr.Code, http.StatusNotFound, rr.Body.String())
	}
}

func TestGeminiCLIEndpointEnabledIsReachable(t *testing.T) {
	server := newTestServer(t)
	server.cfg.EnableGeminiCLIEndpoint = true

	req := httptest.NewRequest(http.MethodPost, "/v1internal:generateContent", strings.NewReader(`{"model":"gemini-2.5-pro"}`))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "127.0.0.1"

	rr := httptest.NewRecorder()
	server.engine.ServeHTTP(rr, req)

	if rr.Code == http.StatusNotFound {
		t.Fatalf("unexpected status code: got %d; body=%s", rr.Code, rr.Body.String())
	}
}
