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

// BlobDeleter is the subset of blob.Client the sweep needs.
type BlobDeleter interface {
	DeleteStored(ctx context.Context, storedURL string) error
}

// RetentionService anonymizes candidates whose retention window has elapsed and
// who are no longer in an active pipeline. It de-identifies in place (rows are
// kept for referential integrity + aggregate analytics) and deletes resume blobs.
type RetentionService struct {
	pool          *pgxpool.Pool
	blob          BlobDeleter
	audit         activity.Writer
	retentionDays int
}

// NewRetentionService builds the retention sweep service.
func NewRetentionService(pool *pgxpool.Pool, blob BlobDeleter, audit activity.Writer, retentionDays int) *RetentionService {
	return &RetentionService{pool: pool, blob: blob, audit: audit, retentionDays: retentionDays}
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
		blobURLs, err := s.piiBlobs(ctx, id)
		if err != nil {
			log.Warn().Err(err).Str("candidate_id", id.String()).Msg("pdpa: gather pii blobs failed; skipping candidate")
			continue
		}
		if err := s.anonymize(ctx, id); err != nil {
			log.Warn().Err(err).Str("candidate_id", id.String()).Msg("pdpa: anonymize failed; skipping candidate")
			continue
		}
		anonymized++

		// DB redaction committed — remaining work is best-effort. The blob URL
		// pointer is already gone, so an orphaned blob is unreachable and the
		// candidate (now marked) is not re-found on the next run.
		for _, url := range blobURLs {
			if err := s.blob.DeleteStored(ctx, url); err != nil {
				log.Warn().Err(err).Str("candidate_id", id.String()).Msg("pdpa: resume blob delete failed")
			}
		}
		if err := s.audit.Record(ctx, activity.ActionRetentionAnonymize, "candidate", id,
			map[string]any{"reason": "retention_window_elapsed"}); err != nil {
			log.Warn().Err(err).Str("candidate_id", id.String()).Msg("pdpa: retention audit record failed")
		}
	}
	return anonymized, nil
}

// eligible returns candidate IDs past the retention window with no active
// application, not already anonymized, oldest first.
func (s *RetentionService) eligible(ctx context.Context, days, batch int) ([]uuid.UUID, error) {
	const q = `
		SELECT c.id
		FROM candidates c
		WHERE c.pdpa_anonymized_at IS NULL
		  AND c.created_at < NOW() - make_interval(days => $1)
		  AND NOT EXISTS (
		      SELECT 1 FROM applications a
		      WHERE a.candidate_id = c.id
		        AND a.status IN ('pending','parsed','scored')
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

// piiBlobs returns every non-empty resume-derived blob URL for a candidate's
// applications — the original upload (resume_blob_url / raw_file_blob_url), the
// OCR text, and the parsed profile — captured before redaction nulls the
// pointers. All hold candidate PII and must be erased from storage.
func (s *RetentionService) piiBlobs(ctx context.Context, candidateID uuid.UUID) ([]string, error) {
	const q = `
		SELECT url FROM (
			SELECT resume_blob_url        AS url FROM applications WHERE candidate_id = $1
			UNION ALL
			SELECT raw_file_blob_url             FROM applications WHERE candidate_id = $1
			UNION ALL
			SELECT ocr_text_blob_url             FROM applications WHERE candidate_id = $1
			UNION ALL
			SELECT parsed_profile_blob_url       FROM applications WHERE candidate_id = $1
		) u
		WHERE url IS NOT NULL AND url <> ''`
	rows, err := s.pool.Query(ctx, q, candidateID)
	if err != nil {
		return nil, fmt.Errorf("pdpa: resume blobs query: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("pdpa: scan pii blob: %w", err)
		}
		out = append(out, url)
	}
	return out, rows.Err()
}

// anonymize redacts a candidate's PII, its applications' PII pointers/free-text,
// and its consent-ledger IPs in a single transaction. The candidate WHERE re-checks
// pdpa_anonymized_at so overlapping runs cannot double-process.
func (s *RetentionService) anonymize(ctx context.Context, candidateID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pdpa: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`UPDATE candidates SET
			full_name = $2,
			phone = NULL, email = NULL, id_card = NULL,
			address = NULL, date_of_birth = NULL,
			pdpa_anonymized_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND pdpa_anonymized_at IS NULL`,
		candidateID, redactedName); err != nil {
		return fmt.Errorf("pdpa: anonymize candidate: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE applications SET
			resume_blob_url = NULL, resume_original_filename = NULL,
			raw_file_blob_url = NULL, ocr_text_blob_url = NULL, parsed_profile_blob_url = NULL,
			ai_summary = NULL, ai_red_flags = NULL, updated_at = NOW()
		 WHERE candidate_id = $1`,
		candidateID); err != nil {
		return fmt.Errorf("pdpa: redact applications: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE pdpa_consents SET ip_address = NULL WHERE candidate_id = $1`,
		candidateID); err != nil {
		return fmt.Errorf("pdpa: redact consents: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pdpa: commit: %w", err)
	}
	return nil
}
