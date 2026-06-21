package hrauth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/auth"
)

// ssoProvisionThrottle is how long a successfully provisioned SSO oid is cached so
// repeated bearer-token requests (every API call carries the token) do not each
// trigger a DB upsert. A short window keeps last_login_at roughly fresh without a
// write per request.
const ssoProvisionThrottle = 15 * time.Minute

// Repository is the hrauth data-access contract (Postgres-backed in production,
// faked in tests).
type Repository interface {
	// FindCredentialsByEmail returns the active user and their bcrypt hash for a
	// normalised email. ErrNotFound when no active user exists OR has no password
	// — the caller still runs a dummy compare to keep timing uniform.
	FindCredentialsByEmail(ctx context.Context, email string) (User, string, error)
	TouchLastLogin(ctx context.Context, userID uuid.UUID) error

	CreateSession(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	// FindSessionUser returns the active user behind a live (unrevoked, unexpired)
	// session token hash, or ErrNotFound.
	FindSessionUser(ctx context.Context, tokenHash string) (User, error)
	RevokeSession(ctx context.Context, tokenHash string) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error

	// UpsertSSOUser JIT-provisions an Entra SSO identity into the users table on
	// sign-in (new accounts get no role: an admin grants access in-app).
	UpsertSSOUser(ctx context.Context, oid, email, fullName string) error
	ListUsers(ctx context.Context) ([]User, error)
	GetUser(ctx context.Context, id uuid.UUID) (User, error)
	CreateUser(ctx context.Context, email, fullName, role string, storeID *int, subregion, passwordHash string) (User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, in UpdateUserInput, passwordHash *string) (User, error)
}

// Service orchestrates HR password login, session lifecycle, and super_admin
// account provisioning.
type Service struct {
	repo       Repository
	sessionTTL time.Duration
	now        func() time.Time
	// roleValid optionally validates an assignable role against a dynamic source
	// (the rbac_roles table). When nil, or on validator error, the built-in
	// allowedRoles set is used (fail-safe: never widens beyond the known roles).
	roleValid func(ctx context.Context, role string) (bool, error)
	// ssoSeen throttles JIT provisioning: oid (string) -> last upsert time.Time.
	ssoSeen sync.Map
}

// NewService builds the hrauth service.
func NewService(repo Repository, sessionTTL time.Duration) *Service {
	return &Service{repo: repo, sessionTTL: sessionTTL, now: time.Now}
}

// SetRoleValidator installs a dynamic role-assignability check (e.g. backed by the
// rbac_roles table) so custom roles become assignable to local accounts.
func (s *Service) SetRoleValidator(f func(ctx context.Context, role string) (bool, error)) {
	s.roleValid = f
}

// roleAllowed reports whether a role may be assigned. Prefers the dynamic
// validator; falls back to the built-in allowlist when unset or on error.
func (s *Service) roleAllowed(ctx context.Context, role string) bool {
	if s.roleValid != nil {
		if ok, err := s.roleValid(ctx, role); err == nil {
			return ok
		}
	}
	return allowedRoles[role]
}

// normalizeEmail lower-cases and trims so login matches creation regardless of
// how the operator typed it.
func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// SessionTTL exposes the configured lifetime (the handler needs it to set the
// cookie expiry).
func (s *Service) SessionTTL() time.Duration { return s.sessionTTL }

// Login verifies credentials and, on success, mints a session. It returns the
// opaque session token (for the cookie) and its expiry. Any failure path returns
// ErrInvalidCredentials with no detail — and still performs a bcrypt comparison —
// so neither the message nor the timing reveals whether the email exists.
func (s *Service) Login(ctx context.Context, email, password string) (token string, expires time.Time, user User, err error) {
	email = normalizeEmail(email)

	u, hash, findErr := s.repo.FindCredentialsByEmail(ctx, email)
	if findErr != nil {
		if errors.Is(findErr, ErrNotFound) {
			_ = checkPassword(dummyHash(), password) // equalise timing
			return "", time.Time{}, User{}, ErrInvalidCredentials
		}
		return "", time.Time{}, User{}, findErr
	}
	if !checkPassword(hash, password) {
		return "", time.Time{}, User{}, ErrInvalidCredentials
	}

	tok, genErr := genToken()
	if genErr != nil {
		return "", time.Time{}, User{}, genErr
	}
	expires = s.now().Add(s.sessionTTL)
	if err := s.repo.CreateSession(ctx, u.ID, hashToken(tok), expires); err != nil {
		return "", time.Time{}, User{}, err
	}
	// Best-effort: a last-login bookkeeping failure must not fail the login.
	_ = s.repo.TouchLastLogin(ctx, u.ID)
	return tok, expires, u, nil
}

// Logout revokes the session behind a token (idempotent: an unknown token is a
// no-op so a stale cookie still clears cleanly).
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.RevokeSession(ctx, hashToken(token))
}

// ValidateSession resolves a session token to an auth.Identity for the auth
// middleware. The bool is false for any missing/expired/revoked/inactive case —
// the middleware turns that into a 401.
func (s *Service) ValidateSession(ctx context.Context, token string) (auth.Identity, bool) {
	if token == "" {
		return auth.Identity{}, false
	}
	u, err := s.repo.FindSessionUser(ctx, hashToken(token))
	if err != nil {
		return auth.Identity{}, false
	}
	return auth.Identity{
		ID:        u.ID.String(),
		Email:     u.Email,
		Role:      u.Role,
		StoreID:   u.StoreID,
		Subregion: u.Subregion,
	}, true
}

// ProvisionSSOUser JIT-provisions a verified Entra SSO identity into the users
// table (best-effort, throttled). It runs from the auth middleware after the token
// is verified, so any failure is returned but must NOT block the request: the user
// is already authenticated. A new row is created with no role (default-deny); an
// admin grants access in-app. Called per request, so it skips the DB when the same
// oid was provisioned within ssoProvisionThrottle.
func (s *Service) ProvisionSSOUser(ctx context.Context, id auth.Identity) error {
	oid := strings.TrimSpace(id.ID)
	if oid == "" {
		return nil // local sessions carry a user UUID, not an Entra oid: nothing to JIT.
	}
	now := s.now()
	if last, ok := s.ssoSeen.Load(oid); ok {
		if t, ok := last.(time.Time); ok && now.Sub(t) < ssoProvisionThrottle {
			return nil
		}
	}
	if err := s.repo.UpsertSSOUser(ctx, oid, normalizeEmail(id.Email), strings.TrimSpace(id.Name)); err != nil {
		return err
	}
	s.ssoSeen.Store(oid, now)
	return nil
}

// --- super_admin account provisioning -------------------------------------

// ListUsers returns all local HR accounts.
func (s *Service) ListUsers(ctx context.Context) ([]User, error) { return s.repo.ListUsers(ctx) }

// CreateUser provisions a new local account with a password. Validates the role
// and password policy before hashing.
func (s *Service) CreateUser(ctx context.Context, in NewUserInput) (User, error) {
	email := normalizeEmail(in.Email)
	if email == "" {
		return User{}, ErrInvalidCredentials
	}
	if !s.roleAllowed(ctx, in.Role) {
		return User{}, ErrInvalidRole
	}
	if err := validatePassword(in.Password); err != nil {
		return User{}, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return User{}, err
	}
	return s.repo.CreateUser(ctx, email, strings.TrimSpace(in.FullName), in.Role, in.StoreID, strings.TrimSpace(in.Subregion), hash)
}

// ErrSelfLockout is returned when an admin tries to disable or demote their own
// account — a footgun that could lock the last super_admin out of the console.
var ErrSelfLockout = errors.New("hrauth: cannot disable or demote your own account")

// UpdateUser edits an account. A supplied password is validated + hashed and all
// the user's existing sessions are revoked (a reset must invalidate live logins).
// Deactivating an account likewise revokes its sessions so access stops at once.
//
// callerID is the authenticated admin performing the change; an admin may not
// deactivate or demote (away from super_admin) their OWN account, to avoid
// locking themselves — potentially the last super_admin — out.
func (s *Service) UpdateUser(ctx context.Context, id uuid.UUID, callerID string, in UpdateUserInput) (User, error) {
	if in.Role != nil && !s.roleAllowed(ctx, *in.Role) {
		return User{}, ErrInvalidRole
	}
	if callerID == id.String() {
		if in.IsActive != nil && !*in.IsActive {
			return User{}, ErrSelfLockout
		}
		if in.Role != nil && *in.Role != "super_admin" {
			return User{}, ErrSelfLockout
		}
	}
	var passwordHash *string
	if in.Password != nil {
		if err := validatePassword(*in.Password); err != nil {
			return User{}, err
		}
		h, err := hashPassword(*in.Password)
		if err != nil {
			return User{}, err
		}
		passwordHash = &h
	}
	u, err := s.repo.UpdateUser(ctx, id, in, passwordHash)
	if err != nil {
		return User{}, err
	}
	if passwordHash != nil || (in.IsActive != nil && !*in.IsActive) {
		// Reset or deactivation: kill all live sessions immediately.
		_ = s.repo.RevokeAllUserSessions(ctx, id)
	}
	return u, nil
}
