package rbac

import "sync/atomic"

// defaultAuthorizer is the process-wide authorizer installed at startup. Using a
// package singleton lets handlers and scope.go call rbac.Can / rbac.ScopeKindFor
// without threading the authorizer through ~16 handler constructors and ~10
// scopeFrom call sites. It is nil until SetDefault is called, in which case the
// legacy compiled-in matrix is used (tests / un-migrated DB) — never fail-open.
var defaultAuthorizer atomic.Pointer[Authorizer]

// SetDefault installs the process-wide authorizer (call once from main after a
// successful Reload). Passing nil reverts to the legacy fallback.
func SetDefault(a *Authorizer) { defaultAuthorizer.Store(a) }

// Can reports whether a role grants a permission, via the installed authorizer or
// the legacy fallback. This is the single check every handler uses.
func Can(role, perm string) bool {
	if a := defaultAuthorizer.Load(); a != nil {
		return a.Can(role, perm)
	}
	return legacyCan(role, perm)
}

// ScopeKindFor returns a role's data-visibility scope (all/subregion/store).
func ScopeKindFor(role string) string {
	if a := defaultAuthorizer.Load(); a != nil {
		return a.ScopeKind(role)
	}
	return legacyScopeKind(role)
}

// Permissions returns a role's granted permission keys (for the /me payload).
func Permissions(role string) []string {
	if a := defaultAuthorizer.Load(); a != nil {
		return a.Permissions(role)
	}
	return legacyPermissions(role)
}
