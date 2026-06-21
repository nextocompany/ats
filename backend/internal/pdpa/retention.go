package pdpa

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/activity"
)

// redactedName replaces an anonymized candidate's full_name. The column is NOT
// NULL, so a sentinel ("[deleted]") is used rather than NULL.
const redactedName = "[ลบข้อมูลแล้ว]"

// defaultRetentionDays / defaultBatch guard against a misconfigured (<=0) value
// reaching the sweep — a zero window would erase everything immediately.
const (
	defaultRetentionDays = 365
	defaultBatch         = 500
)

// BlobDeleter is the subset of blob.Client erasure needs. Stored values are full
// URLs for some rows (applications, seeded data) but bare keys for portal uploads,
// so both deletion forms are required; EraseSubject picks per value.
type BlobDeleter interface {
	Delete(ctx context.Context, name string) error
	DeleteStored(ctx context.Context, storedURL string) error
}

// SearchEraser removes a forgotten subject's documents from the search index.
// Satisfied by search.Indexer; nil is allowed (index erasure is then skipped).
type SearchEraser interface {
	Delete(ctx context.Context, candidateIDs []string) error
}

// RetentionService erases candidates whose retention window has elapsed and who
// are no longer in an active pipeline. Erasure de-identifies PII in place (rows
// are kept for referential integrity + aggregate analytics), deletes PII-bearing
// rows (onboarding docs, generated letters), purges every linked blob, and
// removes the subject from the search index.
type RetentionService struct {
	pool          *pgxpool.Pool
	blob          BlobDeleter
	index         SearchEraser
	audit         activity.Writer
	retentionDays int
}

// NewRetentionService builds the retention sweep service. index may be nil (search
// erasure is then skipped - the noop indexer is the usual mock value).
func NewRetentionService(pool *pgxpool.Pool, blob BlobDeleter, index SearchEraser, audit activity.Writer, retentionDays int) *RetentionService {
	return &RetentionService{pool: pool, blob: blob, index: index, audit: audit, retentionDays: retentionDays}
}

// Sweep anonymizes up to batch eligible candidates and returns the count erased.
// Each candidate is committed independently so a per-candidate failure does not
// abort the run, and the DB redaction is committed BEFORE best-effort blob
// deletion so a storage error cannot roll back erasure.
func (s *RetentionService) Sweep(ctx context.Context, batch int) (int, error) {
	days := s.retentionDays
	if days <= 0 {
		days = defaultRetentionDays
	}
	if batch <= 0 {
		batch = defaultBatch
	}

	ids, err := s.eligible(ctx, days, batch)
	if err != nil {
		return 0, err
	}

	var anonymized int
	for _, id := range ids {
		// EraseSubject runs the full cascade (DB redaction in one tx, then
		// best-effort blob + search-index cleanup). A per-candidate failure is
		// logged and skipped so one bad row never aborts the sweep.
		if err := s.EraseSubject(ctx, id); err != nil {
			log.Warn().Err(err).Str("candidate_id", id.String()).Msg("pdpa: erase failed; skipping candidate")
			continue
		}
		anonymized++

		if err := s.audit.Record(ctx, activity.ActionRetentionAnonymize, "candidate", id,
			map[string]any{"reason": "retention_window_elapsed"}); err != nil {
			log.Warn().Err(err).Str("candidate_id", id.String()).Msg("pdpa: retention audit record failed")
		}
	}
	return anonymized, nil
}

// eligible returns candidate IDs past the retention window that are neither in
// an active pipeline ('pending'/'parsed'/'scored') nor hired, not already
// anonymized, oldest first. Hired candidates are excluded so their PII is kept
// in the ATS beyond the window (client policy: hired records retained for HR/PS
// reconciliation).
func (s *RetentionService) eligible(ctx context.Context, days, batch int) ([]uuid.UUID, error) {
	const q = `
		SELECT c.id
		FROM candidates c
		WHERE c.pdpa_anonymized_at IS NULL
		  AND c.created_at < NOW() - make_interval(days => $1)
		  AND NOT EXISTS (
		      SELECT 1 FROM applications a
		      WHERE a.candidate_id = c.id
		        AND a.status IN ('pending','parsed','scored','hired')
		  )
		ORDER BY c.created_at
		LIMIT $2`
	rows, err := s.pool.Query(ctx, q, days, batch)
	if err != nil {
		return nil, fmt.Errorf("pdpa: eligible query: %w", err)
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("pdpa: scan eligible: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
