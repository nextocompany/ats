// Package activity records and reads the audit/history log (F16). Sprint 4a
// captures status/bulk/resume actions; the candidate timeline reads from it.
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Common action names.
const (
	ActionStatusChange = "status_change"
	ActionBulkAction   = "bulk_action"
	ActionViewResume   = "view_resume"
	ActionConsent      = "consent"
)

// Entry is a single audit record.
type Entry struct {
	Action     string          `json:"action"`
	EntityType string          `json:"entity_type"`
	EntityID   uuid.UUID       `json:"entity_id"`
	NewValue   json.RawMessage `json:"new_value,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Writer records audit entries.
type Writer interface {
	Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error
}

// Reader reads audit entries.
type Reader interface {
	List(ctx context.Context, entityType string, entityID uuid.UUID) ([]Entry, error)
}

type Log struct{ pool *pgxpool.Pool }

// New builds a Postgres-backed activity log (Writer + Reader).
func New(pool *pgxpool.Pool) *Log { return &Log{pool: pool} }

func (r *Log) Record(ctx context.Context, action, entityType string, entityID uuid.UUID, newValue any) error {
	var raw []byte
	if newValue != nil {
		b, err := json.Marshal(newValue)
		if err != nil {
			return fmt.Errorf("activity: marshal: %w", err)
		}
		raw = b
	}
	const q = `INSERT INTO activity_logs (action, entity_type, entity_id, new_value) VALUES ($1,$2,$3,$4)`
	if _, err := r.pool.Exec(ctx, q, action, entityType, entityID, raw); err != nil {
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
