package rbac

import (
	"errors"
	"time"
)

// Sentinel errors for the role/permission admin surface.
var (
	ErrRoleNotFound   = errors.New("rbac: role not found")
	ErrRoleExists     = errors.New("rbac: role already exists")
	ErrRoleBuiltin    = errors.New("rbac: built-in role cannot be modified that way")
	ErrRoleInUse      = errors.New("rbac: role is still assigned to users")
	ErrInvalidScope   = errors.New("rbac: scope_kind must be all, subregion, or store")
	ErrInvalidRoleKey = errors.New("rbac: role key must match [a-z0-9_]+")
	ErrUnknownPerm    = errors.New("rbac: unknown permission key")
)

// Role is a role and its resolved permission set + data scope.
type Role struct {
	Key         string    `json:"key"`
	LabelEn     string    `json:"label_en"`
	LabelTh     string    `json:"label_th"`
	ScopeKind   string    `json:"scope_kind"`
	IsBuiltin   bool      `json:"is_builtin"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

// Permission is a catalog entry (labels + UI grouping for the matrix).
type Permission struct {
	Key      string `json:"key"`
	LabelEn  string `json:"label_en"`
	LabelTh  string `json:"label_th"`
	Category string `json:"category"`
	Sort     int    `json:"sort"`
}

// RoleInput is the editable data for create/update. Pointers on update distinguish
// "not supplied" from "set to zero value"; Permissions nil means "leave as-is".
type RoleInput struct {
	Key         string // create only; ignored on update
	LabelEn     *string
	LabelTh     *string
	ScopeKind   *string
	Permissions *[]string // full replacement set when non-nil
}
