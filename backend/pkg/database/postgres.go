// Package database owns the PostgreSQL connection pool. Sprint 1+ domain
// repositories receive the *pgxpool.Pool via dependency injection — it is never
// a package global.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	connectTimeout  = 10 * time.Second
	defaultMaxConns = 10
)

// Connect parses the DSN, opens a pool, and verifies connectivity with a ping.
// The caller owns the returned pool and must Close it on shutdown.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("database: parse dsn: %w", err)
	}
	cfg.MaxConns = defaultMaxConns

	cctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(cctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database: connect: %w", err)
	}
	if err := pool.Ping(cctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping: %w", err)
	}
	return pool, nil
}
