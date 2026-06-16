package applications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HRDirectory resolves the HR recipients for store-scoped notifications. Accepted
// as an interface so callers (api + worker) and tests stay decoupled from pgx.
type HRDirectory interface {
	// EmailsForStore returns active store-HR emails for the given store. A nil
	// storeID (talent pool / unassigned) returns no recipients — we never email
	// every store.
	EmailsForStore(ctx context.Context, storeID *int) ([]string, error)
}

// hrNotifyRoles are the store-scoped roles that receive candidate notifications.
var hrNotifyRoles = []string{"sgm", "hr_manager", "hr_staff"}

type pgHRDirectory struct {
	pool *pgxpool.Pool
}

// NewHRDirectory builds a Postgres-backed HR directory.
func NewHRDirectory(pool *pgxpool.Pool) HRDirectory { return &pgHRDirectory{pool: pool} }

func (d *pgHRDirectory) EmailsForStore(ctx context.Context, storeID *int) ([]string, error) {
	if storeID == nil {
		return nil, nil
	}
	const q = `
		SELECT email FROM users
		WHERE is_active AND store_id = $1 AND role = ANY($2) AND COALESCE(email,'') <> ''`
	rows, err := d.pool.Query(ctx, q, *storeID, hrNotifyRoles)
	if err != nil {
		return nil, fmt.Errorf("applications: hr emails for store: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("applications: scan hr email: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate hr emails: %w", err)
	}
	return out, nil
}
