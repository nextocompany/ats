package members

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// redactedName replaces the member's full name on anonymization (matches the
// PDPA retention sweep's redaction marker).
const redactedName = "[ลบข้อมูลแล้ว]"

// SetStatus toggles a member between active and suspended. The transaction takes a
// row lock first so a concurrent anonymize can't slip the row into 'anonymized'
// between the guard read and the update. Suspending also deletes the member's
// sessions (immediate force-logout); the candidateauth session-resolve query
// additionally rejects any session whose account is non-active, so even a session
// created in a race stops working.
func (r *pgRepository) SetStatus(ctx context.Context, id uuid.UUID, status string, by *uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("members: set status begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current string
	err = tx.QueryRow(ctx, `SELECT status FROM candidate_accounts WHERE id = $1 FOR UPDATE`, id).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("members: set status lock: %w", err)
	}
	if current == StatusAnonymized {
		return ErrAnonymized // erasure is terminal — never reactivate or re-suspend
	}

	// $2 is cast to text everywhere so Postgres doesn't deduce conflicting types
	// (status column is varchar, the CASE comparisons want text) for one parameter.
	const upd = `
		UPDATE candidate_accounts SET
			status       = $2::text,
			suspended_at = CASE WHEN $2::text = 'suspended' THEN NOW() ELSE NULL END,
			suspended_by = CASE WHEN $2::text = 'suspended' THEN $3::uuid ELSE NULL END,
			updated_at   = NOW()
		WHERE id = $1`
	if _, err := tx.Exec(ctx, upd, id, status, by); err != nil {
		return fmt.Errorf("members: set status update: %w", err)
	}
	if status == StatusSuspended {
		if _, err := tx.Exec(ctx, `DELETE FROM candidate_sessions WHERE account_id = $1`, id); err != nil {
			return fmt.Errorf("members: set status revoke sessions: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("members: set status commit: %w", err)
	}
	return nil
}

// ForceLogout deletes the member's sessions without changing status (a manual
// "sign out everywhere"). Returns ErrNotFound when the member is missing and
// ErrAnonymized for an erased account (consistent with the other lifecycle ops).
func (r *pgRepository) ForceLogout(ctx context.Context, id uuid.UUID) error {
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM candidate_accounts WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("members: force logout lookup: %w", err)
	}
	if status == StatusAnonymized {
		return ErrAnonymized
	}
	if _, err := r.pool.Exec(ctx, `DELETE FROM candidate_sessions WHERE account_id = $1`, id); err != nil {
		return fmt.Errorf("members: force logout: %w", err)
	}
	return nil
}

// UpdateProfile applies a sparse admin edit (empty fields are left unchanged).
// Anonymized accounts are immutable. Returns ErrNotFound / ErrAnonymized on miss.
func (r *pgRepository) UpdateProfile(ctx context.Context, id uuid.UUID, p ProfileUpdate) error {
	const upd = `
		UPDATE candidate_accounts SET
			full_name = COALESCE(NULLIF($2,''), full_name),
			phone     = COALESCE(NULLIF($3,''), phone),
			province  = COALESCE(NULLIF($4,''), province),
			email     = COALESCE(NULLIF($5,''), email),
			updated_at = NOW()
		WHERE id = $1 AND status <> 'anonymized'`
	tag, err := r.pool.Exec(ctx, upd, id, p.FullName, p.Phone, p.Province, p.Email)
	if isUnique(err) {
		return ErrEmailTaken // email collides with another account's unique address
	}
	if err != nil {
		return fmt.Errorf("members: update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return r.classifyMiss(ctx, id) // missing vs anonymized
	}
	return nil
}

// Anonymize irreversibly redacts the account in a transaction and returns the
// resume blob URL (if any) for post-commit deletion. The status guard makes it
// idempotent: a second call finds status='anonymized' and returns ErrAnonymized.
// The erasing actor is captured in activity_logs by the handler (the row has no
// anonymized_by column, and suspended_by is left untouched so a prior suspension's
// actor isn't overwritten). email_verified is cleared too — it's meaningless once
// the email is gone and would otherwise leak "this account had a verified email".
// PDPA consent fields (pdpa_consent/version/consent_at) are intentionally retained
// as consent-of-record proof; no direct identifier remains after redaction.
func (r *pgRepository) Anonymize(ctx context.Context, id uuid.UUID) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("members: anonymize begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status, resumeURL string
	err = tx.QueryRow(ctx,
		`SELECT status, COALESCE(resume_blob_url,'') FROM candidate_accounts WHERE id = $1 FOR UPDATE`, id,
	).Scan(&status, &resumeURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("members: anonymize lock: %w", err)
	}
	if status == StatusAnonymized {
		return "", ErrAnonymized
	}

	const redact = `
		UPDATE candidate_accounts SET
			full_name        = $2,
			email            = NULL,
			email_verified   = FALSE,
			phone            = NULL,
			line_user_id     = NULL,
			line_display_id  = NULL,
			google_sub       = NULL,
			province         = NULL,
			resume_blob_url  = NULL,
			resume_file_type = NULL,
			status           = 'anonymized',
			anonymized_at    = NOW(),
			updated_at       = NOW()
		WHERE id = $1`
	if _, err := tx.Exec(ctx, redact, id, redactedName); err != nil {
		return "", fmt.Errorf("members: anonymize redact: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM candidate_sessions WHERE account_id = $1`, id); err != nil {
		return "", fmt.Errorf("members: anonymize revoke sessions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("members: anonymize commit: %w", err)
	}
	return resumeURL, nil
}

// classifyMiss distinguishes a missing member (ErrNotFound) from an anonymized one
// (ErrAnonymized) after a guarded UPDATE affected zero rows.
func (r *pgRepository) classifyMiss(ctx context.Context, id uuid.UUID) error {
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM candidate_accounts WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("members: classify miss: %w", err)
	}
	if status == StatusAnonymized {
		return ErrAnonymized
	}
	return ErrNotFound
}
