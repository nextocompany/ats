package members

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// uniqueViolation is the Postgres SQLSTATE for a unique-constraint violation.
const uniqueViolation = "23505"

// isUnique reports whether err is a Postgres unique-constraint violation.
func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// ErrNotFound is returned when no member matches the lookup key.
var ErrNotFound = errors.New("members: not found")

// ErrAnonymized is returned when a lifecycle action targets an already-anonymized
// account: erasure is irreversible, so status can't move out of 'anonymized' and
// a redacted account can't be re-anonymized.
var ErrAnonymized = errors.New("members: account already anonymized")

// ErrEmailTaken is returned when an admin profile edit sets an email already held
// by another account (candidate_accounts.email is UNIQUE).
var ErrEmailTaken = errors.New("members: email already in use")

// Repository is the member-admin data-access contract.
type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Member, int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Member, error)
	GetResumeBlobURL(ctx context.Context, id uuid.UUID) (string, error)
	Stats(ctx context.Context) (Stats, error)

	// SetStatus moves a member between 'active' and 'suspended'. Suspending also
	// force-logs-out (deletes the member's sessions). Returns ErrNotFound when the
	// member is missing and ErrAnonymized when it is already erased.
	SetStatus(ctx context.Context, id uuid.UUID, status string, by *uuid.UUID) error
	// ForceLogout deletes the member's sessions (revokes every active login).
	ForceLogout(ctx context.Context, id uuid.UUID) error
	// UpdateProfile applies a sparse admin profile edit.
	UpdateProfile(ctx context.Context, id uuid.UUID, p ProfileUpdate) error
	// Anonymize irreversibly redacts the account's PII, deletes its sessions, and
	// returns the resume blob URL (if any) so the caller can delete it after the
	// commit. Idempotent: a second call returns ErrAnonymized.
	Anonymize(ctx context.Context, id uuid.UUID) (resumeURL string, err error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed member-admin repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

// memberSelect is the shared projection (admin view: provider booleans + derived
// counts, never raw subs/keys). Correlated subqueries are fine at member scale
// (hundreds–thousands); the applications-per-member count is backed by
// idx_candidates_account_id + idx_applications_candidate_id.
const memberSelect = `
	SELECT a.id, a.full_name, COALESCE(a.email,''), COALESCE(a.phone,''), COALESCE(a.province,''),
	       a.email_verified,
	       (a.line_user_id IS NOT NULL)                              AS line_linked,
	       (a.google_sub IS NOT NULL)                                AS google_linked,
	       (a.email IS NOT NULL AND a.email_verified)                AS email_linked,
	       (a.resume_blob_url IS NOT NULL AND a.resume_blob_url <> '') AS has_resume,
	       COALESCE(a.resume_file_type,''),
	       a.status, a.pdpa_consent, COALESCE(a.pdpa_version,''),
	       (SELECT COUNT(*) FROM applications ap JOIN candidates c ON c.id = ap.candidate_id WHERE c.account_id = a.id) AS applications_count,
	       (SELECT COUNT(*) FROM candidate_sessions s WHERE s.account_id = a.id AND s.revoked_at IS NULL AND s.expires_at > NOW()) AS active_sessions,
	       (SELECT MAX(created_at) FROM candidate_sessions s WHERE s.account_id = a.id) AS last_login_at,
	       a.created_at
	FROM candidate_accounts a`

func scanMember(row pgx.Row) (*Member, error) {
	var m Member
	if err := row.Scan(
		&m.ID, &m.FullName, &m.Email, &m.Phone, &m.Province,
		&m.EmailVerified, &m.LineLinked, &m.GoogleLinked, &m.EmailLinked,
		&m.HasResume, &m.ResumeType, &m.Status, &m.PDPAConsent, &m.PDPAVersion,
		&m.AppsCount, &m.ActiveSessions, &m.LastLoginAt, &m.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *pgRepository) List(ctx context.Context, f ListFilter) ([]Member, int, error) {
	f.normalize()

	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	var conds []string
	if f.Search != "" {
		// Escape ILIKE metacharacters so a literal % or _ in the search term isn't
		// treated as a wildcard (over-matching + forces a full scan).
		p := add("%" + escapeLike(f.Search) + "%")
		ilike := ` ILIKE ` + p + ` ESCAPE '\'`
		conds = append(conds, "(a.full_name"+ilike+" OR a.email"+ilike+" OR a.phone"+ilike+")")
	}
	switch f.Provider {
	case "line":
		conds = append(conds, "a.line_user_id IS NOT NULL")
	case "google":
		conds = append(conds, "a.google_sub IS NOT NULL")
	case "email":
		conds = append(conds, "(a.email IS NOT NULL AND a.email_verified)")
	}
	if f.Status != "" {
		conds = append(conds, "a.status = "+add(f.Status))
	}
	if f.HasResume != nil {
		if *f.HasResume {
			conds = append(conds, "(a.resume_blob_url IS NOT NULL AND a.resume_blob_url <> '')")
		} else {
			conds = append(conds, "(a.resume_blob_url IS NULL OR a.resume_blob_url = '')")
		}
	}
	if f.From != nil {
		conds = append(conds, "a.created_at >= "+add(*f.From))
	}
	if f.To != nil {
		conds = append(conds, "a.created_at <= "+add(*f.To))
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM candidate_accounts a"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("members: list count: %w", err)
	}

	limitPH := add(f.Limit)
	offsetPH := add((f.Page - 1) * f.Limit)
	q := memberSelect + where + " ORDER BY a.created_at DESC LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("members: list: %w", err)
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		m, serr := scanMember(rows)
		if serr != nil {
			return nil, 0, fmt.Errorf("members: scan: %w", serr)
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("members: list rows: %w", err)
	}
	return out, total, nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (*Member, error) {
	m, err := scanMember(r.pool.QueryRow(ctx, memberSelect+" WHERE a.id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("members: get by id: %w", err)
	}
	return m, nil
}

func (r *pgRepository) GetResumeBlobURL(ctx context.Context, id uuid.UUID) (string, error) {
	var url string
	err := r.pool.QueryRow(ctx, "SELECT COALESCE(resume_blob_url,'') FROM candidate_accounts WHERE id = $1", id).Scan(&url)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("members: get resume url: %w", err)
	}
	return url, nil
}

func (r *pgRepository) Stats(ctx context.Context) (Stats, error) {
	var s Stats
	// Single point-in-time query so the totals stay self-consistent (a concurrent
	// insert can't make with_applications exceed total).
	const agg = `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'active'),
			COUNT(*) FILTER (WHERE status = 'suspended'),
			COUNT(*) FILTER (WHERE created_at >= NOW() - INTERVAL '7 days'),
			COUNT(*) FILTER (WHERE line_user_id IS NOT NULL),
			COUNT(*) FILTER (WHERE google_sub IS NOT NULL),
			COUNT(*) FILTER (WHERE email IS NOT NULL AND email_verified),
			(SELECT COUNT(DISTINCT ca.id)
			   FROM candidate_accounts ca
			   JOIN candidates c ON c.account_id = ca.id
			   JOIN applications ap ON ap.candidate_id = c.id)
		FROM candidate_accounts`
	var line, google, email int
	if err := r.pool.QueryRow(ctx, agg).Scan(
		&s.Total, &s.Active, &s.Suspended, &s.NewThisWeek, &line, &google, &email, &s.WithApplications,
	); err != nil {
		return Stats{}, fmt.Errorf("members: stats: %w", err)
	}
	s.ByProvider = map[string]int{"line": line, "google": google, "email": email}
	return s, nil
}

// escapeLike escapes ILIKE wildcards so user input matches literally (ESCAPE '\').
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
