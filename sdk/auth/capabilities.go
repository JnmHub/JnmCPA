package auth

import (
	"context"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// AuthFileMirrorMode allows stores to declare whether credentials are mirrored to local auth files.
// File-backed and mirrored stores should return true. Database-native stores can return false to
// disable auth-dir watcher scans and rely on direct runtime updates instead.
type AuthFileMirrorMode interface {
	UsesAuthFileMirror() bool
}

// UsesAuthFileMirror reports whether the provided store keeps auth records mirrored to local files.
// Stores that do not implement AuthFileMirrorMode default to true for backwards compatibility.
func UsesAuthFileMirror(store coreauth.Store) bool {
	if store == nil {
		return true
	}
	if mode, ok := store.(AuthFileMirrorMode); ok {
		return mode.UsesAuthFileMirror()
	}
	return true
}

// AuthChangeAction describes the type of auth record mutation reported by a store subscriber.
type AuthChangeAction string

const (
	AuthChangeActionUpsert AuthChangeAction = "upsert"
	AuthChangeActionDelete AuthChangeAction = "delete"
)

// AuthChange represents one store-originated auth mutation.
type AuthChange struct {
	Action AuthChangeAction
	ID     string
	Auth   *coreauth.Auth
}

// AuthChangeSubscriber streams auth mutations from a store so services can react to out-of-process updates.
type AuthChangeSubscriber interface {
	SubscribeAuthChanges(ctx context.Context, sink func(AuthChange)) error
}
