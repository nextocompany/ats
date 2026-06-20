// Package requisitions owns the HR-facing management of position openings
// (requisitions). It writes manual rows into the shared `vacancies` table
// (source='manual', ps_vacancy_id NULL) through an approval lifecycle, separate
// from the PeopleSoft sync path in internal/vacancies (which it never touches).
package requisitions

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Status values for a manually-managed requisition. The vacancies.status column
// is free-form; these are the app-enforced lifecycle states. A consumer (branch
// assigner, executive, portal, reports) only ever matches 'open', so a
// 'pending_approval' requisition is invisible until approved.
const (
	StatusPendingApproval = "pending_approval"
	StatusOpen            = "open"
	StatusClosed          = "closed"
	StatusCancelled       = "cancelled"
)

// SourceManual marks rows created through this package (vs 'peoplesoft').
const SourceManual = "manual"

// Sentinel errors mapped to HTTP statuses by the handler.
var (
	ErrNotFound = errors.New("requisitions: not found")
	ErrBadState = errors.New("requisitions: requisition not in the required state")
)

// Requisition is a managed vacancy row joined with its position/store labels.
type Requisition struct {
	ID            uuid.UUID  `json:"id"`
	PositionID    *uuid.UUID `json:"position_id"`
	PositionTitle string     `json:"position_title"`
	StoreID       *int       `json:"store_id"`
	StoreName     string     `json:"store_name"`
	Subregion     string     `json:"subregion"`
	Headcount     int        `json:"headcount"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	CreatedBy     *uuid.UUID `json:"created_by"`
	ApprovedBy    *uuid.UUID `json:"approved_by"`
	ApprovedAt    *time.Time `json:"approved_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ListFilter narrows + paginates the requisitions list.
type ListFilter struct {
	Status     string
	StoreID    *int
	PositionID *uuid.UUID
	Page       int
	Limit      int
}

const (
	defaultLimit = 20
	maxLimit     = 100
)

func (f *ListFilter) normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > maxLimit {
		f.Limit = defaultLimit
	}
}

// CreateInput is the payload for opening a new requisition.
type CreateInput struct {
	PositionID uuid.UUID
	StoreID    int
	Headcount  int
}

// UpdateInput sparsely edits a pending requisition (nil = unchanged).
type UpdateInput struct {
	PositionID *uuid.UUID
	StoreID    *int
	Headcount  *int
}

// Repository is the requisition data-access contract.
type Repository interface {
	List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Requisition, int, error)
	Create(ctx context.Context, in CreateInput, createdBy uuid.UUID) (Requisition, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Requisition, error)
	Approve(ctx context.Context, id uuid.UUID, approver uuid.UUID) (Requisition, error)
	Close(ctx context.Context, id uuid.UUID) (Requisition, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// ExistsInScope reports whether the row is visible to the caller's scope.
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
}
