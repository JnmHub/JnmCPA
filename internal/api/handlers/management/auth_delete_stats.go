package management

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/authdeletestats"
)

// GetAuthDeleteStats returns aggregated auto-delete counts for 401/429 auth removals.
func (h *Handler) GetAuthDeleteStats(c *gin.Context) {
	if h == nil || h.authDeleteStats == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth delete statistics unavailable"})
		return
	}

	opts, err := parseAuthDeleteSnapshotOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.authDeleteStats.Snapshot(opts))
}

func parseAuthDeleteSnapshotOptions(c *gin.Context) (authdeletestats.SnapshotOptions, error) {
	now := time.Now().UTC()
	opts := authdeletestats.SnapshotOptions{
		Now:   now,
		Range: strings.TrimSpace(c.Query("range")),
	}

	if fromRaw := strings.TrimSpace(c.Query("from")); fromRaw != "" {
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			return authdeletestats.SnapshotOptions{}, err
		}
		opts.From = from.UTC()
	}

	if toRaw := strings.TrimSpace(c.Query("to")); toRaw != "" {
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			return authdeletestats.SnapshotOptions{}, err
		}
		opts.To = to.UTC()
	}

	if bucketRaw := strings.TrimSpace(c.Query("bucket")); bucketRaw != "" {
		bucket, err := authdeletestats.ParseFlexibleDuration(bucketRaw)
		if err != nil {
			return authdeletestats.SnapshotOptions{}, err
		}
		if bucket <= 0 {
			return authdeletestats.SnapshotOptions{}, fmt.Errorf("invalid bucket duration")
		}
		opts.BucketSize = bucket
	}

	return opts, nil
}
