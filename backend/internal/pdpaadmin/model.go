// Package pdpaadmin serves the PDPA/DPO admin console (Phase 5.4): the surface a
// Data Protection Officer uses to action held DSAR requests, look up a subject's
// consent history, and see a compliance overview (DSAR queue depth, breach
// status, retention policy, ROPA link, DPO contact). Every endpoint is gated by
// the pdpa.admin permission; the console is company-wide (no RBAC data scope).
//
// It reads the tables other PDPA packages own (dsar_requests, pdpa_consents,
// data_breaches) rather than re-implementing them, mirroring the thin admin-CRUD
// shape of internal/requisitions and internal/breach.
package pdpaadmin

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/pdpa"
)

// DSAR request statuses (mirror internal/dsar + the dsar_requests CHECK-free
// status column; app-enforced).
const (
	DSARStatusPending   = "pending"
	DSARStatusCompleted = "completed"
	DSARStatusRejected  = "rejected"
)

// Sentinel errors mapped to HTTP statuses by the handler.
var (
	ErrNotFound = errors.New("pdpaadmin: not found")
	ErrBadState = errors.New("pdpaadmin: request is not pending")
)

// DSARRequest is one queued data-subject request joined with the account's
// identity so the console can show who it concerns.
type DSARRequest struct {
	ID           uuid.UUID  `json:"id"`
	AccountID    uuid.UUID  `json:"account_id"`
	AccountName  string     `json:"account_name"`
	AccountEmail string     `json:"account_email"`
	RequestType  string     `json:"request_type"`
	Status       string     `json:"status"`
	Reason       string     `json:"reason"`
	RequestedAt  time.Time  `json:"requested_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	ResolvedBy   *uuid.UUID `json:"resolved_by"`
}

// ConsentRecord is one row of a subject's consent history (the unified ledger).
type ConsentRecord struct {
	ID           uuid.UUID  `json:"id"`
	CandidateID  *uuid.UUID `json:"candidate_id"`
	AccountID    *uuid.UUID `json:"account_id"`
	ConsentGiven bool       `json:"consent_given"`
	Version      string     `json:"version"`
	Source       string     `json:"source_channel"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Overview is the console summary card data.
type Overview struct {
	DSARPending     int             `json:"dsar_pending"`
	BreachesOpen    int             `json:"breaches_open"`
	BreachesOverdue int             `json:"breaches_overdue"` // open, past the 72h PDPC deadline, not yet notified
	ConsentVersion  string          `json:"current_consent_version"`
	RetentionDays   int             `json:"retention_days"`
	RetentionOn     bool            `json:"retention_sweep_enabled"`
	DPO             pdpa.DPOContact `json:"dpo"`
}

// DSARListFilter narrows + paginates the held-queue list.
type DSARListFilter struct {
	Status string
	Page   int
	Limit  int
}

const (
	defaultLimit = 20
	maxLimit     = 100
	maxReason    = 1000
)

func (f *DSARListFilter) normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > maxLimit {
		f.Limit = defaultLimit
	}
}
