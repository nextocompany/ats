package pdpaadmin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/breach"
)

// Repository is the PDPA-console data-access contract. It reads the tables owned
// by other PDPA packages (dsar_requests, pdpa_consents, data_breaches,
// consent_documents) and never mutates subject data directly - the only writes
// are resolving a queued DSAR request.
type Repository interface {
	ListDSAR(ctx context.Context, f DSARListFilter) ([]DSARRequest, int, error)
	// ResolveDSAR moves a pending request to completed/rejected, stamping the
	// resolver + time. reason is stored when non-empty (kept otherwise). Returns
	// ErrBadState if the request is not pending, ErrNotFound if it is missing.
	ResolveDSAR(ctx context.Context, id uuid.UUID, status, reason string, actor *uuid.UUID) (DSARRequest, error)
	LookupConsents(ctx context.Context, accountID, candidateID *uuid.UUID) ([]ConsentRecord, error)
	// Counts returns the live overview numbers (DSAR pending, breaches open/overdue,
	// current consent version). Retention + DPO are merged from config by the handler.
	Counts(ctx context.Context) (pending, open, overdue int, consentVersion string, err error)
}

type pgRepository struct{ pool *pgxpool.Pool }

// NewRepository builds a Postgres-backed PDPA-console repository.
func NewRepository(pool *pgxpool.Pool) Repository { return &pgRepository{pool: pool} }

const dsarColumns = `
	r.id, r.account_id, COALESCE(a.full_name,'') AS account_name, COALESCE(a.email,'') AS account_email,
	r.request_type, r.status, COALESCE(r.reason,'') AS reason, r.requested_at, r.resolved_at, r.resolved_by`

const dsarFrom = `
	FROM dsar_requests r
	LEFT JOIN candidate_accounts a ON a.id = r.account_id`

func scanDSAR(row pgx.Row) (DSARRequest, error) {
	var d DSARRequest
	if err := row.Scan(
		&d.ID, &d.AccountID, &d.AccountName, &d.AccountEmail,
		&d.RequestType, &d.Status, &d.Reason, &d.RequestedAt, &d.ResolvedAt, &d.ResolvedBy,
	); err != nil {
		return DSARRequest{}, err
	}
	return d, nil
}

func (r *pgRepository) ListDSAR(ctx context.Context, f DSARListFilter) ([]DSARRequest, int, error) {
	f.normalize()
	var conds []string
	var args []any
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("r.status = $%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM dsar_requests r"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("pdpaadmin: dsar count: %w", err)
	}

	args = append(args, f.Limit)
	limitPH := fmt.Sprintf("$%d", len(args))
	args = append(args, (f.Page-1)*f.Limit)
	offsetPH := fmt.Sprintf("$%d", len(args))
	q := "SELECT" + dsarColumns + dsarFrom + where +
		" ORDER BY (r.status = 'pending') DESC, r.requested_at DESC LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("pdpaadmin: dsar list: %w", err)
	}
	defer rows.Close()
	var out []DSARRequest
	for rows.Next() {
		d, serr := scanDSAR(rows)
		if serr != nil {
			return nil, 0, fmt.Errorf("pdpaadmin: dsar scan: %w", serr)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("pdpaadmin: dsar rows: %w", err)
	}
	return out, total, nil
}

func (r *pgRepository) ResolveDSAR(ctx context.Context, id uuid.UUID, status, reason string, actor *uuid.UUID) (DSARRequest, error) {
	// Single atomic statement: the CTE updates the row only when pending, then the
	// outer SELECT reads the (possibly unchanged) row plus whether the update
	// applied. This avoids the UPDATE-then-SELECT race where a concurrent resolve or
	// account-cascade delete between the two statements would mis-signal
	// ErrNotFound/ErrBadState. Only a pending request is moved, so a double click is
	// idempotent (the second call sees applied=false → ErrBadState).
	const q = `
		WITH upd AS (
			UPDATE dsar_requests
			SET status = $2, reason = COALESCE(NULLIF($3,''), reason), resolved_at = now(), resolved_by = $4
			WHERE id = $1 AND status = 'pending'
			RETURNING id
		)
		SELECT` + dsarColumns + `, (EXISTS (SELECT 1 FROM upd)) AS applied` + dsarFrom + `
		WHERE r.id = $1`
	var d DSARRequest
	var applied bool
	err := r.pool.QueryRow(ctx, q, id, status, reason, actor).Scan(
		&d.ID, &d.AccountID, &d.AccountName, &d.AccountEmail,
		&d.RequestType, &d.Status, &d.Reason, &d.RequestedAt, &d.ResolvedAt, &d.ResolvedBy, &applied,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DSARRequest{}, ErrNotFound
	}
	if err != nil {
		return DSARRequest{}, fmt.Errorf("pdpaadmin: dsar resolve: %w", err)
	}
	if !applied {
		return DSARRequest{}, ErrBadState // row exists but was not pending
	}
	return d, nil
}

func (r *pgRepository) LookupConsents(ctx context.Context, accountID, candidateID *uuid.UUID) ([]ConsentRecord, error) {
	var conds []string
	var args []any
	if accountID != nil {
		args = append(args, *accountID)
		conds = append(conds, fmt.Sprintf("account_id = $%d", len(args)))
	}
	if candidateID != nil {
		args = append(args, *candidateID)
		conds = append(conds, fmt.Sprintf("candidate_id = $%d", len(args)))
	}
	if len(conds) == 0 {
		// The handler already enforces a selector; fail explicitly so a direct
		// repository caller can never accidentally run an unfiltered table scan.
		return nil, errors.New("pdpaadmin: consent lookup requires account_id or candidate_id")
	}
	q := `SELECT id, candidate_id, account_id, consent_given, COALESCE(consent_version,''), COALESCE(source_channel,''), created_at
		FROM pdpa_consents WHERE ` + strings.Join(conds, " OR ") + ` ORDER BY created_at DESC LIMIT 200`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("pdpaadmin: consent lookup: %w", err)
	}
	defer rows.Close()
	var out []ConsentRecord
	for rows.Next() {
		var c ConsentRecord
		if err := rows.Scan(&c.ID, &c.CandidateID, &c.AccountID, &c.ConsentGiven, &c.Version, &c.Source, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("pdpaadmin: consent scan: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pdpaadmin: consent rows: %w", err)
	}
	return out, nil
}

func (r *pgRepository) Counts(ctx context.Context) (pending, open, overdue int, consentVersion string, err error) {
	const q = `
		SELECT
			(SELECT COUNT(*) FROM dsar_requests WHERE status = 'pending'),
			(SELECT COUNT(*) FROM data_breaches WHERE status <> 'resolved'),
			(SELECT COUNT(*) FROM data_breaches
				WHERE status <> 'resolved' AND pdpc_notified_at IS NULL
				AND now() > discovered_at + (make_interval(hours => $1))),
			COALESCE((SELECT version FROM consent_documents WHERE is_current LIMIT 1), '')`
	hours := int(breach.NotificationWindow.Hours()) // 72; make_interval takes an int
	if err = r.pool.QueryRow(ctx, q, hours).Scan(&pending, &open, &overdue, &consentVersion); err != nil {
		return 0, 0, 0, "", fmt.Errorf("pdpaadmin: counts: %w", err)
	}
	return pending, open, overdue, consentVersion, nil
}
