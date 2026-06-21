// Package breach owns the PDPA personal-data breach register (Phase 5.3). Thai
// PDPA s.37(4) requires the data controller to notify the PDPC within 72 hours of
// becoming aware of a breach, and to notify affected data subjects without delay
// where the breach is likely to result in a high risk to their rights. There is
// no public PDPC submission API, so this package records incidents, drives the
// 72h countdown (computed from discovered_at), and generates the notification
// content; the actual submission stays a manual operator action.
//
// It mirrors the requisitions CRUD package (model/repository/handler) and is
// gated entirely by the breach.manage permission. Breaches are company-wide, so
// no RBAC data-scope clause applies.
package breach

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
)

// NotificationWindow is the s.37(4) deadline for notifying the PDPC, measured
// from discovered_at ("becoming aware").
const NotificationWindow = 72 * time.Hour

// Severity values (incident impact).
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Status values (incident lifecycle).
const (
	StatusOpen      = "open"
	StatusContained = "contained"
	StatusResolved  = "resolved"
)

// Sentinel errors mapped to HTTP statuses by the handler.
var (
	ErrNotFound = errors.New("breach: not found")
	ErrBadState = errors.New("breach: not in a state that allows this action")
)

func validSeverity(s string) bool {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	default:
		return false
	}
}

func validStatus(s string) bool {
	switch s {
	case StatusOpen, StatusContained, StatusResolved:
		return true
	default:
		return false
	}
}

// Breach is one recorded personal-data breach. The Pdpc* derived fields are
// computed at read time from the stored timestamps (see deadline).
type Breach struct {
	ID                 uuid.UUID  `json:"id"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	Severity           string     `json:"severity"`
	Status             string     `json:"status"`
	AffectedSubjects   int        `json:"affected_subjects"`
	DataCategories     string     `json:"data_categories"`
	DiscoveredAt       time.Time  `json:"discovered_at"`
	OccurredAt         *time.Time `json:"occurred_at"`
	HighRisk           bool       `json:"high_risk"`
	PDPCNotifiedAt     *time.Time `json:"pdpc_notified_at"`
	SubjectsNotifiedAt *time.Time `json:"subjects_notified_at"`
	Remediation        string     `json:"remediation"`
	CreatedBy          *uuid.UUID `json:"created_by"`
	ResolvedBy         *uuid.UUID `json:"resolved_by"`
	ResolvedAt         *time.Time `json:"resolved_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`

	// Derived (not persisted): the 72h PDPC-notification countdown.
	Deadline Deadline `json:"deadline"`
}

// Deadline is the computed s.37(4) 72h status for a breach.
type Deadline struct {
	// DueAt is discovered_at + 72h: the moment the PDPC notification is due.
	DueAt time.Time `json:"due_at"`
	// Notified is true once pdpc_notified_at is set.
	Notified bool `json:"notified"`
	// HoursRemaining is the whole hours left until DueAt, floored so a breach that
	// is any amount past due reports a negative value (e.g. -1 = "1 hour overdue")
	// rather than a contradictory 0-while-overdue. Zero once Notified (discharged).
	HoursRemaining int `json:"hours_remaining"`
	// Overdue is true when the deadline has passed without a PDPC notification.
	Overdue bool `json:"overdue"`
}

// computeDeadline derives the 72h PDPC-notification status for a breach as of
// `now`. Kept pure (no clock dependency) so it is unit-testable.
func computeDeadline(discoveredAt time.Time, pdpcNotifiedAt *time.Time, now time.Time) Deadline {
	due := discoveredAt.Add(NotificationWindow)
	d := Deadline{DueAt: due, Notified: pdpcNotifiedAt != nil}
	if d.Notified {
		return d // obligation discharged: no countdown, never overdue
	}
	remaining := due.Sub(now)
	d.HoursRemaining = int(math.Floor(remaining.Hours()))
	d.Overdue = now.After(due)
	return d
}

// ListFilter narrows + paginates the breach list.
type ListFilter struct {
	Status   string
	Severity string
	Page     int
	Limit    int
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

// CreateInput is the payload for recording a new breach.
type CreateInput struct {
	Title            string
	Description      string
	Severity         string
	AffectedSubjects int
	DataCategories   string
	DiscoveredAt     time.Time
	OccurredAt       *time.Time
	HighRisk         bool
	Remediation      string
}

// UpdateInput sparsely edits a breach (nil = unchanged).
type UpdateInput struct {
	Title            *string
	Description      *string
	Severity         *string
	Status           *string
	AffectedSubjects *int
	DataCategories   *string
	OccurredAt       *time.Time
	HighRisk         *bool
	Remediation      *string
}
