package applications

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	// store when the role is store-scoped (hr_store — nil storeID → none) and
	// ignoring the store for non-store-scoped roles (area_hr / hiring_manager_* /
	// ta resolve store-agnostically). Used to reach the responsible approver(s) for
	// an approval step's SLA escalation. NOTE: area/requisition-scoped approver
	// levels therefore email every holder of that role, not just the area/req
	// subset — acceptable for best-effort SLA nudges; tighten if it gets noisy.
	EmailsForRoleStore(ctx context.Context, role string, storeID *int) ([]string, error)
	// HiringManagerForVacancy resolves the active hiring manager (email + full name)
	// who owns the vacancy. Returns empty strings and no error when the vacancy has
	// no in-app hiring manager (talent-pool routing, or PeopleSoft openings without
	// an owner) — the caller treats that as "nobody to notify".
	HiringManagerForVacancy(ctx context.Context, vacancyID uuid.UUID) (email, fullName string, err error)
}

// legacyRoleFor maps each new role to the pre-cutover role it replaced. During the
// split-deploy window (this code is live, but the cutover migration 000044 that
// migrates users onto the new roles has not run yet), live users still hold the OLD
// role strings — so notification resolution must reach holders of EITHER label, or
// store HR silently stop receiving every alert. After cutover no user holds an old
// role, so these entries become inert (no rows match). Mirrors 000044's mapping,
// store-relevant subset.
var legacyRoleFor = map[string]string{
	"hr_store":             "hr_staff",
	"area_hr":              "hr_manager",
	"hiring_manager_store": "sgm",
	"ta":                   "regional_director",
}

// hrNotifyRoles are the store-scoped roles that receive candidate notifications.
// Includes both the new roles (post-cutover) AND their pre-cutover equivalents so
// the split-deploy window stays clean (see legacyRoleFor).
var hrNotifyRoles = []string{
	"hiring_manager_store", "area_hr", "hr_store", // new
	"sgm", "hr_manager", "hr_staff", // pre-cutover equivalents (transition)
}

// lineManagerRoles are the roles that act as the store's line manager
// (sgm->hiring_manager_store in the cutover; both kept for the transition window).
var lineManagerRoles = []string{"hiring_manager_store", "sgm"}

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

// EmailsForRoleStore resolves the responsible approver(s) for a chain role. During
// the split-deploy window it also resolves the role's pre-cutover equivalent (each
// resolved under its OWN scope, since old/new roles differ in scope kind) and unions
// the result, so SLA/next-level approval nudges reach users not yet migrated.
func (d *pgHRDirectory) EmailsForRoleStore(ctx context.Context, role string, storeID *int) ([]string, error) {
	emails, err := d.emailsForExactRole(ctx, role, storeID)
	if err != nil {
		return nil, err
	}
	if legacy, ok := legacyRoleFor[role]; ok {
		le, err := d.emailsForExactRole(ctx, legacy, storeID)
		if err != nil {
			return nil, err
		}
		emails = dedupStrings(append(emails, le...))
	}
	return emails, nil
}

// emailsForExactRole resolves active emails for a single role, store-filtered when
// the role's RBAC visibility is store-scoped, store-agnostic otherwise.
func (d *pgHRDirectory) emailsForExactRole(ctx context.Context, role string, storeID *int) ([]string, error) {
	if roleIsStoreScoped(role) {
		return d.emailsForStoreRoles(ctx, storeID, []string{role})
	}
	// All-scope role (e.g. ta / regional_director): every active holder, store-agnostic.
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

// dedupStrings returns xs with duplicates removed, preserving first-seen order.
func dedupStrings(xs []string) []string {
	seen := make(map[string]struct{}, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}

func (d *pgHRDirectory) HiringManagerForVacancy(ctx context.Context, vacancyID uuid.UUID) (string, string, error) {
	const q = `
		SELECT COALESCE(u.email, ''), COALESCE(u.full_name, '')
		FROM vacancies v
		JOIN users u ON u.id = v.hiring_manager_user_id
		WHERE v.id = $1 AND u.is_active AND COALESCE(u.email, '') <> ''`
	var email, fullName string
	err := d.pool.QueryRow(ctx, q, vacancyID).Scan(&email, &fullName)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil // no in-app hiring manager — nobody to notify (not an error)
	}
	if err != nil {
		return "", "", fmt.Errorf("applications: hiring manager for vacancy: %w", err)
	}
	return email, fullName, nil
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
