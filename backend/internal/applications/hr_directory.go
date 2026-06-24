package applications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
)

// HRDirectory resolves the HR recipients for store-scoped notifications. Accepted
// as an interface so callers (api + worker) and tests stay decoupled from pgx.
type HRDirectory interface {
	// EmailsForStore returns active store-HR emails for the given store. A nil
	// storeID (talent pool / unassigned) returns no recipients — we never email
	// every store.
	EmailsForStore(ctx context.Context, storeID *int) ([]string, error)
	// LineManagerEmailsForStore returns active line-manager (sgm) emails for the
	// store, for the Top-5 shortlist-ready notification. nil storeID → none.
	LineManagerEmailsForStore(ctx context.Context, storeID *int) ([]string, error)
	// EmailsForRoleStore returns active emails for a single role, scoped to the
	// store when the role is store-scoped (hr_staff/hr_manager/sgm — nil storeID →
	// none) and ignoring the store for all-scope roles (regional_director). Used to
	// reach the responsible approver(s) for an approval step's SLA escalation.
	EmailsForRoleStore(ctx context.Context, role string, storeID *int) ([]string, error)
}

// hrNotifyRoles are the roles that receive store candidate notifications. Union of
// the legacy roles and the new model (hr_store/area_hr + hiring_manager_store)
// during the cutover, so notifications reach holders of either set.
var hrNotifyRoles = []string{"sgm", "hr_manager", "hr_staff", "hr_store", "area_hr", "hiring_manager_store"}

// lineManagerRoles are the roles that act as the store's line manager (sgm →
// hiring_manager_store in the new model).
var lineManagerRoles = []string{"sgm", "hiring_manager_store"}

type pgHRDirectory struct {
	pool *pgxpool.Pool
}

// NewHRDirectory builds a Postgres-backed HR directory.
func NewHRDirectory(pool *pgxpool.Pool) HRDirectory { return &pgHRDirectory{pool: pool} }

func (d *pgHRDirectory) EmailsForStore(ctx context.Context, storeID *int) ([]string, error) {
	return d.emailsForStoreRoles(ctx, storeID, hrNotifyRoles)
}

func (d *pgHRDirectory) LineManagerEmailsForStore(ctx context.Context, storeID *int) ([]string, error) {
	return d.emailsForStoreRoles(ctx, storeID, lineManagerRoles)
}

// roleIsStoreScoped reports whether a role's RBAC visibility is limited to a single
// store (so approver resolution must filter by store_id).
func roleIsStoreScoped(role string) bool {
	return rbac.New(role, nil, "").Kind() == rbac.KindStore
}

func (d *pgHRDirectory) EmailsForRoleStore(ctx context.Context, role string, storeID *int) ([]string, error) {
	if roleIsStoreScoped(role) {
		return d.emailsForStoreRoles(ctx, storeID, []string{role})
	}
	// All-scope role (e.g. regional_director): every active holder, store-agnostic.
	const q = `
		SELECT email FROM users
		WHERE is_active AND role = $1 AND COALESCE(email,'') <> ''`
	rows, err := d.pool.Query(ctx, q, role)
	if err != nil {
		return nil, fmt.Errorf("applications: emails for role: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("applications: scan role email: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate role emails: %w", err)
	}
	return out, nil
}

func (d *pgHRDirectory) emailsForStoreRoles(ctx context.Context, storeID *int, roles []string) ([]string, error) {
	if storeID == nil {
		return nil, nil
	}
	const q = `
		SELECT email FROM users
		WHERE is_active AND store_id = $1 AND role = ANY($2) AND COALESCE(email,'') <> ''`
	rows, err := d.pool.Query(ctx, q, *storeID, roles)
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
