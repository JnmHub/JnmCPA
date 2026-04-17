package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/misc"
	sdkAuth "github.com/router-for-me/CLIProxyAPI/v6/sdk/auth"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

const (
	defaultSQLiteTimeout      = 10 * time.Second
	defaultSQLitePollInterval = 5 * time.Second
	sqliteBusyTimeout         = 5 * time.Second
)

// SQLiteStoreConfig configures a SQLite-backed auth store.
type SQLiteStoreConfig struct {
	Path         string
	Timeout      time.Duration
	PollInterval time.Duration
}

type sqliteAuthRecord struct {
	ID        string
	FileName  string
	Provider  string
	Payload   []byte
	CreatedAt string
	UpdatedAt string
}

// SQLiteStore persists auth state directly in a local SQLite database without mirroring auth JSON files locally.
type SQLiteStore struct {
	db           *sql.DB
	path         string
	timeout      time.Duration
	pollInterval time.Duration

	baseDir string
	dirMu   sync.RWMutex
	writeMu sync.Mutex
}

// NewSQLiteStore creates a SQLite-backed auth store.
func NewSQLiteStore(ctx context.Context, cfg SQLiteStoreConfig) (*SQLiteStore, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return nil, fmt.Errorf("sqlite store: path is required")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultSQLiteTimeout
	}
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultSQLitePollInterval
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if path != ":memory:" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("sqlite store: resolve path: %w", err)
		}
		if err = os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
			return nil, fmt.Errorf("sqlite store: create parent directory: %w", err)
		}
		path = absPath
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: open database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	store := &SQLiteStore{
		db:           db,
		path:         path,
		timeout:      timeout,
		pollInterval: pollInterval,
	}

	if err = store.initialize(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

// Close releases the SQLite database handle.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// SetBaseDir records the logical auth base directory so path-based IDs can be normalized.
func (s *SQLiteStore) SetBaseDir(dir string) {
	if s == nil {
		return
	}
	s.dirMu.Lock()
	s.baseDir = strings.TrimSpace(dir)
	s.dirMu.Unlock()
}

// UsesAuthFileMirror reports that SQLite-backed auth records are not mirrored to local files.
func (s *SQLiteStore) UsesAuthFileMirror() bool {
	return false
}

// Save persists the provided auth record in SQLite.
func (s *SQLiteStore) Save(ctx context.Context, auth *cliproxyauth.Auth) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("sqlite store: not initialized")
	}

	normalized, err := s.normalizeAuthForStorage(auth)
	if err != nil {
		return "", err
	}
	existing, err := s.loadRecordByID(ctx, normalized.ID)
	if err != nil {
		return "", err
	}
	if normalized.CreatedAt.IsZero() {
		if existingCreatedAt, ok := sqliteRecordCreatedAt(existing); ok {
			normalized.CreatedAt = existingCreatedAt
		} else {
			normalized.CreatedAt = time.Now().UTC()
		}
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = time.Now().UTC()
	}

	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("sqlite store: marshal auth payload: %w", err)
	}

	writeCtx, cancel := s.timeoutContext(ctx)
	defer cancel()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	err = s.execWriteWithRetry(
		writeCtx,
		`INSERT INTO auth_store (id, file_name, provider, payload, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			file_name = excluded.file_name,
			provider = excluded.provider,
			payload = excluded.payload,
			updated_at = excluded.updated_at`,
		normalized.ID,
		strings.TrimSpace(normalized.FileName),
		strings.TrimSpace(normalized.Provider),
		payload,
		normalized.CreatedAt.UTC().Format(time.RFC3339Nano),
		normalized.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return "", fmt.Errorf("sqlite store: save auth %s: %w", normalized.ID, err)
	}

	*auth = *normalized.Clone()
	return s.logicalLocation(normalized.ID), nil
}

// List loads all auth records from SQLite.
func (s *SQLiteStore) List(ctx context.Context) ([]*cliproxyauth.Auth, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store: not initialized")
	}

	readCtx, cancel := s.timeoutContext(ctx)
	defer cancel()

	rows, err := s.db.QueryContext(readCtx, `SELECT id, COALESCE(file_name, ''), COALESCE(provider, ''), payload, created_at, updated_at FROM auth_store ORDER BY COALESCE(file_name, ''), id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: list auths: %w", err)
	}
	defer rows.Close()

	entries := make([]*cliproxyauth.Auth, 0)
	for rows.Next() {
		var record sqliteAuthRecord
		if err = rows.Scan(&record.ID, &record.FileName, &record.Provider, &record.Payload, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sqlite store: scan auth row: %w", err)
		}
		auth, errDecode := s.decodeRecord(&record)
		if errDecode != nil {
			return nil, errDecode
		}
		if auth != nil {
			entries = append(entries, auth)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite store: iterate auths: %w", err)
	}
	return entries, nil
}

// Delete removes one auth record from SQLite.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store: not initialized")
	}

	resolved := s.resolveDeleteID(id)
	if resolved == "" {
		return fmt.Errorf("sqlite store: id is empty")
	}

	deleteCtx, cancel := s.timeoutContext(ctx)
	defer cancel()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	err := s.execWriteWithRetry(
		deleteCtx,
		`DELETE FROM auth_store WHERE id = ? OR file_name = ? OR file_name = ?`,
		resolved,
		resolved,
		filepath.Base(resolved),
	)
	if err != nil {
		return fmt.Errorf("sqlite store: delete auth %s: %w", resolved, err)
	}
	return nil
}

// SubscribeAuthChanges watches for external SQLite auth mutations.
// It uses PRAGMA data_version to avoid unnecessary full-table scans when the database file is unchanged.
func (s *SQLiteStore) SubscribeAuthChanges(ctx context.Context, sink func(sdkAuth.AuthChange)) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store: not initialized")
	}
	if sink == nil {
		return fmt.Errorf("sqlite store: auth change sink is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	snapshot, err := s.snapshotAuthMap(ctx)
	if err != nil {
		return err
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		log.WithError(err).Warn("sqlite store: failed to open dedicated connection for data_version, falling back to periodic snapshot polling")
		return s.pollSnapshotChanges(ctx, snapshot, sink)
	}
	defer conn.Close()

	version, err := s.dataVersion(ctx, conn)
	if err != nil {
		log.WithError(err).Warn("sqlite store: data_version unavailable, falling back to periodic snapshot polling")
		return s.pollSnapshotChanges(ctx, snapshot, sink)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			currentVersion, errVersion := s.dataVersion(ctx, conn)
			if errVersion != nil {
				log.WithError(errVersion).Warn("sqlite store: data_version poll failed")
				continue
			}
			if currentVersion == version {
				continue
			}

			current, errSnapshot := s.snapshotAuthMap(ctx)
			if errSnapshot != nil {
				log.WithError(errSnapshot).Warn("sqlite store: snapshot poll failed")
				continue
			}

			emitAuthDiff(snapshot, current, sink, sqliteAuthEqual)
			snapshot = current
			version = currentVersion
		}
	}
}

func (s *SQLiteStore) initialize(ctx context.Context) error {
	writeCtx, cancel := s.timeoutContext(ctx)
	defer cancel()

	statements := []string{
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA synchronous = NORMAL;`,
		fmt.Sprintf(`PRAGMA busy_timeout = %d;`, sqliteBusyTimeout.Milliseconds()),
		`CREATE TABLE IF NOT EXISTS auth_store (
			id TEXT PRIMARY KEY,
			file_name TEXT,
			provider TEXT,
			payload BLOB NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_auth_store_file_name ON auth_store(file_name);`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(writeCtx, statement); err != nil {
			return fmt.Errorf("sqlite store: initialize database: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) loadRecordByID(ctx context.Context, id string) (*sqliteAuthRecord, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil
	}

	readCtx, cancel := s.timeoutContext(ctx)
	defer cancel()

	var record sqliteAuthRecord
	err := s.db.QueryRowContext(
		readCtx,
		`SELECT id, COALESCE(file_name, ''), COALESCE(provider, ''), payload, created_at, updated_at FROM auth_store WHERE id = ?`,
		id,
	).Scan(&record.ID, &record.FileName, &record.Provider, &record.Payload, &record.CreatedAt, &record.UpdatedAt)
	if err == nil {
		return &record, nil
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return nil, fmt.Errorf("sqlite store: load auth %s: %w", id, err)
}

func (s *SQLiteStore) snapshotAuthMap(ctx context.Context) (map[string]*cliproxyauth.Auth, error) {
	list, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	snapshot := make(map[string]*cliproxyauth.Auth, len(list))
	for _, auth := range list {
		if auth == nil || auth.ID == "" {
			continue
		}
		snapshot[auth.ID] = auth.Clone()
	}
	return snapshot, nil
}

func (s *SQLiteStore) pollSnapshotChanges(ctx context.Context, snapshot map[string]*cliproxyauth.Auth, sink func(sdkAuth.AuthChange)) error {
	if snapshot == nil {
		snapshot = make(map[string]*cliproxyauth.Auth)
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := s.snapshotAuthMap(ctx)
			if err != nil {
				log.WithError(err).Warn("sqlite store: snapshot poll failed")
				continue
			}
			emitAuthDiff(snapshot, current, sink, sqliteAuthEqual)
			snapshot = current
		}
	}
}

func emitAuthDiff(
	previous map[string]*cliproxyauth.Auth,
	current map[string]*cliproxyauth.Auth,
	sink func(sdkAuth.AuthChange),
	equal func(a, b *cliproxyauth.Auth) bool,
) {
	for id, auth := range current {
		oldAuth, ok := previous[id]
		if !ok {
			sink(sdkAuth.AuthChange{Action: sdkAuth.AuthChangeActionUpsert, ID: id, Auth: auth.Clone()})
			continue
		}
		if !equal(oldAuth, auth) {
			sink(sdkAuth.AuthChange{Action: sdkAuth.AuthChangeActionUpsert, ID: id, Auth: auth.Clone()})
		}
	}
	for id := range previous {
		if _, ok := current[id]; ok {
			continue
		}
		sink(sdkAuth.AuthChange{Action: sdkAuth.AuthChangeActionDelete, ID: id})
	}
}

func sqliteAuthEqual(a, b *cliproxyauth.Auth) bool {
	return reflect.DeepEqual(normalizeSQLiteAuthForCompare(a), normalizeSQLiteAuthForCompare(b))
}

func normalizeSQLiteAuthForCompare(auth *cliproxyauth.Auth) *cliproxyauth.Auth {
	if auth == nil {
		return nil
	}
	clone := auth.Clone()
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	clone.Runtime = nil
	return clone
}

func (s *SQLiteStore) decodeRecord(record *sqliteAuthRecord) (*cliproxyauth.Auth, error) {
	if record == nil || len(record.Payload) == 0 {
		return nil, nil
	}

	var auth cliproxyauth.Auth
	if err := json.Unmarshal(record.Payload, &auth); err != nil {
		return nil, fmt.Errorf("sqlite store: decode auth payload %s: %w", record.ID, err)
	}

	auth.ID = strings.TrimSpace(record.ID)
	if auth.FileName == "" {
		auth.FileName = strings.TrimSpace(record.FileName)
	}
	if auth.FileName == "" {
		auth.FileName = auth.ID
	}
	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	auth.Attributes["storage_backend"] = "sqlite"
	if auth.Label == "" {
		if email, ok := auth.Metadata["email"].(string); ok {
			auth.Label = strings.TrimSpace(email)
		}
	}

	if createdAt, ok := parseSQLiteTimestamp(record.CreatedAt); ok {
		auth.CreatedAt = createdAt
	}
	if updatedAt, ok := parseSQLiteTimestamp(record.UpdatedAt); ok {
		auth.UpdatedAt = updatedAt
	}

	return auth.Clone(), nil
}

func (s *SQLiteStore) normalizeAuthForStorage(auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, fmt.Errorf("sqlite store: auth is nil")
	}

	clone := auth.Clone()
	if clone.Attributes == nil {
		clone.Attributes = make(map[string]string)
	}
	clone.Attributes["storage_backend"] = "sqlite"

	if clone.Storage != nil {
		type metadataSetter interface {
			SetMetadata(map[string]any)
		}
		if setter, ok := clone.Storage.(metadataSetter); ok {
			setter.SetMetadata(clone.Metadata)
		}
		merged, err := misc.MergeMetadata(clone.Storage, clone.Metadata)
		if err != nil {
			return nil, fmt.Errorf("sqlite store: merge auth metadata: %w", err)
		}
		clone.Metadata = merged
		clone.Storage = nil
	}

	if clone.ID == "" {
		clone.ID = s.deriveAuthID(clone)
	}
	if clone.ID == "" {
		clone.ID = uuid.NewString()
	}
	if clone.FileName == "" {
		clone.FileName = s.defaultFileName(clone)
	}
	return clone, nil
}

func (s *SQLiteStore) deriveAuthID(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if path := s.resolveDeleteID(auth.Attributes["path"]); path != "" {
			return path
		}
	}
	if fileName := s.resolveDeleteID(auth.FileName); fileName != "" {
		return fileName
	}
	provider := strings.ToLower(strings.TrimSpace(auth.Provider))
	if provider == "" {
		provider = "auth"
	}
	email := ""
	if auth.Metadata != nil {
		if raw, ok := auth.Metadata["email"].(string); ok {
			email = strings.TrimSpace(raw)
		}
	}
	if email != "" {
		return fmt.Sprintf("%s-%s.json", provider, email)
	}
	return ""
}

func (s *SQLiteStore) defaultFileName(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.ID != "" {
		return filepath.Base(auth.ID)
	}
	return "auth.json"
}

func (s *SQLiteStore) resolveDeleteID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}

	clean := filepath.Clean(trimmed)
	baseDir := s.baseDirSnapshot()
	if baseDir != "" {
		if rel, err := filepath.Rel(baseDir, clean); err == nil && rel != "" && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	if filepath.IsAbs(clean) {
		return filepath.Base(clean)
	}
	return filepath.ToSlash(clean)
}

func (s *SQLiteStore) logicalLocation(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return fmt.Sprintf("sqlite://%s#%s", s.path, id)
}

func (s *SQLiteStore) dataVersion(ctx context.Context, conn *sql.Conn) (int64, error) {
	if conn == nil {
		return 0, fmt.Errorf("sqlite store: data_version connection is nil")
	}
	readCtx, cancel := s.timeoutContext(ctx)
	defer cancel()

	var version int64
	if err := conn.QueryRowContext(readCtx, `PRAGMA data_version;`).Scan(&version); err != nil {
		return 0, fmt.Errorf("sqlite store: query data_version: %w", err)
	}
	return version, nil
}

func sqliteRecordCreatedAt(record *sqliteAuthRecord) (time.Time, bool) {
	if record == nil {
		return time.Time{}, false
	}
	return parseSQLiteTimestamp(record.CreatedAt)
}

func parseSQLiteTimestamp(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func (s *SQLiteStore) timeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, s.timeout)
}

func (s *SQLiteStore) execWriteWithRetry(ctx context.Context, query string, args ...any) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store: not initialized")
	}

	delay := 25 * time.Millisecond
	for {
		_, err := s.db.ExecContext(ctx, query, args...)
		if err == nil {
			return nil
		}
		if !isSQLiteBusyError(err) {
			return err
		}
		if ctx != nil {
			if errCtx := ctx.Err(); errCtx != nil {
				return err
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
		if delay < 250*time.Millisecond {
			delay *= 2
		}
	}
}

func isSQLiteBusyError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "sqlite_busy") ||
		strings.Contains(text, "database is locked") ||
		strings.Contains(text, "database table is locked")
}

func (s *SQLiteStore) baseDirSnapshot() string {
	s.dirMu.RLock()
	defer s.dirMu.RUnlock()
	return s.baseDir
}
