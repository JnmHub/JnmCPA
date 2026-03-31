package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	defaultMongoCollection = "auth_store"
	defaultMongoTimeout    = 10 * time.Second
	defaultMongoPollInterval = 15 * time.Second
)

// MongoStoreConfig configures a MongoDB-backed auth store.
type MongoStoreConfig struct {
	URI        string
	Database   string
	Collection string
	Timeout    time.Duration
}

type mongoAuthDocument struct {
	ID        string    `bson:"_id"`
	FileName  string    `bson:"file_name,omitempty"`
	Provider  string    `bson:"provider,omitempty"`
	Payload   []byte    `bson:"payload"`
	CreatedAt time.Time `bson:"created_at,omitempty"`
	UpdatedAt time.Time `bson:"updated_at,omitempty"`
}

type mongoAuthChangeEvent struct {
	OperationType string             `bson:"operationType"`
	FullDocument  *mongoAuthDocument `bson:"fullDocument,omitempty"`
	DocumentKey   struct {
		ID string `bson:"_id"`
	} `bson:"documentKey"`
}

// MongoStore persists auth state directly in MongoDB without mirroring auth JSON files locally.
type MongoStore struct {
	client     *mongo.Client
	collection *mongo.Collection
	timeout    time.Duration

	baseDir string
	dirMu   sync.RWMutex
}

// NewMongoStore creates a MongoDB-backed auth store.
func NewMongoStore(ctx context.Context, cfg MongoStoreConfig) (*MongoStore, error) {
	uri := strings.TrimSpace(cfg.URI)
	if uri == "" {
		return nil, fmt.Errorf("mongo store: URI is required")
	}
	database := strings.TrimSpace(cfg.Database)
	if database == "" {
		return nil, fmt.Errorf("mongo store: database is required")
	}
	collection := strings.TrimSpace(cfg.Collection)
	if collection == "" {
		collection = defaultMongoCollection
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultMongoTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}
	connectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo store: connect failed: %w", err)
	}
	if err = client.Ping(connectCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo store: ping failed: %w", err)
	}

	return &MongoStore{
		client:     client,
		collection: client.Database(database).Collection(collection),
		timeout:    timeout,
	}, nil
}

// Close releases the MongoDB client.
func (s *MongoStore) Close(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.client.Disconnect(ctx)
}

// SetBaseDir records the logical auth base directory so path-based IDs can be normalized.
func (s *MongoStore) SetBaseDir(dir string) {
	if s == nil {
		return
	}
	s.dirMu.Lock()
	s.baseDir = strings.TrimSpace(dir)
	s.dirMu.Unlock()
}

// UsesAuthFileMirror reports that Mongo-backed auth records are not mirrored to local files.
func (s *MongoStore) UsesAuthFileMirror() bool {
	return false
}

// Save persists the provided auth record in MongoDB.
func (s *MongoStore) Save(ctx context.Context, auth *cliproxyauth.Auth) (string, error) {
	if s == nil || s.collection == nil {
		return "", fmt.Errorf("mongo store: not initialized")
	}
	normalized, err := s.normalizeAuthForStorage(auth)
	if err != nil {
		return "", err
	}
	existing, err := s.loadDocumentByID(ctx, normalized.ID)
	if err != nil {
		return "", err
	}
	if normalized.CreatedAt.IsZero() {
		if existing != nil && !existing.CreatedAt.IsZero() {
			normalized.CreatedAt = existing.CreatedAt
		} else {
			normalized.CreatedAt = time.Now().UTC()
		}
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = time.Now().UTC()
	}

	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("mongo store: marshal auth payload: %w", err)
	}
	doc := mongoAuthDocument{
		ID:        normalized.ID,
		FileName:  strings.TrimSpace(normalized.FileName),
		Provider:  strings.TrimSpace(normalized.Provider),
		Payload:   payload,
		CreatedAt: normalized.CreatedAt,
		UpdatedAt: normalized.UpdatedAt,
	}

	writeCtx, cancel := s.timeoutContext(ctx)
	defer cancel()
	_, err = s.collection.ReplaceOne(writeCtx, bson.M{"_id": normalized.ID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return "", fmt.Errorf("mongo store: save auth %s: %w", normalized.ID, err)
	}
	*auth = *normalized.Clone()
	return s.logicalLocation(normalized.ID), nil
}

// List loads all auth records from MongoDB.
func (s *MongoStore) List(ctx context.Context) ([]*cliproxyauth.Auth, error) {
	if s == nil || s.collection == nil {
		return nil, fmt.Errorf("mongo store: not initialized")
	}
	readCtx, cancel := s.timeoutContext(ctx)
	defer cancel()
	cursor, err := s.collection.Find(readCtx, bson.M{}, options.Find().SetSort(bson.D{{Key: "file_name", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("mongo store: list auths: %w", err)
	}
	defer cursor.Close(readCtx)

	entries := make([]*cliproxyauth.Auth, 0)
	for cursor.Next(readCtx) {
		var doc mongoAuthDocument
		if err = cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("mongo store: decode auth document: %w", err)
		}
		auth, errDecode := s.decodeDocument(&doc)
		if errDecode != nil {
			return nil, errDecode
		}
		if auth != nil {
			entries = append(entries, auth)
		}
	}
	if err = cursor.Err(); err != nil {
		return nil, fmt.Errorf("mongo store: iterate auths: %w", err)
	}
	return entries, nil
}

// Delete removes one auth record from MongoDB.
func (s *MongoStore) Delete(ctx context.Context, id string) error {
	if s == nil || s.collection == nil {
		return fmt.Errorf("mongo store: not initialized")
	}
	resolved := s.resolveDeleteID(id)
	if resolved == "" {
		return fmt.Errorf("mongo store: id is empty")
	}
	deleteCtx, cancel := s.timeoutContext(ctx)
	defer cancel()
	filter := bson.M{
		"$or": []bson.M{
			{"_id": resolved},
			{"file_name": resolved},
			{"file_name": filepath.Base(resolved)},
		},
	}
	_, err := s.collection.DeleteOne(deleteCtx, filter)
	if err != nil {
		return fmt.Errorf("mongo store: delete auth %s: %w", resolved, err)
	}
	return nil
}

// SubscribeAuthChanges streams auth mutations from MongoDB. It prefers change streams for
// near-real-time updates and automatically falls back to periodic polling when change streams
// are unavailable in the deployment topology.
func (s *MongoStore) SubscribeAuthChanges(ctx context.Context, sink func(sdkAuth.AuthChange)) error {
	if s == nil || s.collection == nil {
		return fmt.Errorf("mongo store: not initialized")
	}
	if sink == nil {
		return fmt.Errorf("mongo store: auth change sink is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	snapshot, err := s.snapshotAuthMap(ctx)
	if err != nil {
		return err
	}

	err = s.consumeChangeStream(ctx, snapshot, sink)
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}

	log.WithError(err).Warn("mongo store: change stream unavailable, falling back to polling")
	return s.pollAuthChanges(ctx, snapshot, sink)
}

func (s *MongoStore) loadDocumentByID(ctx context.Context, id string) (*mongoAuthDocument, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil
	}
	readCtx, cancel := s.timeoutContext(ctx)
	defer cancel()
	var doc mongoAuthDocument
	err := s.collection.FindOne(readCtx, bson.M{"_id": id}).Decode(&doc)
	if err == nil {
		return &doc, nil
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	return nil, fmt.Errorf("mongo store: load auth %s: %w", id, err)
}

func (s *MongoStore) snapshotAuthMap(ctx context.Context) (map[string]*cliproxyauth.Auth, error) {
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

func (s *MongoStore) consumeChangeStream(ctx context.Context, snapshot map[string]*cliproxyauth.Auth, sink func(sdkAuth.AuthChange)) error {
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	stream, err := s.collection.Watch(ctx, mongo.Pipeline{}, opts)
	if err != nil {
		return err
	}
	defer func() {
		_ = stream.Close(context.Background())
	}()

	for stream.Next(ctx) {
		var event mongoAuthChangeEvent
		if err = stream.Decode(&event); err != nil {
			return fmt.Errorf("mongo store: decode change stream event: %w", err)
		}
		change, errConvert := s.changeFromEvent(ctx, &event)
		if errConvert != nil {
			return errConvert
		}
		if change == nil {
			continue
		}
		switch change.Action {
		case sdkAuth.AuthChangeActionUpsert:
			if change.Auth != nil && change.Auth.ID != "" {
				snapshot[change.Auth.ID] = change.Auth.Clone()
			}
		case sdkAuth.AuthChangeActionDelete:
			delete(snapshot, change.ID)
		}
		sink(*change)
	}
	if err = stream.Err(); err != nil {
		return err
	}
	return ctx.Err()
}

func (s *MongoStore) changeFromEvent(ctx context.Context, event *mongoAuthChangeEvent) (*sdkAuth.AuthChange, error) {
	if event == nil {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(event.OperationType)) {
	case "insert", "replace", "update":
		doc := event.FullDocument
		if doc == nil && strings.TrimSpace(event.DocumentKey.ID) != "" {
			var err error
			doc, err = s.loadDocumentByID(ctx, event.DocumentKey.ID)
			if err != nil {
				return nil, err
			}
		}
		auth, err := s.decodeDocument(doc)
		if err != nil {
			return nil, err
		}
		if auth == nil || auth.ID == "" {
			return nil, nil
		}
		return &sdkAuth.AuthChange{
			Action: sdkAuth.AuthChangeActionUpsert,
			ID:     auth.ID,
			Auth:   auth.Clone(),
		}, nil
	case "delete":
		id := strings.TrimSpace(event.DocumentKey.ID)
		if id == "" {
			return nil, nil
		}
		return &sdkAuth.AuthChange{
			Action: sdkAuth.AuthChangeActionDelete,
			ID:     id,
		}, nil
	default:
		return nil, nil
	}
}

func (s *MongoStore) pollAuthChanges(ctx context.Context, snapshot map[string]*cliproxyauth.Auth, sink func(sdkAuth.AuthChange)) error {
	if snapshot == nil {
		snapshot = make(map[string]*cliproxyauth.Auth)
	}
	ticker := time.NewTicker(defaultMongoPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := s.snapshotAuthMap(ctx)
			if err != nil {
				log.WithError(err).Warn("mongo store: polling auth snapshot failed")
				continue
			}
			for id, auth := range current {
				previous, ok := snapshot[id]
				if !ok {
					sink(sdkAuth.AuthChange{Action: sdkAuth.AuthChangeActionUpsert, ID: id, Auth: auth.Clone()})
					continue
				}
				if !mongoAuthEqual(previous, auth) {
					sink(sdkAuth.AuthChange{Action: sdkAuth.AuthChangeActionUpsert, ID: id, Auth: auth.Clone()})
				}
			}
			for id := range snapshot {
				if _, ok := current[id]; !ok {
					sink(sdkAuth.AuthChange{Action: sdkAuth.AuthChangeActionDelete, ID: id})
				}
			}
			snapshot = current
		}
	}
}

func (s *MongoStore) decodeDocument(doc *mongoAuthDocument) (*cliproxyauth.Auth, error) {
	if doc == nil || len(doc.Payload) == 0 {
		return nil, nil
	}
	var auth cliproxyauth.Auth
	if err := json.Unmarshal(doc.Payload, &auth); err != nil {
		return nil, fmt.Errorf("mongo store: decode auth payload %s: %w", doc.ID, err)
	}
	auth.ID = strings.TrimSpace(doc.ID)
	if auth.FileName == "" {
		auth.FileName = strings.TrimSpace(doc.FileName)
	}
	if auth.FileName == "" {
		auth.FileName = auth.ID
	}
	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	auth.Attributes["storage_backend"] = "mongo"
	if auth.Label == "" {
		if email, ok := auth.Metadata["email"].(string); ok {
			auth.Label = strings.TrimSpace(email)
		}
	}
	return auth.Clone(), nil
}

func (s *MongoStore) normalizeAuthForStorage(auth *cliproxyauth.Auth) (*cliproxyauth.Auth, error) {
	if auth == nil {
		return nil, fmt.Errorf("mongo store: auth is nil")
	}
	clone := auth.Clone()
	if clone.Attributes == nil {
		clone.Attributes = make(map[string]string)
	}
	clone.Attributes["storage_backend"] = "mongo"

	if clone.Storage != nil {
		type metadataSetter interface {
			SetMetadata(map[string]any)
		}
		if setter, ok := clone.Storage.(metadataSetter); ok {
			setter.SetMetadata(clone.Metadata)
		}
		merged, err := misc.MergeMetadata(clone.Storage, clone.Metadata)
		if err != nil {
			return nil, fmt.Errorf("mongo store: merge auth metadata: %w", err)
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

func mongoAuthEqual(a, b *cliproxyauth.Auth) bool {
	return reflect.DeepEqual(normalizeMongoAuthForCompare(a), normalizeMongoAuthForCompare(b))
}

func normalizeMongoAuthForCompare(auth *cliproxyauth.Auth) *cliproxyauth.Auth {
	if auth == nil {
		return nil
	}
	clone := auth.Clone()
	clone.CreatedAt = time.Time{}
	clone.UpdatedAt = time.Time{}
	clone.Runtime = nil
	return clone
}

func (s *MongoStore) deriveAuthID(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.Attributes != nil {
		if p := s.resolveDeleteID(auth.Attributes["path"]); p != "" {
			return p
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

func (s *MongoStore) defaultFileName(auth *cliproxyauth.Auth) string {
	if auth == nil {
		return ""
	}
	if auth.ID != "" {
		return filepath.Base(auth.ID)
	}
	return "auth.json"
}

func (s *MongoStore) resolveDeleteID(id string) string {
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

func (s *MongoStore) logicalLocation(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || s.collection == nil {
		return ""
	}
	return fmt.Sprintf("mongo://%s/%s/%s", s.collection.Database().Name(), s.collection.Name(), id)
}

func (s *MongoStore) timeoutContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, s.timeout)
}

func (s *MongoStore) baseDirSnapshot() string {
	s.dirMu.RLock()
	defer s.dirMu.RUnlock()
	return s.baseDir
}
