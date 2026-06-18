package applications

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const onboardingColumns = `id, application_id, doc_type, status, blob_url, COALESCE(file_name,''), COALESCE(file_type,''), COALESCE(review_reason,''), reviewed_by, uploaded_at, reviewed_at`

func scanOnboardingDoc(row pgx.Row) (OnboardingDocument, error) {
	var d OnboardingDocument
	err := row.Scan(&d.ID, &d.ApplicationID, &d.DocType, &d.Status, &d.BlobURL,
		&d.FileName, &d.FileType, &d.ReviewReason, &d.ReviewedBy, &d.UploadedAt, &d.ReviewedAt)
	return d, err
}

// UpsertOnboardingDocument stores (or replaces) the candidate's current document
// for an (application, doc_type). A re-upload resets the document to 'pending' and
// clears the prior review (the pending → rejected → re-upload → pending cycle).
func (r *pgRepository) UpsertOnboardingDocument(ctx context.Context, applicationID uuid.UUID, docType, blobURL, fileName, fileType string, uploadedBy uuid.UUID) (OnboardingDocument, error) {
	const q = `
		INSERT INTO onboarding_documents (application_id, doc_type, status, blob_url, file_name, file_type, uploaded_by, uploaded_at)
		VALUES ($1, $2, 'pending', $3, NULLIF($4,''), NULLIF($5,''), $6, NOW())
		ON CONFLICT (application_id, doc_type)
		DO UPDATE SET status = 'pending', blob_url = EXCLUDED.blob_url, file_name = EXCLUDED.file_name,
		             file_type = EXCLUDED.file_type, review_reason = NULL, reviewed_by = NULL, reviewed_at = NULL,
		             uploaded_by = EXCLUDED.uploaded_by, uploaded_at = NOW(), updated_at = NOW()
		RETURNING ` + onboardingColumns
	d, err := scanOnboardingDoc(r.pool.QueryRow(ctx, q, applicationID, docType, blobURL, fileName, fileType, uploadedBy))
	if err != nil {
		return OnboardingDocument{}, fmt.Errorf("applications: upsert onboarding document: %w", err)
	}
	return d, nil
}

// ListOnboardingByApplication returns all onboarding documents for an application,
// ordered by doc_type for a stable checklist.
func (r *pgRepository) ListOnboardingByApplication(ctx context.Context, applicationID uuid.UUID) ([]OnboardingDocument, error) {
	const q = `SELECT ` + onboardingColumns + ` FROM onboarding_documents WHERE application_id = $1 ORDER BY doc_type`
	rows, err := r.pool.Query(ctx, q, applicationID)
	if err != nil {
		return nil, fmt.Errorf("applications: list onboarding documents: %w", err)
	}
	defer rows.Close()
	out := make([]OnboardingDocument, 0)
	for rows.Next() {
		d, err := scanOnboardingDoc(rows)
		if err != nil {
			return nil, fmt.Errorf("applications: scan onboarding document: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate onboarding documents: %w", err)
	}
	return out, nil
}

// GetOnboardingDocByID fetches a single onboarding document, or nil when absent.
func (r *pgRepository) GetOnboardingDocByID(ctx context.Context, id uuid.UUID) (*OnboardingDocument, error) {
	const q = `SELECT ` + onboardingColumns + ` FROM onboarding_documents WHERE id = $1`
	d, err := scanOnboardingDoc(r.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("applications: get onboarding document: %w", err)
	}
	return &d, nil
}

// ReviewOnboardingDocument records an HR approve/reject in one transaction. The
// document is locked + scoped to its application; a guarded UPDATE that touches no
// rows (concurrent change) surfaces as ErrOnboardingDocConflict. HR may correct a
// prior decision (the guard is id+application, not status).
func (r *pgRepository) ReviewOnboardingDocument(ctx context.Context, docID, applicationID, reviewerID uuid.UUID, approve bool, reason string) (OnboardingDocument, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return OnboardingDocument{}, fmt.Errorf("applications: begin review onboarding: %w", err)
	}
	defer tx.Rollback(ctx)

	var existing uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id FROM onboarding_documents WHERE id = $1 AND application_id = $2 FOR UPDATE`,
		docID, applicationID).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return OnboardingDocument{}, ErrOnboardingDocNotFound
	}
	if err != nil {
		return OnboardingDocument{}, fmt.Errorf("applications: lock onboarding document: %w", err)
	}

	status := OnbApproved
	if !approve {
		status = OnbRejected
	}
	tag, err := tx.Exec(ctx,
		`UPDATE onboarding_documents
		 SET status = $3, review_reason = NULLIF($4,''), reviewed_by = $5, reviewed_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND application_id = $2`,
		docID, applicationID, status, reason, reviewerID)
	if err != nil {
		return OnboardingDocument{}, fmt.Errorf("applications: review onboarding document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return OnboardingDocument{}, ErrOnboardingDocConflict
	}

	if err := tx.Commit(ctx); err != nil {
		return OnboardingDocument{}, fmt.Errorf("applications: commit review onboarding: %w", err)
	}
	got, err := r.GetOnboardingDocByID(ctx, docID)
	if err != nil {
		return OnboardingDocument{}, err
	}
	if got == nil {
		return OnboardingDocument{}, fmt.Errorf("applications: onboarding document vanished after review")
	}
	return *got, nil
}

// FindHiredApplicationByAccount resolves the candidate's hired application from
// their membership account (most recent hire wins). This is how the candidate
// onboarding endpoints scope to a single application without the client ever
// passing an application id.
func (r *pgRepository) FindHiredApplicationByAccount(ctx context.Context, accountID uuid.UUID) (uuid.UUID, error) {
	const q = `
		SELECT a.id
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id
		WHERE c.account_id = $1 AND a.status = $2
		ORDER BY a.hired_at DESC NULLS LAST, a.created_at DESC
		LIMIT 1`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, q, accountID, StatusHired).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrOnboardingNoHiredApp
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("applications: find hired application by account: %w", err)
	}
	return id, nil
}
