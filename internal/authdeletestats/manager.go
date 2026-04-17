package authdeletestats

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

const (
	StatusUnauthorized = 401
	StatusTooMany      = 429

	defaultRetention     = 90 * 24 * time.Hour
	defaultMaxBuckets    = 200000
	defaultSQLiteTimeout = 5 * time.Second
	sqliteBusyTimeout    = 5 * time.Second
)

var chinaLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return loc
}()

type bucketCounts struct {
	Unauthorized401 int
	RateLimited429  int
}

type minuteBucketRow struct {
	MinuteStart     string
	Unauthorized401 int
	RateLimited429  int
}

// Event captures one auth auto-delete triggered by a request failure.
type Event struct {
	Timestamp  time.Time `json:"timestamp"`
	StatusCode int       `json:"status_code"`
	Provider   string    `json:"provider,omitempty"`
	AuthID     string    `json:"auth_id,omitempty"`
}

// Totals summarizes deletions inside the requested time range.
type Totals struct {
	All             int `json:"all"`
	Unauthorized401 int `json:"status_401"`
	RateLimited429  int `json:"status_429"`
}

// Bucket contains one aggregated chart point.
type Bucket struct {
	StartAt         time.Time `json:"start_at"`
	EndAt           time.Time `json:"end_at"`
	Label           string    `json:"label"`
	Total           int       `json:"total"`
	Unauthorized401 int       `json:"status_401"`
	RateLimited429  int       `json:"status_429"`
}

// Snapshot is the frontend-facing stats payload.
type Snapshot struct {
	From             time.Time `json:"from"`
	To               time.Time `json:"to"`
	Range            string    `json:"range"`
	Bucket           string    `json:"bucket"`
	BucketSeconds    int64     `json:"bucket_seconds"`
	Totals           Totals    `json:"totals"`
	Series           []Bucket  `json:"series"`
	AvailableRanges  []string  `json:"available_ranges"`
	AvailableBuckets []string  `json:"available_buckets"`
}

// SnapshotOptions controls range aggregation for chart queries.
type SnapshotOptions struct {
	From       time.Time
	To         time.Time
	Range      string
	BucketSize time.Duration
	Now        time.Time
}

// Manager stores lightweight auth auto-delete statistics aggregated by minute.
// Data is kept in memory for fast reads and optionally mirrored into SQLite for restart persistence.
type Manager struct {
	mu         sync.RWMutex
	buckets    map[time.Time]bucketCounts
	retention  time.Duration
	maxBuckets int

	db     *sql.DB
	dbPath string
}

// NewManager constructs a minute-bucketed stats collector.
func NewManager(retention time.Duration, maxBuckets int) *Manager {
	if retention <= 0 {
		retention = defaultRetention
	}
	if maxBuckets <= 0 {
		maxBuckets = defaultMaxBuckets
	}
	return &Manager{
		buckets:    make(map[time.Time]bucketCounts, 256),
		retention:  retention,
		maxBuckets: maxBuckets,
	}
}

var defaultManager = NewManager(defaultRetention, defaultMaxBuckets)

// DefaultManager exposes the shared stats collector used by the runtime.
func DefaultManager() *Manager {
	return defaultManager
}

// OnAuthRegistered is a no-op hook implementation.
func (m *Manager) OnAuthRegistered(context.Context, *coreauth.Auth) {}

// OnAuthUpdated is a no-op hook implementation.
func (m *Manager) OnAuthUpdated(context.Context, *coreauth.Auth) {}

// OnResult records auto-deletions triggered by 401/429 request failures.
func (m *Manager) OnResult(_ context.Context, result coreauth.Result) {
	if result.Success {
		return
	}
	statusCode := statusCodeFromResult(result)
	if statusCode != StatusUnauthorized && statusCode != StatusTooMany {
		return
	}
	m.Record(Event{
		Timestamp:  time.Now().UTC(),
		StatusCode: statusCode,
		Provider:   strings.TrimSpace(result.Provider),
		AuthID:     strings.TrimSpace(result.AuthID),
	})
}

// ConfigureSQLite enables SQLite persistence for the aggregated minute buckets.
// Existing in-memory buckets are merged into the database and retained.
func (m *Manager) ConfigureSQLite(path string) error {
	if m == nil {
		return nil
	}

	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil
	}
	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return fmt.Errorf("auth delete stats: resolve sqlite path: %w", err)
	}
	if err = os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return fmt.Errorf("auth delete stats: create sqlite directory: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.db != nil && m.dbPath == absPath {
		return nil
	}

	db, err := sql.Open("sqlite", absPath)
	if err != nil {
		return fmt.Errorf("auth delete stats: open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err = initializeSQLite(db); err != nil {
		_ = db.Close()
		return err
	}

	cutoff := minuteBucketStart(time.Now().UTC().Add(-m.retention))
	if err = pruneExpiredBucketsDB(db, cutoff); err != nil {
		_ = db.Close()
		return err
	}

	diskBuckets, err := loadBucketsFromDB(db)
	if err != nil {
		_ = db.Close()
		return err
	}

	for minuteStart, counts := range m.buckets {
		merged := diskBuckets[minuteStart]
		merged.Unauthorized401 += counts.Unauthorized401
		merged.RateLimited429 += counts.RateLimited429
		diskBuckets[minuteStart] = merged
	}

	for minuteStart, counts := range m.buckets {
		if err = upsertMinuteBucket(db, minuteStart, counts); err != nil {
			_ = db.Close()
			return err
		}
	}

	if m.db != nil {
		_ = m.db.Close()
	}
	m.db = db
	m.dbPath = absPath
	m.buckets = diskBuckets
	m.pruneLocked(time.Now().UTC())
	return nil
}

// Close releases the SQLite handle if persistence is enabled.
func (m *Manager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil
	m.dbPath = ""
	return err
}

// Record appends one auto-delete event into the minute bucket store.
func (m *Manager) Record(event Event) {
	if m == nil {
		return
	}
	if event.StatusCode != StatusUnauthorized && event.StatusCode != StatusTooMany {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	} else {
		event.Timestamp = event.Timestamp.UTC()
	}

	minuteStart := minuteBucketStart(event.Timestamp)

	m.mu.Lock()
	counts := m.buckets[minuteStart]
	switch event.StatusCode {
	case StatusUnauthorized:
		counts.Unauthorized401++
	case StatusTooMany:
		counts.RateLimited429++
	}
	m.buckets[minuteStart] = counts
	db := m.db
	m.pruneLocked(event.Timestamp)
	m.mu.Unlock()

	if db != nil {
		if err := upsertMinuteBucket(db, minuteStart, countsForStatus(event.StatusCode)); err != nil {
			log.WithError(err).Warn("auth delete stats: failed to persist bucket update")
			return
		}
		if err := pruneExpiredBucketsDB(db, minuteBucketStart(event.Timestamp.Add(-m.retention))); err != nil {
			log.WithError(err).Warn("auth delete stats: failed to prune old sqlite buckets")
		}
	}
}

// Reset clears all collected stats. Intended for tests.
func (m *Manager) Reset() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.buckets = make(map[time.Time]bucketCounts)
	db := m.db
	m.mu.Unlock()

	if db != nil {
		if _, err := db.Exec(`DELETE FROM auth_delete_stats_minute`); err != nil {
			log.WithError(err).Warn("auth delete stats: failed to reset sqlite buckets")
		}
	}
}

// Snapshot aggregates chart data for the requested time window.
func (m *Manager) Snapshot(opts SnapshotOptions) Snapshot {
	if m == nil {
		return emptySnapshot(opts)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	from, to, rangeLabel := normalizeBounds(opts, now)
	bucketSize := normalizeBucketSize(opts.BucketSize, to.Sub(from))

	m.mu.Lock()
	m.pruneLocked(now)
	bucketSnapshot := make(map[time.Time]bucketCounts, len(m.buckets))
	for minuteStart, counts := range m.buckets {
		bucketSnapshot[minuteStart] = counts
	}
	m.mu.Unlock()

	start := floorTime(from, bucketSize)
	if start.After(from) {
		start = start.Add(-bucketSize)
	}
	count := bucketCount(start, to, bucketSize)
	series := make([]Bucket, count)
	for idx := 0; idx < count; idx++ {
		bucketStart := start.Add(time.Duration(idx) * bucketSize)
		bucketEnd := bucketStart.Add(bucketSize)
		if bucketEnd.After(to) {
			bucketEnd = to
		}
		series[idx] = Bucket{
			StartAt: bucketStart,
			EndAt:   bucketEnd,
			Label:   formatBucketLabel(bucketStart, bucketSize),
		}
	}

	minuteFrom := minuteBucketStart(from)
	minuteTo := minuteBucketStart(to)

	var totals Totals
	for minuteStart, counts := range bucketSnapshot {
		if minuteStart.Before(minuteFrom) || minuteStart.After(minuteTo) {
			continue
		}

		index := int(minuteStart.Sub(start) / bucketSize)
		if index < 0 || index >= len(series) {
			continue
		}

		bucket := &series[index]
		bucket.Unauthorized401 += counts.Unauthorized401
		bucket.RateLimited429 += counts.RateLimited429
		bucket.Total += counts.Unauthorized401 + counts.RateLimited429

		totals.Unauthorized401 += counts.Unauthorized401
		totals.RateLimited429 += counts.RateLimited429
		totals.All += counts.Unauthorized401 + counts.RateLimited429
	}

	return Snapshot{
		From:             from,
		To:               to,
		Range:            rangeLabel,
		Bucket:           bucketSize.String(),
		BucketSeconds:    int64(bucketSize / time.Second),
		Totals:           totals,
		Series:           series,
		AvailableRanges:  []string{"1h", "6h", "24h", "7d", "30d"},
		AvailableBuckets: []string{"1m", "5m", "15m", "1h", "6h", "12h", "24h"},
	}
}

func emptySnapshot(opts SnapshotOptions) Snapshot {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	from, to, rangeLabel := normalizeBounds(opts, now)
	bucketSize := normalizeBucketSize(opts.BucketSize, to.Sub(from))
	return Snapshot{
		From:             from,
		To:               to,
		Range:            rangeLabel,
		Bucket:           bucketSize.String(),
		BucketSeconds:    int64(bucketSize / time.Second),
		Totals:           Totals{},
		Series:           []Bucket{},
		AvailableRanges:  []string{"1h", "6h", "24h", "7d", "30d"},
		AvailableBuckets: []string{"1m", "5m", "15m", "1h", "6h", "12h", "24h"},
	}
}

func normalizeBounds(opts SnapshotOptions, now time.Time) (time.Time, time.Time, string) {
	to := opts.To
	if to.IsZero() {
		to = now
	} else {
		to = to.UTC()
	}
	from := opts.From
	rangeLabel := strings.TrimSpace(opts.Range)
	if from.IsZero() {
		rangeDuration, label := normalizeRange(rangeLabel)
		rangeLabel = label
		from = to.Add(-rangeDuration)
	}
	if from.After(to) {
		from, to = to, from
	}
	if rangeLabel == "" {
		rangeLabel = formatRangeLabel(to.Sub(from))
	}
	return from.UTC(), to.UTC(), rangeLabel
}

func normalizeRange(value string) (time.Duration, string) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return 24 * time.Hour, "24h"
	}
	duration, err := ParseFlexibleDuration(trimmed)
	if err != nil || duration <= 0 {
		return 24 * time.Hour, "24h"
	}
	return duration, formatRangeLabel(duration)
}

func normalizeBucketSize(bucketSize time.Duration, window time.Duration) time.Duration {
	if bucketSize > 0 {
		if bucketSize < time.Minute {
			return time.Minute
		}
		return bucketSize
	}
	switch {
	case window <= time.Hour:
		return time.Minute
	case window <= 6*time.Hour:
		return 5 * time.Minute
	case window <= 24*time.Hour:
		return 15 * time.Minute
	case window <= 7*24*time.Hour:
		return time.Hour
	case window <= 14*24*time.Hour:
		return 6 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func bucketCount(start, to time.Time, bucketSize time.Duration) int {
	if !start.Before(to) {
		return 1
	}
	count := int((to.Sub(start) + bucketSize - time.Nanosecond) / bucketSize)
	if count < 1 {
		return 1
	}
	if count > 720 {
		return 720
	}
	return count
}

func floorTime(value time.Time, bucket time.Duration) time.Time {
	if bucket <= 0 {
		return value
	}
	return value.Truncate(bucket)
}

func minuteBucketStart(value time.Time) time.Time {
	return value.UTC().Truncate(time.Minute)
}

func formatBucketLabel(start time.Time, bucketSize time.Duration) string {
	start = start.In(chinaLocation)
	switch {
	case bucketSize < time.Hour:
		return start.Format("15:04")
	case bucketSize < 24*time.Hour:
		return start.Format("01-02 15:04")
	default:
		return start.Format("01-02")
	}
}

func formatRangeLabel(duration time.Duration) string {
	switch duration {
	case time.Hour:
		return "1h"
	case 6 * time.Hour:
		return "6h"
	case 24 * time.Hour:
		return "24h"
	case 7 * 24 * time.Hour:
		return "7d"
	case 30 * 24 * time.Hour:
		return "30d"
	default:
		return duration.String()
	}
}

func statusCodeFromResult(result coreauth.Result) int {
	if result.StatusCode > 0 {
		return result.StatusCode
	}
	if result.Error != nil && result.Error.HTTPStatus > 0 {
		return result.Error.HTTPStatus
	}

	code := strings.TrimSpace(strings.ToLower(result.Code))
	if code == "" && result.Error != nil {
		code = strings.TrimSpace(strings.ToLower(result.Error.Code))
	}
	switch code {
	case "unauthorized":
		return StatusUnauthorized
	case "rate_limited", "too_many_requests":
		return StatusTooMany
	}
	return 0
}

func countsForStatus(statusCode int) bucketCounts {
	switch statusCode {
	case StatusUnauthorized:
		return bucketCounts{Unauthorized401: 1}
	case StatusTooMany:
		return bucketCounts{RateLimited429: 1}
	default:
		return bucketCounts{}
	}
}

func (m *Manager) pruneLocked(now time.Time) {
	cutoff := minuteBucketStart(now.Add(-m.retention))
	if len(m.buckets) > 0 {
		for minuteStart := range m.buckets {
			if minuteStart.Before(cutoff) {
				delete(m.buckets, minuteStart)
			}
		}
	}
	if len(m.buckets) > m.maxBuckets {
		keys := make([]time.Time, 0, len(m.buckets))
		for minuteStart := range m.buckets {
			keys = append(keys, minuteStart)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].Before(keys[j]) })
		removeCount := len(keys) - m.maxBuckets
		for idx := 0; idx < removeCount; idx++ {
			delete(m.buckets, keys[idx])
		}
	}
}

func initializeSQLite(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("auth delete stats: sqlite database is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultSQLiteTimeout)
	defer cancel()

	statements := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA synchronous = NORMAL;`,
		fmt.Sprintf(`PRAGMA busy_timeout = %d;`, sqliteBusyTimeout.Milliseconds()),
		`CREATE TABLE IF NOT EXISTS auth_delete_stats_minute (
			minute_start TEXT PRIMARY KEY,
			count_401 INTEGER NOT NULL DEFAULT 0,
			count_429 INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		);`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("auth delete stats: initialize sqlite: %w", err)
		}
	}
	return nil
}

func loadBucketsFromDB(db *sql.DB) (map[time.Time]bucketCounts, error) {
	if db == nil {
		return make(map[time.Time]bucketCounts), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultSQLiteTimeout)
	defer cancel()

	rows, err := db.QueryContext(
		ctx,
		`SELECT minute_start, count_401, count_429 FROM auth_delete_stats_minute ORDER BY minute_start`,
	)
	if err != nil {
		return nil, fmt.Errorf("auth delete stats: query sqlite buckets: %w", err)
	}
	defer rows.Close()

	buckets := make(map[time.Time]bucketCounts, 256)
	for rows.Next() {
		var row minuteBucketRow
		if err = rows.Scan(&row.MinuteStart, &row.Unauthorized401, &row.RateLimited429); err != nil {
			return nil, fmt.Errorf("auth delete stats: scan sqlite bucket: %w", err)
		}
		minuteStart, errParse := time.Parse(time.RFC3339Nano, row.MinuteStart)
		if errParse != nil {
			return nil, fmt.Errorf("auth delete stats: parse sqlite bucket time: %w", errParse)
		}
		buckets[minuteStart.UTC()] = bucketCounts{
			Unauthorized401: row.Unauthorized401,
			RateLimited429:  row.RateLimited429,
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("auth delete stats: iterate sqlite buckets: %w", err)
	}
	return buckets, nil
}

func upsertMinuteBucket(db *sql.DB, minuteStart time.Time, counts bucketCounts) error {
	if db == nil {
		return nil
	}
	if counts.Unauthorized401 == 0 && counts.RateLimited429 == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultSQLiteTimeout)
	defer cancel()

	_, err := db.ExecContext(
		ctx,
		`INSERT INTO auth_delete_stats_minute (minute_start, count_401, count_429, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(minute_start) DO UPDATE SET
			count_401 = auth_delete_stats_minute.count_401 + excluded.count_401,
			count_429 = auth_delete_stats_minute.count_429 + excluded.count_429,
			updated_at = excluded.updated_at`,
		minuteStart.UTC().Format(time.RFC3339Nano),
		counts.Unauthorized401,
		counts.RateLimited429,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("auth delete stats: upsert sqlite bucket: %w", err)
	}
	return nil
}

func pruneExpiredBucketsDB(db *sql.DB, cutoff time.Time) error {
	if db == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultSQLiteTimeout)
	defer cancel()

	_, err := db.ExecContext(
		ctx,
		`DELETE FROM auth_delete_stats_minute WHERE minute_start < ?`,
		cutoff.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("auth delete stats: prune sqlite buckets: %w", err)
	}
	return nil
}

// ParseFlexibleDuration accepts stdlib durations plus a day suffix like 7d.
func ParseFlexibleDuration(input string) (time.Duration, error) {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(trimmed, "d") {
		value := strings.TrimSuffix(trimmed, "d")
		days, err := time.ParseDuration(value + "h")
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	return time.ParseDuration(trimmed)
}
