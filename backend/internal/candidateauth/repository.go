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
	SetConsent(ctx context.Context, accountID uuid.UUID, version string) error

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
		return fmt.Errorf("candidateauth: link line: %w (LINE already linked to another account)", err)
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

func (r *pgRepository) SetConsent(ctx context.Context, accountID uuid.UUID, version string) error {
	const q = `UPDATE candidate_accounts SET pdpa_consent = TRUE, pdpa_version = $2, pdpa_consent_at = NOW(), updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, accountID, version); err != nil {
		return fmt.Errorf("candidateauth: set consent: %w", err)
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
