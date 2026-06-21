package pdpa

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

// ErrErasureHeld is returned by EraseByAccount when a linked subject is under
// legal hold (hired - employment data retained on a different lawful basis), so a
// self-service erasure must be queued for HR rather than auto-fulfilled.
var ErrErasureHeld = errors.New("pdpa: erasure held (hired/legal-hold subject)")

// EraseByAccount fully erases a portal account and every candidate behind it,
// used by portal self-service DSAR erasure. It first refuses (ErrErasureHeld) if
// any linked candidate is hired, so employment data under legal hold is never
// auto-erased. Otherwise it erases all linked candidates (EraseSubject, whose
// orphan logic also erases the account) and then force-erases the account to cover
// the no-application case. Idempotent.
func (s *RetentionService) EraseByAccount(ctx context.Context, accountID uuid.UUID) error {
	var held int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM applications a
		 JOIN candidates c ON c.id = a.candidate_id
		 WHERE c.account_id = $1 AND a.status = 'hired'`, accountID,
	).Scan(&held); err != nil {
		return fmt.Errorf("pdpa: erasure hold check: %w", err)
	}
	if held > 0 {
		return ErrErasureHeld
	}

	if err := s.EraseLinkedCandidates(ctx, accountID); err != nil {
		return err
	}
	return s.eraseAccountDirect(ctx, accountID)
}

// eraseAccountDirect erases a portal account unconditionally (no orphan guard - the
// caller has already erased every linked candidate), then purges its sessions, CRM
// notes/tags, and OTPs, and deletes its resume blob. Idempotent: an already-
// anonymized account is a no-op.
func (s *RetentionService) eraseAccountDirect(ctx context.Context, accountID uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pdpa: erase account begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const eraseAccount = `
		WITH old AS (SELECT resume_blob_url, email FROM candidate_accounts WHERE id = $1 FOR UPDATE)
		UPDATE candidate_accounts SET
			full_name        = $2,
			email            = NULL,
			email_verified   = FALSE,
			phone            = NULL,
			line_user_id     = NULL,
			line_display_id  = NULL,
			google_sub       = NULL,
			province         = NULL,
			resume_blob_url   = NULL,
			resume_file_type  = NULL,
			status            = 'anonymized',
			anonymized_at     = NOW(),
			updated_at        = NOW()
		WHERE id = $1 AND status <> 'anonymized'
		RETURNING (SELECT resume_blob_url FROM old), (SELECT email FROM old)`

	var oldResume, oldEmail *string
	err = tx.QueryRow(ctx, eraseAccount, accountID, redactedName).Scan(&oldResume, &oldEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // already anonymized (e.g. by the orphan logic) - nothing to do
	}
	if err != nil {
		return fmt.Errorf("pdpa: erase account: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM candidate_sessions WHERE account_id = $1`, accountID); err != nil {
		return fmt.Errorf("pdpa: erase account sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM member_notes WHERE account_id = $1`, accountID); err != nil {
		return fmt.Errorf("pdpa: erase account notes: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM member_tags WHERE account_id = $1`, accountID); err != nil {
		return fmt.Errorf("pdpa: erase account tags: %w", err)
	}
	if oldEmail != nil && *oldEmail != "" {
		if _, err := tx.Exec(ctx, `DELETE FROM email_otps WHERE email = $1`, *oldEmail); err != nil {
			return fmt.Errorf("pdpa: erase account otps: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("pdpa: erase account commit: %w", err)
	}

	if oldResume != nil && *oldResume != "" {
		if derr := s.deleteBlob(ctx, *oldResume); derr != nil {
			log.Warn().Err(derr).Str("account_id", accountID.String()).Msg("pdpa: account resume blob delete failed (orphaned)")
		}
	}
	return nil
}

// EraseSubject irreversibly erases ALL personal data linked to one candidate: it
// de-identifies the candidate row + every linked record's free-text PII, deletes
// PII-bearing rows (onboarding documents, generated letters), purges every linked
// blob from storage, and removes the subject from the search index. When the
// candidate is the LAST un-erased candidate of its portal account, the account
// (and its sessions / CRM notes / tags / resume) is erased too; an account with a
// still-active sibling candidate is left intact.
//
// One transaction performs all DB redaction (atomic); blob + search-index cleanup
// is best-effort after commit (an orphaned private blob with no DB pointer is
// low-risk, and a re-sweep / re-request can retry). The candidate guard
// (pdpa_anonymized_at IS NULL) plus the leading short-circuit make it idempotent.
func (s *RetentionService) EraseSubject(ctx context.Context, candidateID uuid.UUID) error {
	var (
		anonymizedAt *string
		accountID    *uuid.UUID
	)
	err := s.pool.QueryRow(ctx,
		`SELECT pdpa_anonymized_at::text, account_id FROM candidates WHERE id = $1`, candidateID,
	).Scan(&anonymizedAt, &accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // nothing to erase
	}
	if err != nil {
		return fmt.Errorf("pdpa: erase lookup: %w", err)
	}
	if anonymizedAt != nil {
		return nil // already erased - idempotent no-op
	}

	// Collect candidate-side blob URLs BEFORE redaction nulls/deletes their rows.
	blobs, err := s.candidateBlobs(ctx, candidateID)
	if err != nil {
		return err
	}

	accountResume, err := s.eraseRows(ctx, candidateID, accountID)
	if err != nil {
		return err
	}

	// DB redaction committed - the rest is best-effort cleanup of external stores.
	for _, b := range blobs {
		if derr := s.deleteBlob(ctx, b); derr != nil {
			log.Warn().Err(derr).Str("candidate_id", candidateID.String()).Msg("pdpa: blob delete failed (orphaned)")
		}
	}
	if accountResume != "" {
		if derr := s.deleteBlob(ctx, accountResume); derr != nil {
			log.Warn().Err(derr).Str("candidate_id", candidateID.String()).Msg("pdpa: account resume blob delete failed (orphaned)")
		}
	}
	if s.index != nil {
		if derr := s.index.Delete(ctx, []string{candidateID.String()}); derr != nil {
			log.Warn().Err(derr).Str("candidate_id", candidateID.String()).Msg("pdpa: search index delete failed")
		}
	}
	return nil
}

// EraseLinkedCandidates erases every candidate linked to a portal account, used by
// the members admin erasure route so that anonymizing an account also forgets the
// applicant data behind it. Best-effort per candidate: a single failure is logged
// and the rest continue (the account itself is redacted by the members path).
func (s *RetentionService) EraseLinkedCandidates(ctx context.Context, accountID uuid.UUID) error {
	rows, err := s.pool.Query(ctx, `SELECT id FROM candidates WHERE account_id = $1`, accountID)
	if err != nil {
		return fmt.Errorf("pdpa: linked candidates query: %w", err)
	}
	var ids []uuid.UUID
	func() {
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err = rows.Scan(&id); err != nil {
				return
			}
			ids = append(ids, id)
		}
	}()
	if err != nil {
		return fmt.Errorf("pdpa: scan linked candidate: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("pdpa: linked candidates rows: %w", err)
	}

	var failed int
	for _, id := range ids {
		if err := s.EraseSubject(ctx, id); err != nil {
			failed++
			log.Warn().Err(err).Str("candidate_id", id.String()).Str("account_id", accountID.String()).
				Msg("pdpa: erase linked candidate failed")
		}
	}
	if failed > 0 {
		return fmt.Errorf("pdpa: %d of %d linked candidates not fully erased", failed, len(ids))
	}
	return nil
}

// candidateBlobs returns every non-empty PII blob URL linked to a candidate:
// the four resume-derived application blobs plus onboarding-document scans and
// generated letters (both PII-bearing). The account resume blob is handled
// separately because it is only deleted when the account itself is erased.
func (s *RetentionService) candidateBlobs(ctx context.Context, candidateID uuid.UUID) ([]string, error) {
	const q = `
		SELECT url FROM (
			SELECT resume_blob_url        AS url FROM applications WHERE candidate_id = $1
			UNION ALL
			SELECT raw_file_blob_url             FROM applications WHERE candidate_id = $1
			UNION ALL
			SELECT ocr_text_blob_url             FROM applications WHERE candidate_id = $1
			UNION ALL
			SELECT parsed_profile_blob_url       FROM applications WHERE candidate_id = $1
			UNION ALL
			SELECT od.blob_url
				FROM onboarding_documents od JOIN applications a ON a.id = od.application_id
				WHERE a.candidate_id = $1
			UNION ALL
			SELECT l.blob_url
				FROM letters l JOIN applications a ON a.id = l.application_id
				WHERE a.candidate_id = $1
		) u
		WHERE url IS NOT NULL AND url <> ''`
	rows, err := s.pool.Query(ctx, q, candidateID)
	if err != nil {
		return nil, fmt.Errorf("pdpa: candidate blobs query: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("pdpa: scan candidate blob: %w", err)
		}
		out = append(out, url)
	}
	return out, rows.Err()
}

// eraseRows performs all DB redaction for one candidate in a single transaction
// and returns the account resume blob URL when (and only when) the linked account
// was erased in this call (the last un-erased candidate), so the caller can purge
// it post-commit. The candidate guard re-checks pdpa_anonymized_at so overlapping
// runs cannot double-process.
func (s *RetentionService) eraseRows(ctx context.Context, candidateID uuid.UUID, accountID *uuid.UUID) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("pdpa: erase begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock the candidate row and re-check inside the tx: this serializes against a
	// concurrent sweep / admin erase (the outer short-circuit read is unlocked) so
	// the rest of the cascade runs exactly once.
	var alreadyErased bool
	err = tx.QueryRow(ctx,
		`SELECT pdpa_anonymized_at IS NOT NULL FROM candidates WHERE id = $1 FOR UPDATE`, candidateID,
	).Scan(&alreadyErased)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("pdpa: erase lock: %w", err)
	}
	if alreadyErased {
		return "", nil // a concurrent erase already handled this subject
	}

	// 1) The candidate's own direct identifiers, including the LINE OAuth subject
	//    (a unique per-person platform id).
	if _, err := tx.Exec(ctx,
		`UPDATE candidates SET
			full_name = $2,
			phone = NULL, email = NULL, id_card = NULL,
			address = NULL, date_of_birth = NULL, line_user_id = NULL,
			pdpa_anonymized_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND pdpa_anonymized_at IS NULL`,
		candidateID, redactedName); err != nil {
		return "", fmt.Errorf("pdpa: redact candidate: %w", err)
	}

	// 2) Applications: resume-derived pointers + AI free-text.
	if _, err := tx.Exec(ctx,
		`UPDATE applications SET
			resume_blob_url = NULL, resume_original_filename = NULL,
			raw_file_blob_url = NULL, ocr_text_blob_url = NULL, parsed_profile_blob_url = NULL,
			ai_summary = NULL, ai_red_flags = NULL, updated_at = NOW()
		 WHERE candidate_id = $1`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact applications: %w", err)
	}

	// 3) Consent ledger IPs (consent rows are retained as consent-of-record).
	if _, err := tx.Exec(ctx,
		`UPDATE pdpa_consents SET ip_address = NULL WHERE candidate_id = $1`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact consents: %w", err)
	}

	// 4) AI pre-interview transcript + evaluation free-text. The access_token is a
	//    live bearer credential for the candidate's interview link, so it is
	//    tombstoned (NOT NULL + UNIQUE) to revoke it. The numeric interview_score /
	//    recommendation enum are kept as de-identified aggregates (same policy as
	//    applications.ai_score), now unlinkable to a person.
	if _, err := tx.Exec(ctx,
		`UPDATE interview_sessions SET
			conversation = '[]'::jsonb, summary = NULL, strengths = NULL, concerns = NULL,
			access_token = 'erased:' || id::text
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact interview sessions: %w", err)
	}

	// 5) Cross-position fit analysis free-text.
	if _, err := tx.Exec(ctx,
		`UPDATE application_fit_analyses SET
			summary = '', strengths = '[]'::jsonb, concerns = '[]'::jsonb, no_match_reason = ''
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact fit analyses: %w", err)
	}

	// 6) Human interview feedback free-text.
	if _, err := tx.Exec(ctx,
		`UPDATE interview_feedback SET strengths = NULL, concerns = NULL, notes = NULL
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact interview feedback: %w", err)
	}

	// 7) Offer free-text + the candidate's proposed salary (financial data, PDPA
	//    s.26 sensitive category) and personal start date.
	if _, err := tx.Exec(ctx,
		`UPDATE offers SET terms = NULL, decline_reason = NULL, salary = NULL, start_date = NULL
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact offers: %w", err)
	}

	// 7a) Approval-workflow free-text written about the application (approver
	//     comments + the final decision reason).
	if _, err := tx.Exec(ctx,
		`UPDATE approval_steps SET comment = NULL
		 WHERE request_id IN (
			SELECT id FROM approval_requests
			WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)
		 )`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact approval steps: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE approval_requests SET decision_reason = NULL
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact approval requests: %w", err)
	}

	// 7b) Interview appointment links tied to the subject (Teams join URL +
	//     calendar event id + free-text location); scheduled_at/mode kept for
	//     aggregate funnel analytics.
	if _, err := tx.Exec(ctx,
		`UPDATE interview_appointments SET
			location_text = NULL, online_join_url = NULL, calendar_event_id = NULL
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact interview appointments: %w", err)
	}

	// 8) Onboarding document scans (special-category data) - rows deleted, blobs
	//    already collected for post-commit storage deletion.
	if _, err := tx.Exec(ctx,
		`DELETE FROM onboarding_documents
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: delete onboarding documents: %w", err)
	}

	// 9) Generated letters - rows deleted, blobs collected.
	if _, err := tx.Exec(ctx,
		`DELETE FROM letters
		 WHERE application_id IN (SELECT id FROM applications WHERE candidate_id = $1)`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: delete letters: %w", err)
	}

	// 10) Notification payloads (carry name / contact snapshots).
	if _, err := tx.Exec(ctx,
		`UPDATE notifications SET payload = NULL WHERE candidate_id = $1`, candidateID); err != nil {
		return "", fmt.Errorf("pdpa: redact notifications: %w", err)
	}

	// 10a) Re-engagement outreach history about this person (contact targets +
	//      send/response logs) - deleted to honor erasure.
	for _, del := range []string{
		`DELETE FROM reengagement_contacts WHERE candidate_id = $1`,
		`DELETE FROM reengagement_logs WHERE candidate_id = $1`,
	} {
		if _, err := tx.Exec(ctx, del, candidateID); err != nil {
			return "", fmt.Errorf("pdpa: delete reengagement rows: %w", err)
		}
	}

	// 11) The linked portal account - only when this was its last un-erased
	//     candidate (a still-active sibling keeps the account alive).
	accountResume, err := s.eraseAccountIfOrphan(ctx, tx, candidateID, accountID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("pdpa: erase commit: %w", err)
	}
	return accountResume, nil
}

// eraseAccountIfOrphan redacts the candidate's portal account (and deletes its
// sessions, CRM notes, and tags) ONLY when no other un-erased candidate links to
// it - the current candidate is already marked anonymized at this point in the tx,
// so the NOT EXISTS check sees only siblings. Returns the account's pre-erase
// resume blob URL when the account was erased, else "". A nil accountID (walk-in
// candidate with no account) is a no-op.
func (s *RetentionService) eraseAccountIfOrphan(ctx context.Context, tx pgx.Tx, candidateID uuid.UUID, accountID *uuid.UUID) (string, error) {
	if accountID == nil {
		return "", nil
	}

	// The CTE snapshots the pre-erase resume blob URL and email before the UPDATE
	// nulls them; RETURNING reads from that snapshot. RETURNING yields a row only
	// when the UPDATE matched (last un-erased candidate), so ErrNoRows means the
	// account was kept (active sibling) or already anonymized.
	const eraseAccount = `
		WITH old AS (SELECT resume_blob_url, email FROM candidate_accounts WHERE id = $1 FOR UPDATE)
		UPDATE candidate_accounts SET
			full_name        = $3,
			email            = NULL,
			email_verified   = FALSE,
			phone            = NULL,
			line_user_id     = NULL,
			line_display_id  = NULL,
			google_sub       = NULL,
			province         = NULL,
			resume_blob_url   = NULL,
			resume_file_type  = NULL,
			status            = 'anonymized',
			anonymized_at     = NOW(),
			updated_at        = NOW()
		WHERE id = $1
		  AND status <> 'anonymized'
		  AND NOT EXISTS (
			SELECT 1 FROM candidates c
			WHERE c.account_id = $1 AND c.id <> $2 AND c.pdpa_anonymized_at IS NULL
		  )
		RETURNING (SELECT resume_blob_url FROM old), (SELECT email FROM old)`

	var oldResume, oldEmail *string
	err := tx.QueryRow(ctx, eraseAccount, *accountID, candidateID, redactedName).Scan(&oldResume, &oldEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil // account kept (active sibling) or already anonymized
	}
	if err != nil {
		return "", fmt.Errorf("pdpa: erase account: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM candidate_sessions WHERE account_id = $1`, *accountID); err != nil {
		return "", fmt.Errorf("pdpa: erase account sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM member_notes WHERE account_id = $1`, *accountID); err != nil {
		return "", fmt.Errorf("pdpa: erase account notes: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM member_tags WHERE account_id = $1`, *accountID); err != nil {
		return "", fmt.Errorf("pdpa: erase account tags: %w", err)
	}
	// email_otps is keyed by email (no account_id), so purge by the pre-erase email
	// captured above; otherwise the address lingers until the OTP-expiry sweep.
	if oldEmail != nil && *oldEmail != "" {
		if _, err := tx.Exec(ctx, `DELETE FROM email_otps WHERE email = $1`, *oldEmail); err != nil {
			return "", fmt.Errorf("pdpa: erase account otps: %w", err)
		}
	}

	if oldResume != nil {
		return *oldResume, nil
	}
	return "", nil
}

// deleteBlob removes one stored blob, choosing the deletion form by value shape:
// a full URL (applications, seeded rows) via DeleteStored, a bare key (portal
// uploads) via Delete.
func (s *RetentionService) deleteBlob(ctx context.Context, v string) error {
	if strings.Contains(v, "://") {
		return s.blob.DeleteStored(ctx, v)
	}
	return s.blob.Delete(ctx, v)
}
