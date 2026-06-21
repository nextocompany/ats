// Package hrauth owns the local HR password sign-in path that runs alongside
// Entra SSO. Credentials are bcrypt-hashed on the existing users table (a NULL
// password means SSO-only). Sessions are opaque, server-side tokens (sha256-hashed
// at rest, carried in an httpOnly cookie) — revocable and expiring, unlike a
// stateless JWT — mirroring the candidate_sessions design. A resolved session
// maps onto the same auth.Identity every dashboard handler already reads, so the
// two sign-in paths are interchangeable downstream of the auth middleware.
package hrauth

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors. Login failures are deliberately collapsed into one generic
// error so the response never reveals whether the email exists.
var (
	ErrInvalidCredentials = errors.New("hrauth: invalid email or password")
	ErrNotFound           = errors.New("hrauth: not found")
	ErrEmailExists        = errors.New("hrauth: email already exists")
	ErrWeakPassword       = errors.New("hrauth: password does not meet the policy")
	ErrInvalidRole        = errors.New("hrauth: unknown role")
)

// allowedRoles is the set of HR roles a local account may hold. It mirrors the
// roles rbac.Scope understands; an unknown role would fail closed to store scope,
// so we reject it at creation rather than silently narrowing visibility.
var allowedRoles = map[string]bool{
	"super_admin":        true,
	"regional_director":  true,
	"auditor":            true,
	"operation_director": true,
	"sgm":                true,
	"hr_manager":         true,
	"hr_staff":           true,
}

// User is a local HR account projection (never carries the password hash).
type User struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	Role        string     `json:"role"`
	StoreID     *int       `json:"store_id,omitempty"`
	Subregion   string     `json:"subregion,omitempty"`
	IsActive    bool       `json:"is_active"`
	HasPassword bool       `json:"has_password"`
	Source      string     `json:"source"` // 'local' (password) or 'sso' (Entra JIT)
	Phone       string     `json:"phone,omitempty"`
	IsDPO       bool       `json:"is_dpo"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// NewUserInput is the super_admin-supplied data to provision a local account.
type NewUserInput struct {
	Email     string
	FullName  string
	Role      string
	StoreID   *int
	Subregion string
	Password  string
}

// UpdateUserInput carries the editable fields. Pointers distinguish "not
// supplied" from "set to zero value" so a partial update never blanks a field.
type UpdateUserInput struct {
	FullName  *string
	Role      *string
	StoreID   *int
	Subregion *string
	IsActive  *bool
	Phone     *string // DPO contact phone (PDPA s.41)
	IsDPO     *bool   // designate/clear the account as a published Data Protection Officer
	Password  *string // when set, resets the password (must pass the policy)
}
