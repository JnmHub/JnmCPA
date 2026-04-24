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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
)

type probeBatchExecutor struct {
	mu          sync.Mutex
	streamErrBy map[string]error
	modelByAuth map[string]string
}

func (e *probeBatchExecutor) Identifier() string { return "codex" }

func (e *probeBatchExecutor) Execute(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, fmt.Errorf("not implemented")
}

func (e *probeBatchExecutor) ExecuteStream(_ context.Context, auth *coreauth.Auth, req coreexecutor.Request, _ coreexecutor.Options) (*coreexecutor.StreamResult, error) {
	e.mu.Lock()
	if e.modelByAuth == nil {
		e.modelByAuth = make(map[string]string)
	}
	if auth != nil {
		e.modelByAuth[auth.ID] = req.Model
	}
	var err error
	if auth != nil && e.streamErrBy != nil {
		err = e.streamErrBy[auth.ID]
	}
	e.mu.Unlock()
	if err != nil {
		return nil, err
	}

	chunks := make(chan coreexecutor.StreamChunk, 1)
	chunks <- coreexecutor.StreamChunk{Payload: []byte("data: ok\n\n")}
	close(chunks)
	return &coreexecutor.StreamResult{Chunks: chunks}, nil
}

func (e *probeBatchExecutor) Refresh(context.Context, *coreauth.Auth) (*coreauth.Auth, error) {
	return nil, fmt.Errorf("not implemented")
}

func (e *probeBatchExecutor) CountTokens(context.Context, *coreauth.Auth, coreexecutor.Request, coreexecutor.Options) (coreexecutor.Response, error) {
	return coreexecutor.Response{}, fmt.Errorf("not implemented")
}

func (e *probeBatchExecutor) HttpRequest(context.Context, *coreauth.Auth, *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func registerProbeBatchAuth(t *testing.T, manager *coreauth.Manager, id, fileName string) *coreauth.Auth {
	t.Helper()

	auth := &coreauth.Auth{
		ID:       id,
		FileName: fileName,
		Provider: "codex",
		Attributes: map[string]string{
			"path": "/tmp/" + fileName,
		},
	}
	if _, err := manager.Register(context.Background(), auth); err != nil {
		t.Fatalf("register auth %s: %v", id, err)
	}
	return auth
}

func waitForProbeBatchJob(t *testing.T, h *Handler, id string) *authProbeBatchJob {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		h.authProbeBatchMu.RLock()
		job := h.authProbeBatchJobs[id]
		if job != nil && job.Status == authProbeBatchJobCompleted {
			snapshot := *job
			h.authProbeBatchMu.RUnlock()
			return &snapshot
		}
		h.authProbeBatchMu.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("probe batch job %q did not complete before timeout", id)
	return nil
}

func TestStartAuthProbeBatch_QueuesAndReportsCompletedJob(t *testing.T) {
	t.Setenv("MANAGEMENT_PASSWORD", "")
	gin.SetMode(gin.TestMode)

	store := &memoryAuthStore{}
	manager := coreauth.NewManager(store, nil, nil)
	cfg := &config.Config{
		AuthAutoDelete: config.AuthAutoDeleteConfig{
			Unauthorized: true,
			RateLimited:  false,
		},
		AuthProbeBatch: config.AuthProbeBatchConfig{
			Concurrency: 2,
		},
	}
	manager.SetConfig(cfg)

	executor := &probeBatchExecutor{
		streamErrBy: map[string]error{
			"probe-batch-401": &coreauth.Error{
				HTTPStatus: http.StatusUnauthorized,
				Code:       "token_revoked",
				Message:    "token revoked",
			},
		},
	}
	manager.RegisterExecutor(executor)
	registerProbeBatchAuth(t, manager, "probe-batch-ok", "probe-batch-ok.json")
	registerProbeBatchAuth(t, manager, "probe-batch-401", "probe-batch-401.json")

	h := NewHandlerWithoutConfigFilePath(cfg, manager)

	body := bytes.NewBufferString(`{"names":["probe-batch-ok.json","probe-batch-401.json"]}`)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v0/management/auth-files/probe-batch", body)
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.StartAuthProbeBatch(ctx)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusAccepted, rec.Code, rec.Body.String())
	}

	var startPayload struct {
		Status string             `json:"status"`
		Job    *authProbeBatchJob `json:"job"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &startPayload); err != nil {
		t.Fatalf("decode start payload: %v", err)
	}
	if startPayload.Status != "queued" {
		t.Fatalf("expected queued status, got %q", startPayload.Status)
	}
	if startPayload.Job == nil || startPayload.Job.ID == "" {
		t.Fatalf("expected queued job with id, got %#v", startPayload.Job)
	}

	completed := waitForProbeBatchJob(t, h, startPayload.Job.ID)
	if completed.Total != 2 || completed.Processed != 2 {
		t.Fatalf("unexpected job counts: %+v", completed)
	}
	if completed.Succeeded != 1 {
		t.Fatalf("expected 1 succeeded auth, got %+v", completed)
	}
	if completed.Deleted != 1 {
		t.Fatalf("expected 1 deleted auth, got %+v", completed)
	}
	if completed.Failed != 0 {
		t.Fatalf("expected 0 failed auths, got %+v", completed)
	}
	if completed.CurrentFile != "" {
		t.Fatalf("expected empty current_file after completion, got %+v", completed)
	}

	if _, ok := manager.GetByID("probe-batch-401"); ok {
		t.Fatalf("expected unauthorized auth to be deleted after batch probe")
	}
	if _, ok := manager.GetByID("probe-batch-ok"); !ok {
		t.Fatalf("expected healthy auth to remain after batch probe")
	}

	getRec := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRec)
	getCtx.Params = gin.Params{{Key: "id", Value: startPayload.Job.ID}}
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files/probe-batch/"+startPayload.Job.ID, nil)

	h.GetAuthProbeBatch(getCtx)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusOK, getRec.Code, getRec.Body.String())
	}

	var getPayload struct {
		Job *authProbeBatchJob `json:"job"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("decode get payload: %v", err)
	}
	if getPayload.Job == nil {
		t.Fatalf("expected job payload, got nil")
	}
	if getPayload.Job.Status != authProbeBatchJobCompleted {
		t.Fatalf("expected completed job, got %+v", getPayload.Job)
	}
}

func TestGetAuthProbeBatch_ReturnsNotFoundForUnknownJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewHandlerWithoutConfigFilePath(&config.Config{}, nil)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Params = gin.Params{{Key: "id", Value: "missing-job"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/auth-files/probe-batch/missing-job", nil)

	h.GetAuthProbeBatch(ctx)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d with body %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}
