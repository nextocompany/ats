package candidateauth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// uniqueViolation is the Postgres SQLSTATE for a unique-constraint violation.
const uniqueViolation = "23505"

// Repository is the candidate-account / session / OTP data-access contract.
type Repository interface {
	// FindOrCreateByEmail returns the account for a verified email, creating it
	// (email_verified=true) when absent.
	FindOrCreateByEmail(ctx context.Context, email string) (*Account, error)
	// FindOrCreateUnverifiedByEmail returns the account for an email, creating an
	// UNVERIFIED shell (email_verified=false) when absent. Unlike
	// FindOrCreateByEmail it never flips an existing account's verified flag — a CV
	// at intake does not verify the address. Caller must pass a normalized
	// (lowercased) email. Used by silent at-intake account provisioning (Phase 2).
	FindOrCreateUnverifiedByEmail(ctx context.Context, email string) (*Account, error)
	// FindOrCreateByLineSub returns the account for a verified LINE sub. When a
	// new account is created, name/email seed it; an existing email account is
	// linked rather than duplicated.
	FindOrCreateByLineSub(ctx context.Context, sub, name, email string) (*Account, error)
	// FindOrCreateByGoogleSub mirrors FindOrCreateByLineSub for Google.
	FindOrCreateByGoogleSub(ctx context.Context, sub, name, email string) (*Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)
	// LinkLine attaches a verified LINE sub (+ optional @id) to an existing account.
	LinkLine(ctx context.Context, accountID uuid.UUID, sub, displayID string) error
	UpdateProfile(ctx context.Context, accountID uuid.UUID, p ProfileUpdate) error
	SetResume(ctx context.Context, accountID uuid.UUID, blobURL, fileType string) error

	// Resume library (CV history, capped at MaxResumes, exactly one default).
	// The candidate_accounts.resume_blob_url/resume_file_type pointer is kept in
	// sync with the default by Insert/SetDefault/Delete so quick-apply is unchanged.
	ListResumes(ctx context.Context, accountID uuid.UUID) ([]Resume, error)
	CountResumes(ctx context.Context, accountID uuid.UUID) (int, error)
	InsertResume(ctx context.Context, accountID, id uuid.UUID, blobKey, filename, fileType string, makeDefault bool) error
	SetDefaultResume(ctx context.Context, accountID, resumeID uuid.UUID) error
	// DeleteResume removes a resume and returns its blob key (for blob cleanup).
	// If the deleted resume was the default, the newest remaining one is promoted.
	DeleteResume(ctx context.Context, accountID, resumeID uuid.UUID) (string, error)
	// ResumeBlobKey returns the blob key + file type for one resume, scoped to the
	// owning account (account_id from the session, never the request) so a member
	// can only resolve their own CVs. Returns ErrNotFound otherwise.
	ResumeBlobKey(ctx context.Context, accountID, resumeID uuid.UUID) (key, fileType string, err error)
	SetConsent(ctx context.Context, accountID uuid.UUID, version string) error
	// MarkConsented updates only the consent snapshot (no ledger row) for the apply
	// flow, where the candidate-keyed ledger row is the authoritative record.
	MarkConsented(ctx context.Context, accountID uuid.UUID, version string) error
	// WithdrawConsent flips the account to not-consented and appends a withdrawal
	// row to the unified pdpa_consents ledger.
	WithdrawConsent(ctx context.Context, accountID uuid.UUID, version string) error

	CreateSession(ctx context.Context, accountID uuid.UUID, tokenHash string, expiresAt time.Time) error
	// FindAccountBySessionHash returns the account for a live (unrevoked,
	// unexpired) session token hash, or ErrNotFound.
	FindAccountBySessionHash(ctx context.Context, tokenHash string) (*Account, error)
	RevokeSession(ctx context.Context, tokenHash string) error

	CreateOTP(ctx context.Context, email, codeHash string, expiresAt time.Time) error
	// ConsumeOTP atomically marks the newest live challenge for (email, codeHash)
	// consumed. Returns ErrOTPInvalid when none matches.
	ConsumeOTP(ctx context.Context, email, codeHash string) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed candidateauth repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

const accountColumns = `
	id, full_name, COALESCE(email,''), email_verified, COALESCE(phone,''),
	COALESCE(line_user_id,''), COALESCE(line_display_id,''), COALESCE(google_sub,''),
	COALESCE(province,''), COALESCE(resume_blob_url,''), COALESCE(resume_file_type,''),
	pdpa_consent, COALESCE(pdpa_version,''), status, created_at`

func scanAccount(row pgx.Row) (*Account, error) {
	var a Account
	if err := row.Scan(
		&a.ID, &a.FullName, &a.Email, &a.EmailVerified, &a.Phone,
		&a.LineUserID, &a.LineDisplayID, &a.GoogleSub,
		&a.Province, &a.ResumeBlobURL, &a.ResumeFileType,
		&a.PDPAConsent, &a.PDPAVersion, &a.Status, &a.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Account, error) {
	const q = `SELECT ` + accountColumns + ` FROM candidate_accounts WHERE id = $1`
	a, err := scanAccount(r.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("candidateauth: get by id: %w", err)
	}
	return a, nil
}

func (r *pgRepository) findByColumn(ctx context.Context, col, val string) (*Account, error) {
	q := `SELECT ` + accountColumns + ` FROM candidate_accounts WHERE ` + col + ` = $1`
	a, err := scanAccount(r.pool.QueryRow(ctx, q, val))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("candidateauth: find by %s: %w", col, err)
	}
	return a, nil
}

func (r *pgRepository) FindOrCreateByEmail(ctx context.Context, email string) (*Account, error) {
	if a, err := r.findByColumn(ctx, "email", email); err == nil {
		if !a.EmailVerified {
			_, _ = r.pool.Exec(ctx, `UPDATE candidate_accounts SET email_verified = TRUE, updated_at = NOW() WHERE id = $1`, a.ID)
			a.EmailVerified = true
		}
		return a, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	const ins = `
		INSERT INTO candidate_accounts (email, email_verified)
		VALUES ($1, TRUE)
		RETURNING ` + accountColumns
	a, err := scanAccount(r.pool.QueryRow(ctx, ins, email))
	if isUnique(err) { // lost a create race — read the winner
		return r.findByColumn(ctx, "email", email)
	}
	if err != nil {
		return nil, fmt.Errorf("candidateauth: create by email: %w", err)
	}
	return a, nil
}

func (r *pgRepository) FindOrCreateUnverifiedByEmail(ctx context.Context, email string) (*Account, error) {
	// Return an existing account as-is — never flip email_verified (a CV does not
	// verify the address; matches the Phase-1 backfill shell semantics).
	if a, err := r.findByColumn(ctx, "email", email); err == nil {
		return a, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	const ins = `
		INSERT INTO candidate_accounts (email, email_verified)
		VALUES ($1, FALSE)
		RETURNING ` + accountColumns
	a, err := scanAccount(r.pool.QueryRow(ctx, ins, email))
	if isUnique(err) { // lost a create race (concurrent intake) — read the winner
		return r.findByColumn(ctx, "email", email)
	}
	if err != nil {
		return nil, fmt.Errorf("candidateauth: ensure unverified by email: %w", err)
	}
	return a, nil
}

// findOrCreateBySub unifies the LINE/Google find-or-create flow: look up by the
// provider sub; else link onto an existing email account; else insert a fresh one.
func (r *pgRepository) findOrCreateBySub(ctx context.Context, subCol, sub, name, email string) (*Account, error) {
	if a, err := r.findByColumn(ctx, subCol, sub); err == nil {
		return a, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	// Unify by email: an account created via email-OTP gets this provider linked.
	if email != "" {
		if a, err := r.findByColumn(ctx, "email", email); err == nil {
			if _, uerr := r.pool.Exec(ctx,
				`UPDATE candidate_accounts SET `+subCol+` = $2, full_name = COALESCE(NULLIF(full_name,''), $3), updated_at = NOW() WHERE id = $1`,
				a.ID, sub, name); uerr != nil {
				return nil, fmt.Errorf("candidateauth: link %s: %w", subCol, uerr)
			}
			return r.GetByID(ctx, a.ID)
		} else if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}

	ins := `
		INSERT INTO candidate_accounts (` + subCol + `, full_name, email, email_verified)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + accountColumns
	a, err := scanAccount(r.pool.QueryRow(ctx, ins, sub, name, nullable(email), email != ""))
	if isUnique(err) { // race on sub or email — read the winner by sub
		return r.findByColumn(ctx, subCol, sub)
	}
	if err != nil {
		return nil, fmt.Errorf("candidateauth: create by %s: %w", subCol, err)
	}
	return a, nil
}

func (r *pgRepository) FindOrCreateByLineSub(ctx context.Context, sub, name, email string) (*Account, error) {
	return r.findOrCreateBySub(ctx, "line_user_id", sub, name, email)
}

func (r *pgRepository) FindOrCreateByGoogleSub(ctx context.Context, sub, name, email string) (*Account, error) {
	return r.findOrCreateBySub(ctx, "google_sub", sub, name, email)
}

func (r *pgRepository) LinkLine(ctx context.Context, accountID uuid.UUID, sub, displayID string) error {
	const q = `
		UPDATE candidate_accounts
		SET line_user_id = $2, line_display_id = COALESCE(NULLIF($3,''), line_display_id), updated_at = NOW()
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, accountID, sub, displayID)
	if isUnique(err) {
		return ErrLineLinkedToOther // the LINE sub belongs to a different account
	}
	if err != nil {
		return fmt.Errorf("candidateauth: link line: %w", err)
	}
	return nil
}

func (r *pgRepository) UpdateProfile(ctx context.Context, accountID uuid.UUID, p ProfileUpdate) error {
	const q = `
		UPDATE candidate_accounts SET
			full_name       = COALESCE(NULLIF($2,''), full_name),
			phone           = COALESCE(NULLIF($3,''), phone),
			line_display_id = COALESCE(NULLIF($4,''), line_display_id),
			province        = COALESCE(NULLIF($5,''), province),
			updated_at      = NOW()
		WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, accountID, p.FullName, p.Phone, p.LineDisplayID, p.Province); err != nil {
		return fmt.Errorf("candidateauth: update profile: %w", err)
	}
	return nil
}

func (r *pgRepository) SetResume(ctx context.Context, accountID uuid.UUID, blobURL, fileType string) error {
	const q = `UPDATE candidate_accounts SET resume_blob_url = $2, resume_file_type = $3, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, accountID, blobURL, fileType); err != nil {
		return fmt.Errorf("candidateauth: set resume: %w", err)
	}
	return nil
}

// syncDefaultPointer points candidate_accounts at the given default resume blob,
// keeping the denormalized quick-apply read path correct.
const syncDefaultPointer = `UPDATE candidate_accounts SET resume_blob_url = $2, resume_file_type = $3, updated_at = NOW() WHERE id = $1`

func (r *pgRepository) ListResumes(ctx context.Context, accountID uuid.UUID) ([]Resume, error) {
	const q = `SELECT id, COALESCE(original_filename,''), file_type, is_default, created_at
	           FROM candidate_account_resumes WHERE account_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, fmt.Errorf("candidateauth: list resumes: %w", err)
	}
	defer rows.Close()
	var out []Resume
	for rows.Next() {
		var res Resume
		if err := rows.Scan(&res.ID, &res.OriginalFilename, &res.FileType, &res.IsDefault, &res.CreatedAt); err != nil {
			return nil, fmt.Errorf("candidateauth: scan resume: %w", err)
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *pgRepository) CountResumes(ctx context.Context, accountID uuid.UUID) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM candidate_account_resumes WHERE account_id = $1`, accountID).Scan(&n); err != nil {
		return 0, fmt.Errorf("candidateauth: count resumes: %w", err)
	}
	return n, nil
}

// InsertResume adds a resume. makeDefault must only be set when the account has
// no other resume (the caller guarantees this), so no existing default is unset.
func (r *pgRepository) InsertResume(ctx context.Context, accountID, id uuid.UUID, blobKey, filename, fileType string, makeDefault bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("candidateauth: insert resume begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO candidate_account_resumes (id, account_id, blob_key, original_filename, file_type, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, accountID, blobKey, filename, fileType, makeDefault); err != nil {
		return fmt.Errorf("candidateauth: insert resume: %w", err)
	}
	if makeDefault {
		if _, err := tx.Exec(ctx, syncDefaultPointer, accountID, blobKey, fileType); err != nil {
			return fmt.Errorf("candidateauth: sync default pointer: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (r *pgRepository) SetDefaultResume(ctx context.Context, accountID, resumeID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("candidateauth: set default begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var blobKey, fileType string
	err = tx.QueryRow(ctx,
		`SELECT blob_key, file_type FROM candidate_account_resumes WHERE id = $1 AND account_id = $2`,
		resumeID, accountID).Scan(&blobKey, &fileType)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("candidateauth: set default lookup: %w", err)
	}
	// Two ordered statements, NOT a single `is_default = (id = $2)`: the partial
	// unique index (account_id WHERE is_default) is enforced per-row and is not
	// deferrable, so a single statement could momentarily have two defaults and
	// throw a unique violation. Clear the old default first, then set the new one.
	if _, err := tx.Exec(ctx,
		`UPDATE candidate_account_resumes SET is_default = FALSE WHERE account_id = $1 AND is_default`,
		accountID); err != nil {
		return fmt.Errorf("candidateauth: clear default: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE candidate_account_resumes SET is_default = TRUE WHERE id = $1 AND account_id = $2`,
		resumeID, accountID); err != nil {
		return fmt.Errorf("candidateauth: set default: %w", err)
	}
	if _, err := tx.Exec(ctx, syncDefaultPointer, accountID, blobKey, fileType); err != nil {
		return fmt.Errorf("candidateauth: sync default pointer: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *pgRepository) ResumeBlobKey(ctx context.Context, accountID, resumeID uuid.UUID) (string, string, error) {
	var key, fileType string
	err := r.pool.QueryRow(ctx,
		`SELECT blob_key, file_type FROM candidate_account_resumes WHERE id = $1 AND account_id = $2`,
		resumeID, accountID).Scan(&key, &fileType)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("candidateauth: resume blob key: %w", err)
	}
	return key, fileType, nil
}

func (r *pgRepository) DeleteResume(ctx context.Context, accountID, resumeID uuid.UUID) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("candidateauth: delete resume begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var blobKey string
	var wasDefault bool
	err = tx.QueryRow(ctx,
		`SELECT blob_key, is_default FROM candidate_account_resumes WHERE id = $1 AND account_id = $2`,
		resumeID, accountID).Scan(&blobKey, &wasDefault)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("candidateauth: delete lookup: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM candidate_account_resumes WHERE id = $1`, resumeID); err != nil {
		return "", fmt.Errorf("candidateauth: delete resume: %w", err)
	}
	if wasDefault {
		// Promote the newest remaining resume to default and re-point the account.
		var nid uuid.UUID
		var nkey, ntype string
		e := tx.QueryRow(ctx,
			`SELECT id, blob_key, file_type FROM candidate_account_resumes
			 WHERE account_id = $1 ORDER BY created_at DESC LIMIT 1`, accountID).Scan(&nid, &nkey, &ntype)
		switch {
		case e == nil:
			if _, err := tx.Exec(ctx, `UPDATE candidate_account_resumes SET is_default = TRUE WHERE id = $1`, nid); err != nil {
				return "", fmt.Errorf("candidateauth: promote default: %w", err)
			}
			if _, err := tx.Exec(ctx, syncDefaultPointer, accountID, nkey, ntype); err != nil {
				return "", fmt.Errorf("candidateauth: sync default pointer: %w", err)
			}
		case errors.Is(e, pgx.ErrNoRows):
			// No resumes left: clear the quick-apply pointer.
			if _, err := tx.Exec(ctx,
				`UPDATE candidate_accounts SET resume_blob_url = NULL, resume_file_type = NULL, updated_at = NOW() WHERE id = $1`,
				accountID); err != nil {
				return "", fmt.Errorf("candidateauth: clear resume pointer: %w", err)
			}
		default:
			return "", fmt.Errorf("candidateauth: promote lookup: %w", e)
		}
	}
	return blobKey, tx.Commit(ctx)
}

// MarkConsented updates ONLY the account consent snapshot (no ledger row). Used by
// the apply flow, where the authoritative ledger row is the candidate-keyed one
// written by the apply handler - this just stops the member being re-prompted.
func (r *pgRepository) MarkConsented(ctx context.Context, accountID uuid.UUID, version string) error {
	const q = `
		UPDATE candidate_accounts
		SET pdpa_consent = TRUE, pdpa_version = $2, pdpa_consent_at = NOW(), updated_at = NOW()
		WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, accountID, version)
	if err != nil {
		return fmt.Errorf("candidateauth: mark consented: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetConsent records account consent: it updates the account snapshot AND appends
// to the unified pdpa_consents ledger (account_id keyed) in one transaction, so the
// portal and apply flows share a single queryable consent trail. Used by the
// portal profile/signup consent path (no candidate row exists there yet).
func (r *pgRepository) SetConsent(ctx context.Context, accountID uuid.UUID, version string) error {
	return r.writeConsent(ctx, accountID, version, true, "account")
}

// WithdrawConsent records a consent withdrawal (PDPA s.19: as easy to withdraw as
// to give): it flips the account snapshot to not-consented AND appends a
// consent_given=false ledger row in one transaction.
func (r *pgRepository) WithdrawConsent(ctx context.Context, accountID uuid.UUID, version string) error {
	return r.writeConsent(ctx, accountID, version, false, "account_withdraw")
}

func (r *pgRepository) writeConsent(ctx context.Context, accountID uuid.UUID, version string, given bool, source string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("candidateauth: consent begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// On withdrawal (given=false) keep pdpa_version + pdpa_consent_at unchanged so
	// the snapshot still reflects WHEN/WHAT the member originally consented to; only
	// the boolean flips. The withdrawal event itself is captured in the ledger row.
	const snap = `
		UPDATE candidate_accounts SET
			pdpa_consent    = $2,
			pdpa_version    = CASE WHEN $2 THEN $3 ELSE pdpa_version END,
			pdpa_consent_at = CASE WHEN $2 THEN NOW() ELSE pdpa_consent_at END,
			updated_at      = NOW()
		WHERE id = $1`
	tag, err := tx.Exec(ctx, snap, accountID, given, version)
	if err != nil {
		return fmt.Errorf("candidateauth: consent snapshot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	const ledger = `
		INSERT INTO pdpa_consents (account_id, consent_given, consent_version, source_channel)
		VALUES ($1, $2, $3, $4)`
	if _, err := tx.Exec(ctx, ledger, accountID, given, version, source); err != nil {
		return fmt.Errorf("candidateauth: consent ledger: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("candidateauth: consent commit: %w", err)
	}
	return nil
}

func (r *pgRepository) CreateSession(ctx context.Context, accountID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	const q = `INSERT INTO candidate_sessions (account_id, token_hash, expires_at) VALUES ($1, $2, $3)`
	if _, err := r.pool.Exec(ctx, q, accountID, tokenHash, expiresAt); err != nil {
		return fmt.Errorf("candidateauth: create session: %w", err)
	}
	return nil
}

func (r *pgRepository) FindAccountBySessionHash(ctx context.Context, tokenHash string) (*Account, error) {
	// Resolve via a subquery (not a JOIN) so accountColumns stays unqualified —
	// candidate_accounts and candidate_sessions share column names (id, created_at)
	// that would be ambiguous in a JOIN's select list.
	// The trailing status filter makes suspension/anonymization revoke an existing
	// cookie too: once status leaves 'active', the live session no longer resolves
	// (treated as logged-out), without having to delete the session row.
	const q = `
		SELECT ` + accountColumns + `
		FROM candidate_accounts
		WHERE id = (
			SELECT account_id FROM candidate_sessions
			WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
		) AND status = 'active'`
	a, err := scanAccount(r.pool.QueryRow(ctx, q, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("candidateauth: find account by session: %w", err)
	}
	return a, nil
}

func (r *pgRepository) RevokeSession(ctx context.Context, tokenHash string) error {
	const q = `UPDATE candidate_sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`
	if _, err := r.pool.Exec(ctx, q, tokenHash); err != nil {
		return fmt.Errorf("candidateauth: revoke session: %w", err)
	}
	return nil
}

func (r *pgRepository) CreateOTP(ctx context.Context, email, codeHash string, expiresAt time.Time) error {
	const q = `INSERT INTO email_otps (email, code_hash, expires_at) VALUES ($1, $2, $3)`
	if _, err := r.pool.Exec(ctx, q, email, codeHash, expiresAt); err != nil {
		return fmt.Errorf("candidateauth: create otp: %w", err)
	}
	return nil
}

// maxOTPAttempts is the per-challenge failed-verify cap. After this many wrong
// guesses the newest live challenge is locked (consumed), so a 6-digit code can be
// guessed at most maxOTPAttempts times before the attacker must request a new code
// (which is IP-rate-limited at /email/start). This is the brute-force throttle.
const maxOTPAttempts = 5

func (r *pgRepository) ConsumeOTP(ctx context.Context, email, codeHash string) error {
	// Lock the newest live challenge for this email and count EVERY verify attempt
	// (success or failure) against it — the previous version only counted matches,
	// so failures were never throttled. Done in a tx with FOR UPDATE to serialise
	// concurrent guesses.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("candidateauth: consume otp begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		id         uuid.UUID
		storedHash string
		attempts   int
	)
	const sel = `
		SELECT id, code_hash, attempts FROM email_otps
		WHERE email = $1 AND consumed_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1
		FOR UPDATE`
	err = tx.QueryRow(ctx, sel, email).Scan(&id, &storedHash, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOTPInvalid // no live challenge (none/expired/already consumed)
	}
	if err != nil {
		return fmt.Errorf("candidateauth: consume otp select: %w", err)
	}

	// Too many failed guesses → lock the challenge out, force a fresh code.
	if attempts >= maxOTPAttempts {
		if _, err := tx.Exec(ctx, `UPDATE email_otps SET consumed_at = NOW() WHERE id = $1`, id); err != nil {
			return fmt.Errorf("candidateauth: consume otp lock: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("candidateauth: consume otp commit: %w", err)
		}
		return ErrOTPInvalid
	}

	match := subtle.ConstantTimeCompare([]byte(storedHash), []byte(codeHash)) == 1
	if match {
		if _, err := tx.Exec(ctx, `UPDATE email_otps SET consumed_at = NOW(), attempts = attempts + 1 WHERE id = $1`, id); err != nil {
			return fmt.Errorf("candidateauth: consume otp success: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("candidateauth: consume otp commit: %w", err)
		}
		return nil
	}

	// Wrong code: count the failed attempt, keep the challenge live until the cap.
	if _, err := tx.Exec(ctx, `UPDATE email_otps SET attempts = attempts + 1 WHERE id = $1`, id); err != nil {
		return fmt.Errorf("candidateauth: consume otp fail: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("candidateauth: consume otp commit: %w", err)
	}
	return ErrOTPInvalid
}

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
