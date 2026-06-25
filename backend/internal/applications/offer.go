package applications

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Offer management (Module-3 3.6). After the approval chain advances an application
// to `offer`, HR composes an offer package (salary/start date/terms), sends it, and
// the candidate accepts or declines via the career-portal membership. Accept flips
// the application to `hired` (+ PeopleSoft push); decline flips it to `rejected`.
// The offer's own lifecycle lives on the offers table (migration 000023); the
// application funnel reuses offer → hired/rejected.

// Offer lifecycle states (offers.status). 'negotiating' is the candidate-countered
// state: the offer is paused awaiting an HR revise & re-send. offers.status is a
// plain TEXT column (no CHECK), so this value needs no migration.
const (
	OfferDraft       = "draft"
	OfferSent        = "sent"
	OfferNegotiating = "negotiating"
	OfferAccepted    = "accepted"
	OfferDeclined    = "declined"
	OfferExpired     = "expired"
)

// Candidate decision verbs (respond endpoint).
const (
	OfferDecisionAccept    = "accept"
	OfferDecisionDecline   = "decline"
	OfferDecisionNegotiate = "negotiate"
)

// canManageOffer may compose/send offers — now resolved via dynamic RBAC
// (rbac.PermOfferWrite). Reads are open to anyone with RBAC visibility.
func canManageOffer(role string) bool { return rbac.Can(role, rbac.PermOfferWrite) }

func validOfferDecision(d string) bool {
	return d == OfferDecisionAccept || d == OfferDecisionDecline || d == OfferDecisionNegotiate
}

// Sentinel errors mapped to HTTP status by the handlers.
var (
	ErrOfferExists       = errors.New("applications: offer already exists for application")
	ErrOfferNotEditable  = errors.New("applications: offer is not editable")
	ErrOfferConflict     = errors.New("applications: offer not in a respondable state")
	ErrOfferNotFound     = errors.New("applications: offer not found for this account")
	ErrNegotiationClosed = errors.New("applications: negotiation rounds exhausted")
)

// Benefit is one structured benefit line on an offer (e.g. {label:"ประกันสังคม",
// value:"ตามกฎหมาย"}). Composed by HR, reviewed by the candidate alongside salary.
type Benefit struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Offer is the offer package + its lifecycle. Salary/dates are pointers because a
// draft may be composed incrementally. CreatedBy is server-stamped (json:"-").
type Offer struct {
	ID               uuid.UUID  `json:"id"`
	ApplicationID    uuid.UUID  `json:"application_id"`
	Status           string     `json:"status"`
	Salary           *float64   `json:"salary"`
	StartDate        *time.Time `json:"start_date"`
	Terms            string     `json:"terms,omitempty"`
	Benefits         []Benefit  `json:"benefits,omitempty"`
	CounterSalary    *float64   `json:"counter_salary,omitempty"`
	NegotiationNote  string     `json:"negotiation_note,omitempty"`
	NegotiationRound int        `json:"negotiation_round"`
	CreatedBy        *uuid.UUID `json:"-"`
	SentAt           *time.Time `json:"sent_at"`
	RespondedAt      *time.Time `json:"responded_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	DeclineReason    string     `json:"decline_reason,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// OfferInput is the HR compose/edit payload. Dates arrive as RFC3339; the frontend
// sends UTC-midnight ("yyyy-mm-ddT00:00:00Z") for the date-only start_date so the
// DATE column stores the intended calendar day regardless of viewer timezone.
type OfferInput struct {
	Salary    *float64   `json:"salary"`
	StartDate *time.Time `json:"start_date"`
	Terms     string     `json:"terms"`
	Benefits  []Benefit  `json:"benefits"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// OfferResponseInput is the candidate accept/decline/negotiate payload.
type OfferResponseInput struct {
	Decision      string   `json:"decision"`       // accept | decline | negotiate
	Reason        string   `json:"reason"`         // required when Decision == decline
	CounterSalary *float64 `json:"counter_salary"` // required when Decision == negotiate
	Note          string   `json:"note"`           // optional note on negotiate
}

// OfferView is a candidate-facing offer row (offer + minimal application context).
type OfferView struct {
	Offer
	PositionTitle string `json:"position_title,omitempty"`
	StoreID       *int   `json:"store_id"`
}

// ValidateOfferForSend reports whether an offer is complete enough to send.
func ValidateOfferForSend(o Offer) error {
	if o.Salary == nil || *o.Salary <= 0 {
		return errors.New("a positive salary is required to send an offer")
	}
	if o.StartDate == nil {
		return errors.New("a start date is required to send an offer")
	}
	return nil
}

// effectiveOfferStatus reports a sent offer as expired once its deadline passes,
// without mutating the row (the respond transaction enforces the real rule).
func effectiveOfferStatus(o Offer, now time.Time) string {
	if o.Status == OfferSent && o.ExpiresAt != nil && now.After(*o.ExpiresAt) {
		return OfferExpired
	}
	return o.Status
}
