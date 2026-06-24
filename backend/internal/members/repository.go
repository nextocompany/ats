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

	"github.com/nexto/hr-ats/internal/rbac"
)

// Postgres SQLSTATEs we branch on.
const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
)

// isUnique reports whether err is a Postgres unique-constraint violation.
func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// isForeignKey reports whether err is a Postgres FK-constraint violation (e.g. a
// note/tag insert referencing a member that doesn't exist).
func isForeignKey(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolation
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
	// List/Stats/ListForExport are scoped by the caller's rbac.Scope: a
	// store/subregion role sees only accounts owning a linked candidate in its
	// scope (KindAll sees every account, incl. 0-application ones).
	List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Member, int, error)
	// ListForExport returns up to max rows matching the filter (no pagination,
	// newest-first) for the CSV export, scoped like List.
	ListForExport(ctx context.Context, f ListFilter, scope rbac.Scope, max int) ([]Member, error)
	// GetByID returns one account unscoped (admin CRM/lifecycle paths).
	GetByID(ctx context.Context, id uuid.UUID) (*Member, error)
	// GetScopedByID returns one account only when it is visible in the caller's
	// scope, else ErrNotFound (so a scoped role can't read an out-of-scope person
	// by guessing an id).
	GetScopedByID(ctx context.Context, id uuid.UUID, scope rbac.Scope) (*Member, error)
	// ResolveAccountID maps a per-intake candidate id to its owning account id
	// (ErrNotFound when the candidate is missing or has no linked account). Lets
	// the unified detail accept either an account id or a candidate id.
	ResolveAccountID(ctx context.Context, candidateID uuid.UUID) (uuid.UUID, error)
	// ListApplicationsByAccount returns the account's applications across every
	// linked candidate row (per position + funnel status), newest first.
	ListApplicationsByAccount(ctx context.Context, accountID uuid.UUID) ([]AccountApplication, error)
	GetResumeBlobURL(ctx context.Context, id uuid.UUID) (string, error)
	Stats(ctx context.Context, scope rbac.Scope) (Stats, error)

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

	// AddNote appends an HR note; ListNotes returns a member's notes newest-first.
	// Both return ErrNotFound when the member is missing.
	AddNote(ctx context.Context, id uuid.UUID, author, body string) (*Note, error)
	ListNotes(ctx context.Context, id uuid.UUID) ([]Note, error)
	// AddTag attaches a tag (idempotent); RemoveTag detaches it; ListTags returns
	// a member's tags sorted. AddTag returns ErrNotFound for a missing member.
	AddTag(ctx context.Context, id uuid.UUID, tag string) error
	RemoveTag(ctx context.Context, id uuid.UUID, tag string) error
	ListTags(ctx context.Context, id uuid.UUID) ([]string, error)
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
	       a.created_at,
	       (SELECT c.id FROM candidates c WHERE c.account_id = a.id AND c.is_duplicate_of IS NULL ORDER BY c.created_at LIMIT 1) AS candidate_id
	FROM candidate_accounts a`

func scanMember(row pgx.Row) (*Member, error) {
	var m Member
	if err := row.Scan(
		&m.ID, &m.FullName, &m.Email, &m.Phone, &m.Province,
		&m.EmailVerified, &m.LineLinked, &m.GoogleLinked, &m.EmailLinked,
		&m.HasResume, &m.ResumeType, &m.Status, &m.PDPAConsent, &m.PDPAVersion,
		&m.AppsCount, &m.ActiveSessions, &m.LastLoginAt, &m.CreatedAt, &m.CandidateID,
	); err != nil {
		return nil, err
	}
	return &m, nil
}

// buildMemberWhere turns a ListFilter + caller scope into a parameterised WHERE
// clause + args. Shared by List (paginated) and ListForExport (capped, no offset)
// so the filter + scope semantics can't drift between the directory and its CSV
// export. Callers continue the placeholder numbering from len(args).
func buildMemberWhere(f ListFilter, scope rbac.Scope) (string, []any) {
	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	var conds []string
	// Role scoping first (correlated EXISTS on the linked candidates of alias "a").
	if sc, scArgs := scope.AccountsClause("a", len(args)+1); sc != "" {
		conds = append(conds, sc)
		args = append(args, scArgs...)
	}
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
	if f.Tag != "" {
		conds = append(conds, "EXISTS (SELECT 1 FROM member_tags t WHERE t.account_id = a.id AND t.tag = "+add(f.Tag)+")")
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
	return where, args
}

func (r *pgRepository) List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Member, int, error) {
	f.normalize()
	where, args := buildMemberWhere(f, scope)

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM candidate_accounts a"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("members: list count: %w", err)
	}

	nextPH := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	limitPH := nextPH(f.Limit)
	offsetPH := nextPH((f.Page - 1) * f.Limit)
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

func (r *pgRepository) ListForExport(ctx context.Context, f ListFilter, scope rbac.Scope, max int) ([]Member, error) {
	f.normalize() // parity with List, even though pagination fields are unused here
	where, args := buildMemberWhere(f, scope)
	args = append(args, max)
	q := fmt.Sprintf("%s%s ORDER BY a.created_at DESC LIMIT $%d", memberSelect, where, len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("members: export query: %w", err)
	}
	defer rows.Close()

	var out []Member
	for rows.Next() {
		m, serr := scanMember(rows)
		if serr != nil {
			return nil, fmt.Errorf("members: export scan: %w", serr)
		}
		out = append(out, *m)
	}
	return out, rows.Err()
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

func (r *pgRepository) GetScopedByID(ctx context.Context, id uuid.UUID, scope rbac.Scope) (*Member, error) {
	q := memberSelect + " WHERE a.id = $1"
	args := []any{id}
	if sc, scArgs := scope.AccountsClause("a", len(args)+1); sc != "" {
		q += " AND " + sc
		args = append(args, scArgs...)
	}
	m, err := scanMember(r.pool.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound // missing OR out of the caller's scope (don't leak which)
	}
	if err != nil {
		return nil, fmt.Errorf("members: get scoped by id: %w", err)
	}
	return m, nil
}

func (r *pgRepository) ResolveAccountID(ctx context.Context, candidateID uuid.UUID) (uuid.UUID, error) {
	var acct *uuid.UUID
	err := r.pool.QueryRow(ctx, "SELECT account_id FROM candidates WHERE id = $1", candidateID).Scan(&acct)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("members: resolve account id: %w", err)
	}
	if acct == nil {
		return uuid.Nil, ErrNotFound // accountless candidate (e.g. legacy walk-in, no email)
	}
	return *acct, nil
}

func (r *pgRepository) ListApplicationsByAccount(ctx context.Context, accountID uuid.UUID) ([]AccountApplication, error) {
	const q = `
		SELECT ap.id, ap.position_id,
		       COALESCE(NULLIF(p.title_en,''), p.title_th, '') AS position_title,
		       ap.status, ap.ai_score, ap.created_at
		FROM applications ap
		JOIN candidates c ON c.id = ap.candidate_id
		LEFT JOIN positions p ON p.id = ap.position_id
		WHERE c.account_id = $1
		ORDER BY ap.created_at DESC`
	rows, err := r.pool.Query(ctx, q, accountID)
	if err != nil {
		return nil, fmt.Errorf("members: list applications by account: %w", err)
	}
	defer rows.Close()

	var out []AccountApplication
	for rows.Next() {
		var a AccountApplication
		if err := rows.Scan(&a.ID, &a.PositionID, &a.PositionTitle, &a.Status, &a.AIScore, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("members: scan account application: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
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

func (r *pgRepository) Stats(ctx context.Context, scope rbac.Scope) (Stats, error) {
	var s Stats
	// Single point-in-time query so the totals stay self-consistent (a concurrent
	// insert can't make with_applications exceed total). Scoped via the same
	// correlated EXISTS as the list so a store role's stats match its visible rows.
	var args []any
	where := ""
	if sc, scArgs := scope.AccountsClause("a", 1); sc != "" {
		where = " WHERE " + sc
		args = scArgs
	}
	agg := `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE a.status = 'active'),
			COUNT(*) FILTER (WHERE a.status = 'suspended'),
			COUNT(*) FILTER (WHERE a.created_at >= NOW() - INTERVAL '7 days'),
			COUNT(*) FILTER (WHERE a.line_user_id IS NOT NULL),
			COUNT(*) FILTER (WHERE a.google_sub IS NOT NULL),
			COUNT(*) FILTER (WHERE a.email IS NOT NULL AND a.email_verified),
			COUNT(*) FILTER (WHERE EXISTS (
				SELECT 1 FROM candidates c JOIN applications ap ON ap.candidate_id = c.id
				WHERE c.account_id = a.id))
		FROM candidate_accounts a` + where
	var line, google, email int
	if err := r.pool.QueryRow(ctx, agg, args...).Scan(
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
