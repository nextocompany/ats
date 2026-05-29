package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Export is a persisted report export (recurring or on-demand).
type Export struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Period    string    `json:"period"`
	CSVBlob   string    `json:"csv_blob"`
	JSONBlob  string    `json:"json_blob"`
	Delivered bool      `json:"delivered"`
	CreatedAt time.Time `json:"created_at"`
}

// RecordExport upserts an export record (idempotent on (kind, period) so asynq
// retries overwrite rather than duplicate) and returns it with id/created_at set.
func (r *Repo) RecordExport(ctx context.Context, e Export) (Export, error) {
	const q = `
		INSERT INTO report_exports (kind, period, csv_blob, json_blob, delivered)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (kind, period) DO UPDATE SET
			csv_blob = EXCLUDED.csv_blob,
			json_blob = EXCLUDED.json_blob,
			delivered = EXCLUDED.delivered
		RETURNING id, created_at`
	if err := r.pool.QueryRow(ctx, q, e.Kind, e.Period, e.CSVBlob, e.JSONBlob, e.Delivered).
		Scan(&e.ID, &e.CreatedAt); err != nil {
		return Export{}, fmt.Errorf("reports: record export: %w", err)
	}
	return e, nil
}

// MarkDelivered flips an export's delivered flag after successful delivery.
func (r *Repo) MarkDelivered(ctx context.Context, id uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `UPDATE report_exports SET delivered = TRUE WHERE id = $1`, id); err != nil {
		return fmt.Errorf("reports: mark delivered: %w", err)
	}
	return nil
}

// ListExports returns the most recent exports, newest first.
func (r *Repo) ListExports(ctx context.Context, limit int) ([]Export, error) {
	const q = `
		SELECT id, kind, period, COALESCE(csv_blob,''), COALESCE(json_blob,''), delivered, created_at
		FROM report_exports ORDER BY created_at DESC LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("reports: list exports: %w", err)
	}
	defer rows.Close()

	var out []Export
	for rows.Next() {
		var e Export
		if err := rows.Scan(&e.ID, &e.Kind, &e.Period, &e.CSVBlob, &e.JSONBlob, &e.Delivered, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("reports: scan export: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
