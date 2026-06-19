package rbac

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RoleReader is the read side the authorizer needs (satisfied by Repository).
type RoleReader interface {
	ListRoles(ctx context.Context) ([]Role, error)
}

type roleEntry struct {
	scopeKind string
	perms     map[string]struct{}
}

// Authorizer answers Can/Permissions/ScopeKind from an in-memory snapshot of the
// role→permission matrix. The snapshot is refreshed on a TTL ticker (so every api
// replica converges within ttl after an admin edit on any replica) and explicitly
// via Reload after a local write. Reads are lock-cheap and IO-free on the hot path.
//
// super_admin is a hard code bypass: Can(super_admin, *) is always true and its
// scope is always "all", independent of the DB — so the matrix can never lock the
// system out. Unknown/missing roles fail closed (no permissions, "store" scope).
type Authorizer struct {
	reader RoleReader
	ttl    time.Duration

	mu    sync.RWMutex
	roles map[string]roleEntry
}

// NewAuthorizer builds an authorizer with an empty snapshot. Call Reload (or Start)
// before serving so the cache is warm.
func NewAuthorizer(reader RoleReader, ttl time.Duration) *Authorizer {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Authorizer{reader: reader, ttl: ttl, roles: map[string]roleEntry{}}
}

// Reload replaces the snapshot from the reader. Safe to call concurrently.
func (a *Authorizer) Reload(ctx context.Context) error {
	roles, err := a.reader.ListRoles(ctx)
	if err != nil {
		return err
	}
	next := make(map[string]roleEntry, len(roles))
	for _, r := range roles {
		perms := make(map[string]struct{}, len(r.Permissions))
		for _, p := range r.Permissions {
			perms[p] = struct{}{}
		}
		next[r.Key] = roleEntry{scopeKind: r.ScopeKind, perms: perms}
	}
	a.mu.Lock()
	a.roles = next
	a.mu.Unlock()
	return nil
}

// Start runs a background refresh loop until ctx is cancelled. Errors are logged
// and the previous snapshot is retained (fail-static, never fail-open).
func (a *Authorizer) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(a.ttl)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := a.Reload(ctx); err != nil {
					log.Error().Err(err).Msg("rbac: authorizer refresh failed; keeping prior snapshot")
				}
			}
		}
	}()
}

// Can reports whether a role grants a permission. super_admin always can.
func (a *Authorizer) Can(role, perm string) bool {
	if role == RoleSuperAdmin {
		return true
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.roles[role]
	if !ok {
		return false // unknown role → fail closed
	}
	_, ok = e.perms[perm]
	return ok
}

// Permissions returns the role's granted permission keys (sorted-ish snapshot
// order). super_admin returns the full catalog. Unknown roles return empty.
func (a *Authorizer) Permissions(role string) []string {
	if role == RoleSuperAdmin {
		out := make([]string, len(AllPermissions))
		copy(out, AllPermissions)
		return out
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.roles[role]
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(e.perms))
	for p := range e.perms {
		out = append(out, p)
	}
	return out
}

// ScopeKind returns the role's data-visibility scope. super_admin is always
// KindAll; unknown/missing roles fall back to the most restrictive KindStore so a
// misconfigured role never widens visibility.
func (a *Authorizer) ScopeKind(role string) string {
	if role == RoleSuperAdmin {
		return KindAll
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	e, ok := a.roles[role]
	if !ok || e.scopeKind == "" {
		return KindStore
	}
	return e.scopeKind
}
