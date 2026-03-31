package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

func intPtr(value int) *int { return &value }

func TestManagerMarkResultUsesConfiguredCooldowns(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	manager.SetConfig(&internalconfig.Config{
		ErrorCooldowns: internalconfig.ErrorCooldowns{
			PaymentRequiredSeconds:   intPtr(7),
			NotFoundSeconds:          intPtr(11),
			ModelNotSupportedSeconds: intPtr(13),
			TransientErrorSeconds:    intPtr(17),
		},
	})

	auth := &Auth{ID: "auth-cooldown", Provider: "claude"}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	start := time.Now()
	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: "claude",
		Model:    "claude-sonnet",
		Success:  false,
		Error: &Error{
			HTTPStatus: http.StatusForbidden,
			Message:    "forbidden",
		},
	})

	updated, ok := manager.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain registered")
	}
	state := updated.ModelStates["claude-sonnet"]
	if state == nil {
		t.Fatalf("expected model state to be created")
	}
	if diff := state.NextRetryAfter.Sub(start); diff < 6*time.Second || diff > 9*time.Second {
		t.Fatalf("expected configured 403 cooldown around 7s, got %v", diff)
	}

	start = time.Now()
	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: "claude",
		Model:    "claude-opus",
		Success:  false,
		Error: &Error{
			HTTPStatus: http.StatusBadRequest,
			Message:    "requested model is not supported",
		},
	})

	updated, ok = manager.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain registered after model support failure")
	}
	state = updated.ModelStates["claude-opus"]
	if state == nil {
		t.Fatalf("expected model support state to be created")
	}
	if diff := state.NextRetryAfter.Sub(start); diff < 12*time.Second || diff > 15*time.Second {
		t.Fatalf("expected configured model support cooldown around 13s, got %v", diff)
	}

	start = time.Now()
	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: "claude",
		Model:    "claude-haiku",
		Success:  false,
		Error: &Error{
			HTTPStatus: http.StatusBadGateway,
			Message:    "upstream unavailable",
		},
	})

	updated, ok = manager.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain registered after transient failure")
	}
	state = updated.ModelStates["claude-haiku"]
	if state == nil {
		t.Fatalf("expected transient state to be created")
	}
	if diff := state.NextRetryAfter.Sub(start); diff < 16*time.Second || diff > 19*time.Second {
		t.Fatalf("expected configured transient cooldown around 17s, got %v", diff)
	}
}

func TestManagerMarkResultUsesConfiguredRateLimitBackoff(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	manager.SetConfig(&internalconfig.Config{
		ErrorCooldowns: internalconfig.ErrorCooldowns{
			RateLimitBaseSeconds: intPtr(3),
			RateLimitMaxSeconds:  intPtr(9),
		},
	})

	auth := &Auth{ID: "auth-rate-limit", Provider: "gemini"}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}

	start := time.Now()
	manager.MarkResult(context.Background(), Result{
		AuthID:   auth.ID,
		Provider: "gemini",
		Model:    "gemini-pro",
		Success:  false,
		Error: &Error{
			HTTPStatus: http.StatusTooManyRequests,
			Message:    "quota exhausted",
		},
	})

	updated, ok := manager.GetByID(auth.ID)
	if !ok || updated == nil {
		t.Fatalf("expected auth to remain registered")
	}
	state := updated.ModelStates["gemini-pro"]
	if state == nil {
		t.Fatalf("expected rate limit state to be created")
	}
	if diff := state.NextRetryAfter.Sub(start); diff < 2*time.Second || diff > 5*time.Second {
		t.Fatalf("expected configured 429 base cooldown around 3s, got %v", diff)
	}
}
