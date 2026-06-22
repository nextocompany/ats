package candidates

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/activity"
	"github.com/nexto/hr-ats/internal/rbac"
)

// Pagination bounds.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Filter is the candidate list query.
type Filter struct {
	Status string
	Page   int
	Limit  int
}

func (f *Filter) normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
}

// List returns a filtered, role-scoped, paginated page of candidates + total.
func (r *pgRepository) List(ctx context.Context, f Filter, scope rbac.Scope) ([]Candidate, int, error) {
	f.normalize()

	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	// Exclude deduplicated candidates: once a CV is merged into a canonical
	// candidate, its row is orphaned (is_duplicate_of set, no applications) and
	// must never appear in the HR roster - clicking one opens a ghost profile.
	// Mirrors the search projection's filter (internal/search/docs.go).
	conds := []string{"is_duplicate_of IS NULL"}
	if f.Status != "" {
		conds = append(conds, "status = "+add(f.Status))
	}
	if sc, scArgs := scope.CandidatesClause(len(args) + 1); sc != "" {
		conds = append(conds, sc)
		args = append(args, scArgs...)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM candidates "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("candidates: list count: %w", err)
	}

	limitPH := add(f.Limit)
	offsetPH := add((f.Page - 1) * f.Limit)
	q := `SELECT id, full_name, COALESCE(phone,''), COALESCE(email,''), COALESCE(id_card,''),
	             COALESCE(province,''), COALESCE(subregion,''), COALESCE(source_channel,''), status, created_at
	      FROM candidates ` + where + " ORDER BY created_at DESC LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("candidates: list: %w", err)
	}
	defer rows.Close()

	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.ID, &c.FullName, &c.Phone, &c.Email, &c.IDCard,
			&c.Province, &c.Subregion, &c.SourceChannel, &c.Status, &c.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("candidates: list scan: %w", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// Timeline returns audit entries for the candidate and all of its applications,
// newest first (F16 history).
func (r *pgRepository) Timeline(ctx context.Context, id uuid.UUID) ([]activity.Entry, error) {
	const q = `
		SELECT action, entity_type, entity_id, new_value, created_at
		FROM activity_logs
		WHERE entity_id = $1
		   OR entity_id IN (SELECT id FROM applications WHERE candidate_id = $1)
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, id)
	if err != nil {
		return nil, fmt.Errorf("candidates: timeline: %w", err)
	}
	defer rows.Close()

	var out []activity.Entry
	for rows.Next() {
		var e activity.Entry
		if err := rows.Scan(&e.Action, &e.EntityType, &e.EntityID, &e.NewValue, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("candidates: timeline scan: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
