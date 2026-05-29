// Package vacancies owns vacancy lookups for branch assignment. In Sprint 2 the
// data is seeded; PeopleSoft sync (Sprint 3) becomes the real source.
package vacancies

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenVacancy is an open vacancy joined with its store's location/format.
type OpenVacancy struct {
	StoreNo    int
	StoreName  string
	FormatType string
	Subregion  string
	StoreLat   *float64
	StoreLng   *float64
	Headcount  int
}

// Vacancy is the upsert view used by the PeopleSoft sync (Direction A).
type Vacancy struct {
	PSVacancyID string
	StoreID     *int
	PositionID  *uuid.UUID
	Headcount   int
	Status      string
	OpenedAt    time.Time
}

// Repository is the vacancy data-access contract.
type Repository interface {
	// FindOpen returns open vacancies for a position within a subregion.
	FindOpen(ctx context.Context, subregion string, positionID uuid.UUID) ([]OpenVacancy, error)
	// CountOpenForPosition reports how many open vacancies exist anywhere.
	CountOpenForPosition(ctx context.Context, positionID uuid.UUID) (int, error)
	// Upsert creates or updates a vacancy keyed by ps_vacancy_id (idempotent on
	// PeopleSoft redelivery).
	Upsert(ctx context.Context, v Vacancy) error
	// SetStatusByPSID updates a vacancy's status by its PeopleSoft id.
	SetStatusByPSID(ctx context.Context, psVacancyID, status string) error
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed vacancy repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) FindOpen(ctx context.Context, subregion string, positionID uuid.UUID) ([]OpenVacancy, error) {
	const q = `
		SELECT s.store_no, s.store_name, COALESCE(s.format_type,''), COALESCE(s.subregion,''),
		       s.latitude, s.longitude, v.headcount
		FROM vacancies v
		JOIN stores s ON s.store_no = v.store_id
		WHERE v.status = 'open' AND v.position_id = $1 AND s.subregion = $2`
	rows, err := r.pool.Query(ctx, q, positionID, subregion)
	if err != nil {
		return nil, fmt.Errorf("vacancies: find open: %w", err)
	}
	defer rows.Close()

	var out []OpenVacancy
	for rows.Next() {
		var v OpenVacancy
		if err := rows.Scan(&v.StoreNo, &v.StoreName, &v.FormatType, &v.Subregion, &v.StoreLat, &v.StoreLng, &v.Headcount); err != nil {
			return nil, fmt.Errorf("vacancies: scan: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vacancies: rows: %w", err)
	}
	return out, nil
}

func (r *pgRepository) CountOpenForPosition(ctx context.Context, positionID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(*) FROM vacancies WHERE status = 'open' AND position_id = $1`
	var n int
	if err := r.pool.QueryRow(ctx, q, positionID).Scan(&n); err != nil {
		return 0, fmt.Errorf("vacancies: count open: %w", err)
	}
	return n, nil
}

func (r *pgRepository) Upsert(ctx context.Context, v Vacancy) error {
	const q = `
		INSERT INTO vacancies (ps_vacancy_id, store_id, position_id, headcount, status, opened_at, ps_synced_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (ps_vacancy_id) DO UPDATE SET
			store_id     = EXCLUDED.store_id,
			position_id  = EXCLUDED.position_id,
			headcount    = EXCLUDED.headcount,
			status       = EXCLUDED.status,
			opened_at    = EXCLUDED.opened_at,
			ps_synced_at = NOW()`
	if _, err := r.pool.Exec(ctx, q, v.PSVacancyID, v.StoreID, v.PositionID, v.Headcount, v.Status, v.OpenedAt); err != nil {
		return fmt.Errorf("vacancies: upsert: %w", err)
	}
	return nil
}

func (r *pgRepository) SetStatusByPSID(ctx context.Context, psVacancyID, status string) error {
	const q = `UPDATE vacancies SET status = $2, ps_synced_at = NOW() WHERE ps_vacancy_id = $1`
	if _, err := r.pool.Exec(ctx, q, psVacancyID, status); err != nil {
		return fmt.Errorf("vacancies: set status by ps id: %w", err)
	}
	return nil
}
