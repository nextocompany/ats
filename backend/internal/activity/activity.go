// Package activity records and reads the audit/history log (F16). Sprint 4a
// captures status/bulk/resume actions; the candidate timeline reads from it.
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxUserAgentLen caps the stored user agent so a crafted oversized header cannot
// bloat the audit table; the value is informational, not security-critical.
const maxUserAgentLen = 512

// Common action names.
const (
	ActionStatusChange = "status_change"
	ActionBulkAction   = "bulk_action"
	ActionViewResume   = "view_resume"
	ActionConsent      = "consent"
	ActionReengage     = "reengage"
	// ActionAssignment records a manual branch (re)assignment or move to the pool.
	ActionAssignment = "assignment"
	// ActionRetentionAnonymize records a PDPA retention-sweep anonymization (S7).
	ActionRetentionAnonymize = "retention_anonymize"
)

// PDPA-relevant audit actions (Phase 5.1 coverage). These mark data-subject
// rights exercises and breach-register mutations so the trail can demonstrate
// compliance (who, when, from where).
const (
	ActionDSARExport       = "dsar_export"
	ActionDSARErase        = "dsar_erase"
	ActionDSAREraseHeld    = "dsar_erase_held"
	ActionConsentWithdraw  = "consent_withdraw"
	ActionDSARComplete     = "dsar_complete" // DPO closed a held request as fulfilled
	ActionDSARReject       = "dsar_reject"   // DPO rejected a held request (with reason)
	ActionBreachRecord     = "breach_record"
	ActionBreachUpdate     = "breach_update"
	ActionBreachNotifyPDPC = "breach_notify_pdpc"
	ActionBreachNotifySubj = "breach_notify_subjects"
	ActionBreachResolve    = "breach_resolve"
	ActionBreachDelete     = "breach_delete"
)

// Entry is a single audit record. It intentionally omits the actor columns
// (user_id/ip_address/user_agent): the candidate-facing timeline that reads
// Entry must not expose who/where. A privileged DPO console (Phase 5.4) reads
// those columns through a separate query.
type Entry struct {
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   uuid.UUID       `json:"entity_id"`
	NewValue   json.RawMessage `json:"new_value,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Actor identifies who performed an audited action and from where. Any field may
// be zero: UserID is nil for system/background actions (the retention sweep) or
// when the caller has no internal id; IP/UserAgent are empty when unavailable.
type Actor struct {
	UserID    *uuid.UUID // HR Entra OID, local user id, or candidate account id
	IP        string     // spoof-resistant client IP (middleware.ClientIP)
	UserAgent string
}

// Writer records audit entries.
type Writer interface {
	Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error
}

// ActorWriter records audit entries with actor/IP/user-agent attribution. The
// PDPA-relevant handlers depend on this; *Log satisfies both interfaces.
type ActorWriter interface {
	Writer
	RecordWith(ctx context.Context, a Actor, action, entityType string, entityID uuid.UUID, newValue any) error
}

// Reader reads audit entries.
type Reader interface {
	List(ctx context.Context, entityType string, entityID uuid.UUID) ([]Entry, error)
}

type Log struct{ pool *pgxpool.Pool }

// New builds a Postgres-backed activity log (Writer + Reader).
func New(pool *pgxpool.Pool) *Log { return &Log{pool: pool} }

// Record writes an audit entry with no actor attribution (user_id/ip/user_agent
// left NULL). Retained for system/background callers; HTTP handlers should prefer
// RecordWith so the trail carries who and from where.
func (r *Log) Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error {
	return r.RecordWith(ctx, Actor{}, action, entityType, entityID, newValue)
}

// RecordWith writes an audit entry attributed to the given actor. A nil UserID
// and empty IP/UserAgent are stored as SQL NULL.
func (r *Log) RecordWith(ctx context.Context, a Actor, action, entityType string, entityID uuid.UUID, newValue any) error {
	var raw []byte
	if newValue != nil {
		b, err := json.Marshal(newValue)
		if err != nil {
			return fmt.Errorf("activity: marshal: %w", err)
		}
		raw = b
	}
	// Empty IP/UA must be NULL, not ""; an empty string is not a valid INET.
	// Defensively store NULL for any non-parseable IP so a bad caller value cannot
	// fail the INSERT (the ip_address column is INET), and cap the user agent.
	var ipArg, uaArg any
	if a.IP != "" && net.ParseIP(a.IP) != nil {
		ipArg = a.IP
	}
	if a.UserAgent != "" {
		ua := a.UserAgent
		if len(ua) > maxUserAgentLen {
			ua = ua[:maxUserAgentLen]
		}
		uaArg = ua
	}
	const q = `INSERT INTO activity_logs (user_id, action, entity_type, entity_id, new_value, ip_address, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	if _, err := r.pool.Exec(ctx, q, a.UserID, action, entityType, entityID, raw, ipArg, uaArg); err != nil {
		return fmt.Errorf("activity: record: %w", err)
	}
	return nil
}

func (r *Log) List(ctx context.Context, entityType string, entityID uuid.UUID) ([]Entry, error) {
	const q = `
		SELECT action, entity_type, entity_id, new_value, created_at
		FROM activity_logs WHERE entity_type = $1 AND entity_id = $2
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("activity: list: %w", err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Action, &e.EntityType, &e.EntityID, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("activity: scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
