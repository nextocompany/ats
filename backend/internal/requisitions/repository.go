package requisitions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
)

// reqColumns is the projection shared by every single-row read. The position and
// store labels are joined so the UI never needs a second lookup.
const reqColumns = `
	v.id, v.position_id, COALESCE(NULLIF(p.title_en,''), p.title_th, '') AS position_title,
	v.store_id, COALESCE(s.store_name,'') AS store_name, COALESCE(s.subregion,'') AS subregion,
	v.headcount, v.status, v.source, v.created_by, v.approved_by, v.approved_at, v.created_at, v.updated_at,
	COALESCE(v.responsibilities,''), COALESCE(v.qualifications,''), COALESCE(v.benefits,''), COALESCE(v.other_details,''),
	COALESCE(v.employment_type,''), v.salary_min, v.salary_max, COALESCE(v.priority,''), COALESCE(v.open_reason,'')`

const reqFrom = `
	FROM vacancies v
	LEFT JOIN positions p ON p.id = v.position_id
	LEFT JOIN stores s ON s.store_no = v.store_id`

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed requisition repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func scanRequisition(row pgx.Row) (Requisition, error) {
	var r Requisition
	if err := row.Scan(
		&r.ID, &r.PositionID, &r.PositionTitle,
		&r.StoreID, &r.StoreName, &r.Subregion,
		&r.Headcount, &r.Status, &r.Source, &r.CreatedBy, &r.ApprovedBy, &r.ApprovedAt, &r.CreatedAt, &r.UpdatedAt,
		&r.Responsibilities, &r.Qualifications, &r.Benefits, &r.OtherDetails,
		&r.EmploymentType, &r.SalaryMin, &r.SalaryMax, &r.Priority, &r.OpenReason,
	); err != nil {
		return Requisition{}, err
	}
	return r, nil
}

func (r *pgRepository) List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Requisition, int, error) {
	f.normalize()

	// Build the WHERE incrementally with $-numbered placeholders.
	var conds []string
	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.Status != "" {
		conds = append(conds, "v.status = "+add(f.Status))
	}
	if f.StoreID != nil {
		conds = append(conds, "v.store_id = "+add(*f.StoreID))
	}
	if f.PositionID != nil {
		conds = append(conds, "v.position_id = "+add(*f.PositionID))
	}
	if clause, sargs := scope.VacanciesClause(len(args) + 1); clause != "" {
		// VacanciesClause numbers from len(args)+1 and may reference store_id only.
		conds = append(conds, clause)
		args = append(args, sargs...)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM vacancies v"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("requisitions: list count: %w", err)
	}

	limitPH := add(f.Limit)
	offsetPH := add((f.Page - 1) * f.Limit)
	q := "SELECT" + reqColumns + reqFrom + where + " ORDER BY v.created_at DESC, v.id LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("requisitions: list: %w", err)
	}
	defer rows.Close()
	var out []Requisition
	for rows.Next() {
		req, serr := scanRequisition(rows)
		if serr != nil {
			return nil, 0, fmt.Errorf("requisitions: scan: %w", serr)
		}
		out = append(out, req)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("requisitions: list rows: %w", err)
	}
	return out, total, nil
}

func (r *pgRepository) Create(ctx context.Context, in CreateInput, createdBy uuid.UUID) (Requisition, error) {
	const ins = `
		INSERT INTO vacancies (
			position_id, store_id, headcount, status, source, created_by, opened_at, created_at, updated_at,
			responsibilities, qualifications, benefits, other_details,
			employment_type, salary_min, salary_max, priority, open_reason)
		VALUES ($1, $2, $3, $4, $5, $6, NULL, now(), now(),
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15)
		RETURNING id`
	var id uuid.UUID
	if err := r.pool.QueryRow(ctx, ins,
		in.PositionID, in.StoreID, in.Headcount, StatusPendingApproval, SourceManual, createdBy,
		in.Responsibilities, in.Qualifications, in.Benefits, in.OtherDetails,
		in.EmploymentType, in.SalaryMin, in.SalaryMax, in.Priority, in.OpenReason,
	).Scan(&id); err != nil {
		return Requisition{}, fmt.Errorf("requisitions: create: %w", err)
	}
	return r.getByID(ctx, id)
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Requisition, error) {
	var set []string
	var args []any
	add := func(expr string, val any) {
		args = append(args, val)
		set = append(set, fmt.Sprintf("%s = $%d", expr, len(args)))
	}
	if in.PositionID != nil {
		add("position_id", *in.PositionID)
	}
	if in.StoreID != nil {
		add("store_id", *in.StoreID)
	}
	if in.Headcount != nil {
		add("headcount", *in.Headcount)
	}
	if in.Responsibilities != nil {
		add("responsibilities", *in.Responsibilities)
	}
	if in.Qualifications != nil {
		add("qualifications", *in.Qualifications)
	}
	if in.Benefits != nil {
		add("benefits", *in.Benefits)
	}
	if in.OtherDetails != nil {
		add("other_details", *in.OtherDetails)
	}
	if in.EmploymentType != nil {
		add("employment_type", *in.EmploymentType)
	}
	if in.SalaryMin != nil {
		add("salary_min", *in.SalaryMin)
	}
	if in.SalaryMax != nil {
		add("salary_max", *in.SalaryMax)
	}
	if in.Priority != nil {
		add("priority", *in.Priority)
	}
	if in.OpenReason != nil {
		add("open_reason", *in.OpenReason)
	}
	if len(set) == 0 {
		return r.getByID(ctx, id) // nothing to change
	}
	set = append(set, "updated_at = now()")
	args = append(args, id)
	// Manual requisitions are editable while pending OR open (HR can adjust a live
	// opening without re-approval); PS rows are immutable here and a closed/cancelled
	// requisition is locked as a historical record.
	q := fmt.Sprintf(`UPDATE vacancies SET %s WHERE id = $%d AND source = '%s' AND status IN ('%s','%s')`,
		strings.Join(set, ", "), len(args), SourceManual, StatusPendingApproval, StatusOpen)
	ct, err := r.pool.Exec(ctx, q, args...)
	if err != nil {
		return Requisition{}, fmt.Errorf("requisitions: update: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return Requisition{}, ErrBadState
	}
	return r.getByID(ctx, id)
}

func (r *pgRepository) Approve(ctx context.Context, id uuid.UUID, approver uuid.UUID) (Requisition, error) {
	const q = `
		UPDATE vacancies
		SET status = $2, approved_by = $3, approved_at = now(), opened_at = now(), updated_at = now()
		WHERE id = $1 AND source = $4 AND status = $5`
	ct, err := r.pool.Exec(ctx, q, id, StatusOpen, approver, SourceManual, StatusPendingApproval)
	if err != nil {
		return Requisition{}, fmt.Errorf("requisitions: approve: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return Requisition{}, ErrBadState
	}
	return r.getByID(ctx, id)
}

func (r *pgRepository) Close(ctx context.Context, id uuid.UUID) (Requisition, error) {
	const q = `
		UPDATE vacancies SET status = $2, updated_at = now()
		WHERE id = $1 AND source = $3 AND status = $4`
	ct, err := r.pool.Exec(ctx, q, id, StatusClosed, SourceManual, StatusOpen)
	if err != nil {
		return Requisition{}, fmt.Errorf("requisitions: close: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return Requisition{}, ErrBadState
	}
	return r.getByID(ctx, id)
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Only a manual, still-pending requisition can be hard-deleted; open ones must
	// be closed (auditable), PS rows are off-limits.
	const q = `DELETE FROM vacancies WHERE id = $1 AND source = $2 AND status = $3`
	ct, err := r.pool.Exec(ctx, q, id, SourceManual, StatusPendingApproval)
	if err != nil {
		return fmt.Errorf("requisitions: delete: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrBadState
	}
	return nil
}

func (r *pgRepository) ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error) {
	q := "SELECT EXISTS(SELECT 1 FROM vacancies WHERE id = $1"
	args := []any{id}
	if clause, sargs := scope.VacanciesClause(2); clause != "" {
		q += " AND " + clause
		args = append(args, sargs...)
	}
	q += ")"
	var ok bool
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&ok); err != nil {
		return false, fmt.Errorf("requisitions: exists in scope: %w", err)
	}
	return ok, nil
}

func (r *pgRepository) getByID(ctx context.Context, id uuid.UUID) (Requisition, error) {
	row := r.pool.QueryRow(ctx, "SELECT"+reqColumns+reqFrom+" WHERE v.id = $1", id)
	req, err := scanRequisition(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Requisition{}, ErrNotFound
		}
		return Requisition{}, fmt.Errorf("requisitions: get by id: %w", err)
	}
	return req, nil
}
