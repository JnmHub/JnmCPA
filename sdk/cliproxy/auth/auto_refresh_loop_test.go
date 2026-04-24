package auth

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	internalconfig "github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type autoRefreshTestExecutor struct {
	provider string
	calls    atomic.Int32
}

func (e *autoRefreshTestExecutor) Identifier() string { return e.provider }

func (e *autoRefreshTestExecutor) Execute(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *autoRefreshTestExecutor) ExecuteStream(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	return nil, nil
}

func (e *autoRefreshTestExecutor) Refresh(ctx context.Context, auth *Auth) (*Auth, error) {
	e.calls.Add(1)
	return auth.Clone(), nil
}

func (e *autoRefreshTestExecutor) CountTokens(ctx context.Context, auth *Auth, req cliproxyexecutor.Request, opts cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, nil
}

func (e *autoRefreshTestExecutor) HttpRequest(ctx context.Context, auth *Auth, req *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestStartAutoRefresh_UsesConfiguredWorkers(t *testing.T) {
	t.Parallel()

	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	manager.SetConfig(&internalconfig.Config{AuthAutoRefreshWorkers: 3})
	manager.StartAutoRefresh(context.Background(), 50*time.Millisecond)
	t.Cleanup(manager.StopAutoRefresh)

	manager.mu.RLock()
	loop := manager.refreshLoop
	manager.mu.RUnlock()
	if loop == nil {
		t.Fatal("refreshLoop = nil")
	}
	if loop.concurrency != 3 {
		t.Fatalf("refreshLoop.concurrency = %d, want %d", loop.concurrency, 3)
	}
}

func TestStartAutoRefresh_RefreshesDueAuthWithoutTightLoop(t *testing.T) {
	t.Parallel()

	const provider = "auto-refresh-test"
	RegisterRefreshLeadProvider(provider, func() *time.Duration {
		d := time.Second
		return &d
	})

	manager := NewManager(nil, &RoundRobinSelector{}, nil)
	exec := &autoRefreshTestExecutor{provider: provider}
	manager.RegisterExecutor(exec)

	auth := &Auth{
		ID:       "auth-refresh-1",
		Provider: provider,
		Metadata: map[string]any{"email": "refresh@example.com"},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	manager.StartAutoRefresh(context.Background(), 20*time.Millisecond)
	t.Cleanup(manager.StopAutoRefresh)

	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		if exec.calls.Load() >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := exec.calls.Load(); got != 1 {
		t.Fatalf("refresh calls after initial wait = %d, want %d", got, 1)
	}

	time.Sleep(150 * time.Millisecond)
	if got := exec.calls.Load(); got != 1 {
		t.Fatalf("refresh calls after short stability window = %d, want %d", got, 1)
	}
}
