package management

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/executor"
	sdktranslator "github.com/router-for-me/CLIProxyAPI/v6/sdk/translator"
)

type authProbeBatchJobStatus string

const (
	authProbeBatchJobQueued    authProbeBatchJobStatus = "queued"
	authProbeBatchJobRunning   authProbeBatchJobStatus = "running"
	authProbeBatchJobCompleted authProbeBatchJobStatus = "completed"
	authProbeBatchJobFailed    authProbeBatchJobStatus = "failed"
)

const (
	authProbeBatchQueueSize = 8
	authProbeBatchMaxJobs   = 64
	authProbeBatchRetainFor = 24 * time.Hour
	authProbeBatchPollEvery = 2 * time.Second
)

type authProbeBatchStartRequest struct {
	Query *authFileListQuery `json:"query,omitempty"`
	Names []string           `json:"names,omitempty"`
}

type authProbeSingleRequest struct {
	Name string `json:"name"`
}

type authProbeBatchJob struct {
	ID          string                  `json:"id"`
	Status      authProbeBatchJobStatus `json:"status"`
	CreatedAt   time.Time               `json:"created_at"`
	StartedAt   *time.Time              `json:"started_at,omitempty"`
	FinishedAt  *time.Time              `json:"finished_at,omitempty"`
	Total       int                     `json:"total"`
	Processed   int                     `json:"processed"`
	Succeeded   int                     `json:"succeeded"`
	Deleted     int                     `json:"deleted"`
	Failed      int                     `json:"failed"`
	CurrentFile string                  `json:"current_file,omitempty"`
	LastError   string                  `json:"last_error,omitempty"`
}

type authProbeBatchQueueItem struct {
	job   *authProbeBatchJob
	names []string
}

func (h *Handler) ensureAuthProbeBatchWorker() {
	if h == nil {
		return
	}
	h.authProbeBatchWorkerOnce.Do(func() {
		h.authProbeBatchQueue = make(chan authProbeBatchQueueItem, authProbeBatchQueueSize)
		if h.authProbeBatchJobs == nil {
			h.authProbeBatchJobs = make(map[string]*authProbeBatchJob)
		}
		go h.runAuthProbeBatchWorker()
	})
}

func (h *Handler) runAuthProbeBatchWorker() {
	ticker := time.NewTicker(authProbeBatchRetainFor)
	defer ticker.Stop()

	for {
		select {
		case item := <-h.authProbeBatchQueue:
			h.runAuthProbeBatchJob(item)
		case <-ticker.C:
			h.pruneAuthProbeBatchJobs()
		}
	}
}

func (h *Handler) runAuthProbeBatchJob(item authProbeBatchQueueItem) {
	if h == nil || item.job == nil {
		return
	}
	startedAt := time.Now().UTC()

	h.authProbeBatchMu.Lock()
	job := h.authProbeBatchJobs[item.job.ID]
	if job == nil {
		h.authProbeBatchMu.Unlock()
		return
	}
	job.Status = authProbeBatchJobRunning
	job.StartedAt = &startedAt
	h.authProbeBatchMu.Unlock()

	concurrency := h.authProbeBatchConcurrency()
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, name := range item.names {
		name := name
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			h.authProbeBatchMu.Lock()
			job = h.authProbeBatchJobs[item.job.ID]
			if job == nil {
				h.authProbeBatchMu.Unlock()
				return
			}
			job.CurrentFile = name
			h.authProbeBatchMu.Unlock()

			auth := h.findAuthForDelete(name)
			if auth == nil {
				h.authProbeBatchMu.Lock()
				job = h.authProbeBatchJobs[item.job.ID]
				if job != nil {
					job.Processed++
					job.Failed++
				}
				h.authProbeBatchMu.Unlock()
				return
			}

			outcome, err := h.probeAuthFileFirstPacket(context.Background(), auth)
			h.authProbeBatchMu.Lock()
			job = h.authProbeBatchJobs[item.job.ID]
			if job == nil {
				h.authProbeBatchMu.Unlock()
				return
			}
			job.Processed++
			if err != nil {
				job.Failed++
				job.LastError = err.Error()
			} else if outcome.deleted {
				job.Deleted++
			} else if outcome.success {
				job.Succeeded++
			}
			if job.Processed >= job.Total {
				job.CurrentFile = ""
			}
			h.authProbeBatchMu.Unlock()
		}()
	}
	wg.Wait()

	finishedAt := time.Now().UTC()
	h.authProbeBatchMu.Lock()
	job = h.authProbeBatchJobs[item.job.ID]
	if job != nil {
		job.Status = authProbeBatchJobCompleted
		job.FinishedAt = &finishedAt
		job.CurrentFile = ""
	}
	h.authProbeBatchMu.Unlock()
}

func (h *Handler) authProbeBatchConcurrency() int {
	if h == nil || h.cfg == nil {
		return 1
	}
	value := h.cfg.AuthProbeBatch.Concurrency
	switch {
	case value <= 0:
		return 1
	case value > 64:
		return 64
	default:
		return value
	}
}

func (h *Handler) pruneAuthProbeBatchJobs() {
	if h == nil {
		return
	}
	cutoff := time.Now().UTC().Add(-authProbeBatchRetainFor)
	h.authProbeBatchMu.Lock()
	defer h.authProbeBatchMu.Unlock()
	for id, job := range h.authProbeBatchJobs {
		if job == nil {
			delete(h.authProbeBatchJobs, id)
			continue
		}
		if job.CreatedAt.Before(cutoff) {
			delete(h.authProbeBatchJobs, id)
		}
	}
	if len(h.authProbeBatchJobs) <= authProbeBatchMaxJobs {
		return
	}
	type pair struct {
		id string
		at time.Time
	}
	pairs := make([]pair, 0, len(h.authProbeBatchJobs))
	for id, job := range h.authProbeBatchJobs {
		pairs = append(pairs, pair{id: id, at: job.CreatedAt})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].at.Before(pairs[j].at) })
	for idx := 0; idx < len(pairs)-authProbeBatchMaxJobs; idx++ {
		delete(h.authProbeBatchJobs, pairs[idx].id)
	}
}

func (h *Handler) StartAuthProbeBatch(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}

	var req authProbeBatchStartRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
	}

	var names []string
	if len(req.Names) > 0 {
		names = uniqueAuthFileNames(req.Names)
	} else {
		query, err := parseAuthFileListQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Query != nil {
			query = *req.Query
		}
		names = h.authFileNamesMatchingQuery(query)
	}
	if len(names) == 0 {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "job": &authProbeBatchJob{
			ID:         "",
			Status:     authProbeBatchJobCompleted,
			CreatedAt:  time.Now().UTC(),
			FinishedAt: ptrTime(time.Now().UTC()),
			Total:      0,
		}})
		return
	}

	jobID, err := newAuthProbeBatchID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	job := &authProbeBatchJob{
		ID:        jobID,
		Status:    authProbeBatchJobQueued,
		CreatedAt: time.Now().UTC(),
		Total:     len(names),
	}

	h.ensureAuthProbeBatchWorker()
	h.authProbeBatchMu.Lock()
	if h.authProbeBatchJobs == nil {
		h.authProbeBatchJobs = make(map[string]*authProbeBatchJob)
	}
	h.authProbeBatchJobs[jobID] = job
	h.authProbeBatchMu.Unlock()

	select {
	case h.authProbeBatchQueue <- authProbeBatchQueueItem{job: job, names: names}:
		c.JSON(http.StatusAccepted, gin.H{"status": "queued", "job": job})
	default:
		h.authProbeBatchMu.Lock()
		delete(h.authProbeBatchJobs, jobID)
		h.authProbeBatchMu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "probe queue is full"})
	}
}

func (h *Handler) GetAuthProbeBatch(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler not initialized"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	h.authProbeBatchMu.RLock()
	job := h.authProbeBatchJobs[id]
	h.authProbeBatchMu.RUnlock()
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "probe job not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": job})
}

type probeFirstPacketOutcome struct {
	success      bool
	deleted      bool
	statusCode   int
	errorCode    string
	errorMessage string
	model        string
	modelSource  string
}

func (h *Handler) ProbeAuthFile(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}

	var req authProbeSingleRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(c.Query("name"))
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	auth := h.findAuthForDelete(name)
	if auth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth file not found"})
		return
	}

	outcome, err := h.probeAuthFileFirstPacket(c.Request.Context(), auth)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"status":             "ok",
		"name":               auth.FileName,
		"auth_id":            auth.ID,
		"provider":           authProviderKey(auth),
		"success":            outcome.success,
		"deleted":            outcome.deleted,
		"status_code":        nil,
		"error_code":         nil,
		"error_message":      nil,
		"probe_model":        outcome.model,
		"probe_model_source": outcome.modelSource,
	}
	if response["name"] == "" {
		response["name"] = auth.ID
	}
	if outcome.statusCode > 0 {
		response["status_code"] = outcome.statusCode
	}
	if strings.TrimSpace(outcome.errorCode) != "" {
		response["error_code"] = outcome.errorCode
	}
	if strings.TrimSpace(outcome.errorMessage) != "" {
		response["error_message"] = outcome.errorMessage
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) authProbeStatusWouldDelete(statusCode int) bool {
	if statusCode == 0 {
		return false
	}
	switch statusCode {
	case http.StatusUnauthorized:
		if h == nil || h.cfg == nil {
			return true
		}
		return h.cfg.AuthAutoDelete.Unauthorized
	case http.StatusTooManyRequests:
		if h == nil || h.cfg == nil {
			return true
		}
		return h.cfg.AuthAutoDelete.RateLimited
	default:
		return false
	}
}

func buildProbeOutcomeFromError(err error, selection authProbeModelSelection) (probeFirstPacketOutcome, bool) {
	normalized := coreauth.NormalizeError(err)
	if normalized == nil {
		return probeFirstPacketOutcome{}, false
	}
	return probeFirstPacketOutcome{
		statusCode:   normalized.HTTPStatus,
		errorCode:    strings.TrimSpace(normalized.Code),
		errorMessage: strings.TrimSpace(normalized.Message),
		model:        selection.Model,
		modelSource:  selection.Source,
	}, true
}

func (h *Handler) markProbeErrorResult(ctx context.Context, auth *coreauth.Auth, provider string, selection authProbeModelSelection, err error) {
	if h == nil || h.authManager == nil || auth == nil {
		return
	}
	normalized := coreauth.NormalizeError(err)
	h.authManager.MarkResult(ctx, coreauth.Result{
		AuthID:     auth.ID,
		Provider:   provider,
		Model:      selection.Model,
		Success:    false,
		StatusCode: 0,
		Error:      normalized,
	})
}

func (h *Handler) probeAuthFileFirstPacket(ctx context.Context, auth *coreauth.Auth) (probeFirstPacketOutcome, error) {
	if h == nil || h.authManager == nil || auth == nil {
		return probeFirstPacketOutcome{}, fmt.Errorf("auth probe is unavailable")
	}

	provider := authProviderKey(auth)
	selection := probeModelSelectionForAuth(h.cfg, auth)
	if provider == "" || selection.Model == "" {
		return probeFirstPacketOutcome{}, fmt.Errorf("probe model unavailable")
	}
	payload, err := buildAuthProbeStreamPayload(selection.Model)
	if err != nil {
		return probeFirstPacketOutcome{}, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req := cliproxyexecutor.Request{
		Model:   selection.Model,
		Payload: payload,
	}
	opts := cliproxyexecutor.Options{
		SourceFormat:    sdktranslator.FromString("openai"),
		OriginalRequest: payload,
		Metadata: map[string]any{
			cliproxyexecutor.PinnedAuthMetadataKey: auth.ID,
		},
	}

	var (
		streamResult *cliproxyexecutor.StreamResult
	)
	if selection.Routeable {
		streamResult, err = h.authManager.ExecuteStream(probeCtx, []string{provider}, req, opts)
	} else {
		executor, ok := h.authManager.Executor(provider)
		if !ok || executor == nil {
			return probeFirstPacketOutcome{}, fmt.Errorf("executor not found for provider %s", provider)
		}
		streamResult, err = executor.ExecuteStream(probeCtx, auth, req, opts)
		if err != nil {
			h.markProbeErrorResult(probeCtx, auth, provider, selection, err)
		}
	}
	if err != nil {
		outcome, ok := buildProbeOutcomeFromError(err, selection)
		if !ok {
			return probeFirstPacketOutcome{}, err
		}
		outcome.deleted = h.authProbeStatusWouldDelete(outcome.statusCode)
		return outcome, nil
	}
	if streamResult == nil || streamResult.Chunks == nil {
		if outcome, ok := buildProbeOutcomeFromError(fmt.Errorf("empty stream result"), selection); ok {
			return outcome, nil
		}
		return probeFirstPacketOutcome{}, fmt.Errorf("empty stream result")
	}

	select {
	case <-probeCtx.Done():
		if outcome, ok := buildProbeOutcomeFromError(probeCtx.Err(), selection); ok {
			return outcome, nil
		}
		return probeFirstPacketOutcome{}, probeCtx.Err()
	case chunk, ok := <-streamResult.Chunks:
		if !ok {
			if outcome, normalized := buildProbeOutcomeFromError(fmt.Errorf("stream closed before first packet"), selection); normalized {
				if !selection.Routeable {
					h.markProbeErrorResult(probeCtx, auth, provider, selection, fmt.Errorf("stream closed before first packet"))
				}
				return outcome, nil
			}
			return probeFirstPacketOutcome{}, fmt.Errorf("stream closed before first packet")
		}
		if chunk.Err != nil {
			if !selection.Routeable {
				h.markProbeErrorResult(probeCtx, auth, provider, selection, chunk.Err)
			}
			outcome, ok := buildProbeOutcomeFromError(chunk.Err, selection)
			if !ok {
				return probeFirstPacketOutcome{}, chunk.Err
			}
			outcome.deleted = h.authProbeStatusWouldDelete(outcome.statusCode)
			return outcome, nil
		}
		cancel()
		h.authManager.MarkResult(coreauth.WithSkipPersist(context.Background()), coreauth.Result{
			AuthID:     auth.ID,
			Provider:   provider,
			Model:      selection.Model,
			Success:    true,
			StatusCode: http.StatusOK,
		})
		return probeFirstPacketOutcome{
			success:     true,
			statusCode:  http.StatusOK,
			model:       selection.Model,
			modelSource: selection.Source,
		}, nil
	}
}

func newAuthProbeBatchID() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("failed to generate probe job id: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
