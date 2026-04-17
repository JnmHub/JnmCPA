package authdeletestats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestManagerSnapshotAggregates401And429(t *testing.T) {
	manager := NewManager(30*24*time.Hour, 2048)
	base := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)

	manager.Record(Event{Timestamp: base.Add(-50 * time.Minute), StatusCode: StatusUnauthorized})
	manager.Record(Event{Timestamp: base.Add(-20 * time.Minute), StatusCode: StatusTooMany})
	manager.Record(Event{Timestamp: base.Add(-10 * time.Minute), StatusCode: StatusUnauthorized})
	manager.Record(Event{Timestamp: base.Add(-2 * time.Hour), StatusCode: StatusUnauthorized})

	snapshot := manager.Snapshot(SnapshotOptions{
		Now:        base,
		Range:      "1h",
		BucketSize: 15 * time.Minute,
	})

	if snapshot.Totals.All != 3 {
		t.Fatalf("Totals.All = %d, want 3", snapshot.Totals.All)
	}
	if snapshot.Totals.Unauthorized401 != 2 {
		t.Fatalf("Totals.Unauthorized401 = %d, want 2", snapshot.Totals.Unauthorized401)
	}
	if snapshot.Totals.RateLimited429 != 1 {
		t.Fatalf("Totals.RateLimited429 = %d, want 1", snapshot.Totals.RateLimited429)
	}

	found401Bucket := false
	found429Bucket := false
	for _, bucket := range snapshot.Series {
		if bucket.Unauthorized401 > 0 {
			found401Bucket = true
		}
		if bucket.RateLimited429 > 0 {
			found429Bucket = true
		}
	}
	if !found401Bucket {
		t.Fatalf("expected at least one bucket with 401 counts")
	}
	if !found429Bucket {
		t.Fatalf("expected at least one bucket with 429 counts")
	}
}

func TestManagerOnResultOnlyRecordsAutoDeleteStatuses(t *testing.T) {
	manager := NewManager(24*time.Hour, 512)

	manager.OnResult(context.Background(), coreauth.Result{
		Success: false,
		Error: &coreauth.Error{
			HTTPStatus: StatusUnauthorized,
		},
	})
	manager.OnResult(context.Background(), coreauth.Result{
		Success: false,
		Error: &coreauth.Error{
			HTTPStatus: 500,
		},
	})
	manager.OnResult(context.Background(), coreauth.Result{
		Success: true,
		Error: &coreauth.Error{
			HTTPStatus: StatusTooMany,
		},
	})

	snapshot := manager.Snapshot(SnapshotOptions{
		Now:        time.Now().UTC(),
		Range:      "24h",
		BucketSize: time.Hour,
	})
	if snapshot.Totals.All != 1 {
		t.Fatalf("Totals.All = %d, want 1", snapshot.Totals.All)
	}
	if snapshot.Totals.Unauthorized401 != 1 {
		t.Fatalf("Totals.Unauthorized401 = %d, want 1", snapshot.Totals.Unauthorized401)
	}
	if snapshot.Totals.RateLimited429 != 0 {
		t.Fatalf("Totals.RateLimited429 = %d, want 0", snapshot.Totals.RateLimited429)
	}
}

func TestManagerConfigureSQLitePersistsMinuteBuckets(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "auth-delete-stats.db")
	base := time.Date(2026, 4, 16, 12, 34, 0, 0, time.UTC)

	manager := NewManager(24*time.Hour, 512)
	manager.Record(Event{Timestamp: base, StatusCode: StatusUnauthorized})
	manager.Record(Event{Timestamp: base.Add(10 * time.Second), StatusCode: StatusTooMany})

	if err := manager.ConfigureSQLite(dbPath); err != nil {
		t.Fatalf("ConfigureSQLite() error = %v", err)
	}
	defer func() {
		_ = manager.Close()
	}()

	reloaded := NewManager(24*time.Hour, 512)
	if err := reloaded.ConfigureSQLite(dbPath); err != nil {
		t.Fatalf("reloaded.ConfigureSQLite() error = %v", err)
	}
	defer func() {
		_ = reloaded.Close()
	}()

	snapshot := reloaded.Snapshot(SnapshotOptions{
		From:       base.Add(-time.Minute),
		To:         base.Add(time.Minute),
		BucketSize: time.Minute,
	})

	if snapshot.Totals.All != 2 {
		t.Fatalf("Totals.All = %d, want 2", snapshot.Totals.All)
	}
	if snapshot.Totals.Unauthorized401 != 1 {
		t.Fatalf("Totals.Unauthorized401 = %d, want 1", snapshot.Totals.Unauthorized401)
	}
	if snapshot.Totals.RateLimited429 != 1 {
		t.Fatalf("Totals.RateLimited429 = %d, want 1", snapshot.Totals.RateLimited429)
	}
}
