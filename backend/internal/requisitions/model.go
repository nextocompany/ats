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

// Employment types, priorities, and open reasons for a requisition. These are
// app-enforced enums stored in free-form VARCHAR columns (no DB CHECK), validated
// by the handler. Empty employment_type / open_reason mean "unspecified"; priority
// always resolves to a value (defaults to PriorityNormal).
const (
	EmploymentFullTime = "full_time"
	EmploymentPartTime = "part_time"
	EmploymentContract = "contract"
	EmploymentSeasonal = "seasonal"

	PriorityNormal = "normal"
	PriorityUrgent = "urgent"

	ReasonNewHeadcount = "new_headcount"
	ReasonReplacement  = "replacement"
)

// maxJDTextLen caps each long-text JD field to keep payloads sane.
const maxJDTextLen = 5000

// ValidEmployment reports whether v is an allowed employment type.
func ValidEmployment(v string) bool {
	switch v {
	case EmploymentFullTime, EmploymentPartTime, EmploymentContract, EmploymentSeasonal:
		return true
	}
	return false
}

// ValidPriority reports whether v is an allowed priority.
func ValidPriority(v string) bool {
	return v == PriorityNormal || v == PriorityUrgent
}

// ValidReason reports whether v is an allowed open reason.
func ValidReason(v string) bool {
	return v == ReasonNewHeadcount || v == ReasonReplacement
}

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

	// Detailed JD + opening metadata (manual requisitions). Stored on the vacancy row.
	Responsibilities string `json:"responsibilities"`
	Qualifications   string `json:"qualifications"`
	Benefits         string `json:"benefits"`
	OtherDetails     string `json:"other_details"`
	EmploymentType   string `json:"employment_type"`
	SalaryMin        *int   `json:"salary_min"`
	SalaryMax        *int   `json:"salary_max"`
	Priority         string `json:"priority"`
	OpenReason       string `json:"open_reason"`
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

// CreateInput is the payload for opening a new requisition. The JD/metadata fields
// are optional; the handler defaults Priority to PriorityNormal when empty.
type CreateInput struct {
	PositionID uuid.UUID
	StoreID    int
	Headcount  int

	Responsibilities string
	Qualifications   string
	Benefits         string
	OtherDetails     string
	EmploymentType   string
	SalaryMin        *int
	SalaryMax        *int
	Priority         string
	OpenReason       string
}

// UpdateInput sparsely edits a pending requisition (nil = unchanged).
type UpdateInput struct {
	PositionID *uuid.UUID
	StoreID    *int
	Headcount  *int

	Responsibilities *string
	Qualifications   *string
	Benefits         *string
	OtherDetails     *string
	EmploymentType   *string
	SalaryMin        *int
	SalaryMax        *int
	Priority         *string
	OpenReason       *string
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
