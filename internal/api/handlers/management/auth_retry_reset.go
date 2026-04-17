package management

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type authRetryResetRequest struct {
	Query *authFileListQuery `json:"query,omitempty"`
	Names []string           `json:"names,omitempty"`
}

func clearAuthRetryState(auth *coreauth.Auth, now time.Time) bool {
	if auth == nil {
		return false
	}

	changed := false
	if auth.Unavailable {
		auth.Unavailable = false
		changed = true
	}
	if !auth.NextRetryAfter.IsZero() {
		auth.NextRetryAfter = time.Time{}
		changed = true
	}
	if auth.Quota.Exceeded || !auth.Quota.NextRecoverAt.IsZero() || auth.Quota.BackoffLevel != 0 || auth.Quota.Reason != "" {
		auth.Quota = coreauth.QuotaState{}
		changed = true
	}

	for _, state := range auth.ModelStates {
		if state == nil {
			continue
		}
		stateChanged := false
		if state.Unavailable {
			state.Unavailable = false
			stateChanged = true
		}
		if !state.NextRetryAfter.IsZero() {
			state.NextRetryAfter = time.Time{}
			stateChanged = true
		}
		if state.Quota.Exceeded || !state.Quota.NextRecoverAt.IsZero() || state.Quota.BackoffLevel != 0 || state.Quota.Reason != "" {
			state.Quota = coreauth.QuotaState{}
			stateChanged = true
		}
		if stateChanged {
			state.UpdatedAt = now
			changed = true
		}
	}

	if !changed {
		return false
	}

	auth.UpdatedAt = now
	return true
}

func authRetryResetModelIDs(auth *coreauth.Auth) []string {
	if auth == nil {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(auth.ModelStates))
	for model := range auth.ModelStates {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	for _, model := range registry.GetGlobalRegistry().GetModelsForClient(auth.ID) {
		if model == nil {
			continue
		}
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			modelID = strings.TrimSpace(model.Name)
		}
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		result = append(result, modelID)
	}
	return result
}

// ResetAllAuthRetryTimes clears cooldown / retry timing for all auths without deleting historical status info.
func (h *Handler) ResetAllAuthRetryTimes(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "core auth manager unavailable"})
		return
	}

	var req authRetryResetRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
	}

	ctx := c.Request.Context()
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

	auths := make([]*coreauth.Auth, 0, len(names))
	for _, name := range names {
		auth := h.findAuthForDelete(name)
		if auth == nil {
			continue
		}
		auths = append(auths, auth)
	}
	now := time.Now()
	resetCount := 0

	for _, auth := range auths {
		if auth == nil || strings.TrimSpace(auth.ID) == "" {
			continue
		}
		if !clearAuthRetryState(auth, now) {
			continue
		}

		for _, modelID := range authRetryResetModelIDs(auth) {
			registry.GetGlobalRegistry().ResumeClientModel(auth.ID, modelID)
			registry.GetGlobalRegistry().ClearModelQuotaExceeded(auth.ID, modelID)
		}

		updated, err := h.authManager.Update(ctx, auth)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		h.emitRuntimeAuthUpsert(ctx, watcher.AuthUpdateActionModify, updated)
		resetCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"reset":  resetCount,
		"total":  len(auths),
	})
}
