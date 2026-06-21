package breach

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// breachColumns is the projection shared by every read.
const breachColumns = `
	id, title, description, severity, status, affected_subjects, data_categories,
	discovered_at, occurred_at, high_risk, pdpc_notified_at, subjects_notified_at,
	remediation, created_by, resolved_by, resolved_at, created_at, updated_at`

// Repository is the breach-register data-access contract.
type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Breach, int, error)
	Create(ctx context.Context, in CreateInput, createdBy uuid.UUID) (Breach, error)
	GetByID(ctx context.Context, id uuid.UUID) (Breach, error)
	Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Breach, error)
	// MarkPDPCNotified stamps pdpc_notified_at = now() (idempotent: a no-op if
	// already set), discharging the 72h obligation.
	MarkPDPCNotified(ctx context.Context, id uuid.UUID) (Breach, error)
	// MarkSubjectsNotified stamps subjects_notified_at = now() (idempotent).
	MarkSubjectsNotified(ctx context.Context, id uuid.UUID) (Breach, error)
	// Resolve closes the incident (status=resolved + resolver + resolved_at).
	Resolve(ctx context.Context, id uuid.UUID, resolver uuid.UUID) (Breach, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type pgRepository struct {
	pool *pgxpool.Pool
	// now is injectable so the derived 72h Deadline is deterministic in tests.
	now func() time.Time
}

// NewRepository builds a Postgres-backed breach repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool, now: time.Now}
}

func scanBreach(row pgx.Row, now time.Time) (Breach, error) {
	var b Breach
	if err := row.Scan(
		&b.ID, &b.Title, &b.Description, &b.Severity, &b.Status, &b.AffectedSubjects, &b.DataCategories,
		&b.DiscoveredAt, &b.OccurredAt, &b.HighRisk, &b.PDPCNotifiedAt, &b.SubjectsNotifiedAt,
		&b.Remediation, &b.CreatedBy, &b.ResolvedBy, &b.ResolvedAt, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return Breach{}, err
	}
	b.Deadline = computeDeadline(b.DiscoveredAt, b.PDPCNotifiedAt, now)
	return b, nil
}

func (r *pgRepository) List(ctx context.Context, f ListFilter) ([]Breach, int, error) {
	f.normalize()

	var conds []string
	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.Status != "" {
		conds = append(conds, "status = "+add(f.Status))
	}
	if f.Severity != "" {
		conds = append(conds, "severity = "+add(f.Severity))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM data_breaches"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("breach: list count: %w", err)
	}

	limitPH := add(f.Limit)
	offsetPH := add((f.Page - 1) * f.Limit)
	q := "SELECT" + breachColumns + " FROM data_breaches" + where +
		" ORDER BY discovered_at DESC, id LIMIT " + limitPH + " OFFSET " + offsetPH

	now := r.now()
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("breach: list: %w", err)
	}
	defer rows.Close()
	var out []Breach
	for rows.Next() {
		b, serr := scanBreach(rows, now)
		if serr != nil {
			return nil, 0, fmt.Errorf("breach: scan: %w", serr)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("breach: list rows: %w", err)
	}
	return out, total, nil
}

func (r *pgRepository) Create(ctx context.Context, in CreateInput, createdBy uuid.UUID) (Breach, error) {
	const ins = `
		INSERT INTO data_breaches
			(title, description, severity, affected_subjects, data_categories,
			 discovered_at, occurred_at, high_risk, remediation, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, ins,
		in.Title, in.Description, in.Severity, in.AffectedSubjects, in.DataCategories,
		in.DiscoveredAt, in.OccurredAt, in.HighRisk, in.Remediation, createdBy,
	).Scan(&id); err != nil {
		return Breach{}, fmt.Errorf("breach: create: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Breach, error) {
	var set []string
	var args []any
	add := func(col string, val any) {
		args = append(args, val)
		set = append(set, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if in.Title != nil {
		add("title", *in.Title)
	}
	if in.Description != nil {
		add("description", *in.Description)
	}
	if in.Severity != nil {
		add("severity", *in.Severity)
	}
	if in.Status != nil {
		add("status", *in.Status)
	}
	if in.AffectedSubjects != nil {
		add("affected_subjects", *in.AffectedSubjects)
	}
	if in.DataCategories != nil {
		add("data_categories", *in.DataCategories)
	}
	if in.OccurredAt != nil {
		add("occurred_at", *in.OccurredAt)
	}
	if in.HighRisk != nil {
		add("high_risk", *in.HighRisk)
	}
	if in.Remediation != nil {
		add("remediation", *in.Remediation)
	}
	if len(set) == 0 {
		return r.GetByID(ctx, id) // nothing to change
	}
	set = append(set, "updated_at = now()")
	args = append(args, id)
	q := fmt.Sprintf(`UPDATE data_breaches SET %s WHERE id = $%d`, strings.Join(set, ", "), len(args))
	ct, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return Breach{}, fmt.Errorf("breach: update: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return Breach{}, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

// The notify stamps use COALESCE so a re-notify is idempotent: the first recorded
// time is preserved and never moved. The column names are baked into the constant
// queries (never interpolated) so the statements can carry no caller-supplied SQL.
const (
	stampPDPCQ     = `UPDATE data_breaches SET pdpc_notified_at = COALESCE(pdpc_notified_at, now()), updated_at = now() WHERE id = $1`
	stampSubjectsQ = `UPDATE data_breaches SET subjects_notified_at = COALESCE(subjects_notified_at, now()), updated_at = now() WHERE id = $1`
)

func (r *pgRepository) MarkPDPCNotified(ctx context.Context, id uuid.UUID) (Breach, error) {
	return r.stamp(ctx, id, stampPDPCQ)
}

func (r *pgRepository) MarkSubjectsNotified(ctx context.Context, id uuid.UUID) (Breach, error) {
	return r.stamp(ctx, id, stampSubjectsQ)
}

// stamp runs one of the fixed notify-stamp queries above. The row must exist;
// RowsAffected is 0 only for a missing row (an already-stamped value still
// matches and returns 1 via the COALESCE no-op).
func (r *pgRepository) stamp(ctx context.Context, id uuid.UUID, q string) (Breach, error) {
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return Breach{}, fmt.Errorf("breach: stamp notify: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return Breach{}, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *pgRepository) Resolve(ctx context.Context, id uuid.UUID, resolver uuid.UUID) (Breach, error) {
	// COALESCE both resolver fields so a second resolve is idempotent: it never
	// overwrites who first resolved the incident or when.
	const q = `
		UPDATE data_breaches
		SET status = $2, resolved_by = COALESCE(resolved_by, $3), resolved_at = COALESCE(resolved_at, now()), updated_at = now()
		WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id, StatusResolved, resolver)
	if err != nil {
		return Breach{}, fmt.Errorf("breach: resolve: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return Breach{}, ErrNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// A breach becomes part of the regulatory record once the PDPC or affected
	// subjects have been notified; from then it must not be hard-deleted (it can
	// only be resolved). Only an un-notified draft (e.g. a mistaken entry) may be
	// removed. RowsAffected=0 means either the row is gone or it is notified, so
	// disambiguate with a follow-up read.
	const q = `DELETE FROM data_breaches WHERE id = $1 AND pdpc_notified_at IS NULL AND subjects_notified_at IS NULL`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("breach: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		if _, gerr := r.GetByID(ctx, id); gerr != nil {
			return gerr // ErrNotFound (or a real error)
		}
		return ErrBadState // exists but already notified → protected
	}
	return nil
}

func (r *pgRepository) GetByID(ctx context.Context, id uuid.UUID) (Breach, error) {
	row := r.pool.QueryRow(ctx, "SELECT"+breachColumns+" FROM data_breaches WHERE id = $1", id)
	b, err := scanBreach(row, r.now())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Breach{}, ErrNotFound
		}
		return Breach{}, fmt.Errorf("breach: get by id: %w", err)
	}
	return b, nil
}
