// Package areas manages the dynamic store groupings used by the area RBAC scope:
// an area is a named set of stores (10-20 typically), and area_hr users are
// assigned to cover one or more areas. Membership on both sides is admin-editable
// (it changes often), which is why this lives in the DB rather than a compiled-in
// map like the legacy subregion enum.
package areas

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when an area id does not exist.
var ErrNotFound = errors.New("area not found")

// Area is a dynamic grouping of stores.
type Area struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Active     bool      `json:"active"`
	StoreCount int       `json:"store_count"`
	StoreNos   []int     `json:"store_nos,omitempty"`
	MemberIDs  []string  `json:"member_ids,omitempty"`
}

// Repository is the area data-access contract.
type Repository interface {
	List(ctx context.Context) ([]Area, error)
	Get(ctx context.Context, id uuid.UUID) (Area, error)
	Create(ctx context.Context, name string) (Area, error)
	Update(ctx context.Context, id uuid.UUID, name *string, active *bool) (Area, error)
	Delete(ctx context.Context, id uuid.UUID) error
	SetStores(ctx context.Context, id uuid.UUID, storeNos []int) error
	SetMembers(ctx context.Context, id uuid.UUID, userIDs []uuid.UUID) error
}

type pgRepository struct{ pool *pgxpool.Pool }

// NewRepository builds a Postgres-backed area repository.
func NewRepository(pool *pgxpool.Pool) Repository { return &pgRepository{pool: pool} }

func (r *pgRepository) List(ctx context.Context) ([]Area, error) {
	const q = `
		SELECT a.id, a.name, a.active, COALESCE(cnt.n, 0)
		FROM areas a
		LEFT JOIN (SELECT area_id, COUNT(*) n FROM area_stores GROUP BY area_id) cnt ON cnt.area_id = a.id
		ORDER BY a.name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("areas: list: %w", err)
	}
	defer rows.Close()
	var out []Area
	for rows.Next() {
		var a Area
		if err := rows.Scan(&a.ID, &a.Name, &a.Active, &a.StoreCount); err != nil {
			return nil, fmt.Errorf("areas: scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *pgRepository) Get(ctx context.Context, id uuid.UUID) (Area, error) {
	var a Area
	err := r.pool.QueryRow(ctx, `SELECT id, name, active FROM areas WHERE id = $1`, id).Scan(&a.ID, &a.Name, &a.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Area{}, ErrNotFound
	}
	if err != nil {
		return Area{}, fmt.Errorf("areas: get: %w", err)
	}
	// Stores.
	srows, err := r.pool.Query(ctx, `SELECT store_no FROM area_stores WHERE area_id = $1 ORDER BY store_no`, id)
	if err != nil {
		return Area{}, fmt.Errorf("areas: get stores: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		var n int
		if err := srows.Scan(&n); err != nil {
			return Area{}, fmt.Errorf("areas: scan store: %w", err)
		}
		a.StoreNos = append(a.StoreNos, n)
	}
	a.StoreCount = len(a.StoreNos)
	// Members.
	mrows, err := r.pool.Query(ctx, `SELECT user_id FROM user_areas WHERE area_id = $1`, id)
	if err != nil {
		return Area{}, fmt.Errorf("areas: get members: %w", err)
	}
	defer mrows.Close()
	for mrows.Next() {
		var uid uuid.UUID
		if err := mrows.Scan(&uid); err != nil {
			return Area{}, fmt.Errorf("areas: scan member: %w", err)
		}
		a.MemberIDs = append(a.MemberIDs, uid.String())
	}
	return a, nil
}

func (r *pgRepository) Create(ctx context.Context, name string) (Area, error) {
	var a Area
	err := r.pool.QueryRow(ctx, `INSERT INTO areas (name) VALUES ($1) RETURNING id, name, active`, name).
		Scan(&a.ID, &a.Name, &a.Active)
	if err != nil {
		return Area{}, fmt.Errorf("areas: create: %w", err)
	}
	return a, nil
}

func (r *pgRepository) Update(ctx context.Context, id uuid.UUID, name *string, active *bool) (Area, error) {
	const q = `
		UPDATE areas
		SET name = COALESCE($2, name), active = COALESCE($3, active), updated_at = now()
		WHERE id = $1
		RETURNING id, name, active`
	var a Area
	err := r.pool.QueryRow(ctx, q, id, name, active).Scan(&a.ID, &a.Name, &a.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Area{}, ErrNotFound
	}
	if err != nil {
		return Area{}, fmt.Errorf("areas: update: %w", err)
	}
	return a, nil
}

func (r *pgRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM areas WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("areas: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStores replaces an area's store membership in one transaction.
func (r *pgRepository) SetStores(ctx context.Context, id uuid.UUID, storeNos []int) error {
	return r.replace(ctx, "area_stores", "store_no", id, anyInts(storeNos))
}

// SetMembers replaces the set of users covering an area in one transaction.
func (r *pgRepository) SetMembers(ctx context.Context, id uuid.UUID, userIDs []uuid.UUID) error {
	return r.replace(ctx, "user_areas", "user_id", id, anyUUIDs(userIDs))
}

// replace clears an area's rows in a junction table and inserts the new set
// atomically. valCol is the second column (store_no / user_id).
func (r *pgRepository) replace(ctx context.Context, table, valCol string, areaID uuid.UUID, vals []any) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("areas: begin: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE area_id = $1", table), areaID); err != nil {
		return fmt.Errorf("areas: clear %s: %w", table, err)
	}
	ins := fmt.Sprintf("INSERT INTO %s (area_id, %s) VALUES ($1, $2) ON CONFLICT DO NOTHING", table, valCol)
	for _, v := range vals {
		if _, err := tx.Exec(ctx, ins, areaID, v); err != nil {
			return fmt.Errorf("areas: insert %s: %w", table, err)
		}
	}
	return tx.Commit(ctx)
}

func anyInts(in []int) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

func anyUUIDs(in []uuid.UUID) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
