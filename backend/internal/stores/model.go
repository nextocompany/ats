package stores

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store maps the stores table (columns used in Sprint 2).
type Store struct {
	StoreNo    int      `json:"store_no"`
	StoreName  string   `json:"store_name"`
	FormatType string   `json:"format_type"`
	Subregion  string   `json:"subregion"`
	Province   string   `json:"province"`
	Latitude   *float64 `json:"latitude"`
	Longitude  *float64 `json:"longitude"`
}

// Repository is the store data-access contract.
type Repository interface {
	FindByNo(ctx context.Context, no int) (*Store, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed store repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) FindByNo(ctx context.Context, no int) (*Store, error) {
	const q = `
		SELECT store_no, store_name, COALESCE(format_type,''), COALESCE(subregion,''),
		       COALESCE(province,''), latitude, longitude
		FROM stores WHERE store_no = $1`
	var s Store
	if err := r.pool.QueryRow(ctx, q, no).Scan(
		&s.StoreNo, &s.StoreName, &s.FormatType, &s.Subregion, &s.Province, &s.Latitude, &s.Longitude,
	); err != nil {
		return nil, fmt.Errorf("stores: find by no: %w", err)
	}
	return &s, nil
}
