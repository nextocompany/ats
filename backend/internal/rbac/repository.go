package rbac

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// uniqueViolation is the Postgres SQLSTATE for a unique-constraint violation.
const uniqueViolation = "23505"

// Repository is the data access for roles, the role→permission matrix, and the
// permission catalog. The authorizer reads through it; the admin handler writes.
type Repository interface {
	ListRoles(ctx context.Context) ([]Role, error)
	GetRole(ctx context.Context, key string) (Role, error)
	RoleExists(ctx context.Context, key string) (bool, error)
	ListPermissions(ctx context.Context) ([]Permission, error)
	CreateRole(ctx context.Context, key, labelEn, labelTh, scopeKind string, perms []string) (Role, error)
	UpdateRole(ctx context.Context, key string, in RoleInput) (Role, error)
	DeleteRole(ctx context.Context, key string) error
	CountUsersWithRole(ctx context.Context, key string) (int, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed rbac repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

const roleColumns = `key, label_en, label_th, scope_kind, is_builtin, created_at`

func scanRole(row pgx.Row) (Role, error) {
	var r Role
	if err := row.Scan(&r.Key, &r.LabelEn, &r.LabelTh, &r.ScopeKind, &r.IsBuiltin, &r.CreatedAt); err != nil {
		return Role{}, err
	}
	return r, nil
}

// permsByRole returns role_key → sorted permission keys for the whole table.
func (r *pgRepository) permsByRole(ctx context.Context) (map[string][]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT role_key, permission FROM rbac_role_permissions`)
	if err != nil {
		return nil, fmt.Errorf("rbac: list role permissions: %w", err)
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var rk, p string
		if err := rows.Scan(&rk, &p); err != nil {
			return nil, fmt.Errorf("rbac: scan role permission: %w", err)
		}
		out[rk] = append(out[rk], p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rbac: iterate role permissions: %w", err)
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out, nil
}

func (r *pgRepository) ListRoles(ctx context.Context) ([]Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+roleColumns+` FROM rbac_roles ORDER BY is_builtin DESC, key`)
	if err != nil {
		return nil, fmt.Errorf("rbac: list roles: %w", err)
	}
	defer rows.Close()
	var roles []Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, fmt.Errorf("rbac: scan role: %w", err)
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rbac: iterate roles: %w", err)
	}
	perms, err := r.permsByRole(ctx)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		roles[i].Permissions = perms[roles[i].Key]
		if roles[i].Permissions == nil {
			roles[i].Permissions = []string{}
		}
	}
	return roles, nil
}

func (r *pgRepository) GetRole(ctx context.Context, key string) (Role, error) {
	role, err := scanRole(r.pool.QueryRow(ctx, `SELECT `+roleColumns+` FROM rbac_roles WHERE key = $1`, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrRoleNotFound
	}
	if err != nil {
		return Role{}, fmt.Errorf("rbac: get role: %w", err)
	}
	perms, err := r.permsByRole(ctx)
	if err != nil {
		return Role{}, err
	}
	role.Permissions = perms[role.Key]
	if role.Permissions == nil {
		role.Permissions = []string{}
	}
	return role, nil
}

func (r *pgRepository) RoleExists(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rbac_roles WHERE key = $1)`, key).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("rbac: role exists: %w", err)
	}
	return exists, nil
}

func (r *pgRepository) ListPermissions(ctx context.Context) ([]Permission, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT key, label_en, label_th, category, sort FROM rbac_permissions ORDER BY category, sort, key`)
	if err != nil {
		return nil, fmt.Errorf("rbac: list permissions: %w", err)
	}
	defer rows.Close()
	var perms []Permission
	for rows.Next() {
		var p Permission
		if err := rows.Scan(&p.Key, &p.LabelEn, &p.LabelTh, &p.Category, &p.Sort); err != nil {
			return nil, fmt.Errorf("rbac: scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rbac: iterate permissions: %w", err)
	}
	return perms, nil
}

func (r *pgRepository) CreateRole(ctx context.Context, key, labelEn, labelTh, scopeKind string, perms []string) (Role, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Role{}, fmt.Errorf("rbac: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	role, err := scanRole(tx.QueryRow(ctx,
		`INSERT INTO rbac_roles (key, label_en, label_th, scope_kind, is_builtin)
		 VALUES ($1, $2, $3, $4, FALSE) RETURNING `+roleColumns,
		key, labelEn, labelTh, scopeKind))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return Role{}, ErrRoleExists
	}
	if err != nil {
		return Role{}, fmt.Errorf("rbac: create role: %w", err)
	}
	if err := replacePerms(ctx, tx, key, perms); err != nil {
		return Role{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Role{}, fmt.Errorf("rbac: commit: %w", err)
	}
	role.Permissions = dedupSorted(perms)
	return role, nil
}

func (r *pgRepository) UpdateRole(ctx context.Context, key string, in RoleInput) (Role, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Role{}, fmt.Errorf("rbac: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Partial column update (mirror hrauth UpdateUser's dynamic builder).
	set := []string{}
	args := []any{}
	add := func(expr string, val any) {
		args = append(args, val)
		set = append(set, fmt.Sprintf("%s = $%d", expr, len(args)))
	}
	if in.LabelEn != nil {
		add("label_en", *in.LabelEn)
	}
	if in.LabelTh != nil {
		add("label_th", *in.LabelTh)
	}
	if in.ScopeKind != nil {
		add("scope_kind", *in.ScopeKind)
	}
	if len(set) > 0 {
		args = append(args, key)
		q := fmt.Sprintf(`UPDATE rbac_roles SET %s WHERE key = $%d`, strings.Join(set, ", "), len(args))
		ct, err := tx.Exec(ctx, q, args...)
		if err != nil {
			return Role{}, fmt.Errorf("rbac: update role: %w", err)
		}
		if ct.RowsAffected() == 0 {
			return Role{}, ErrRoleNotFound
		}
	} else {
		exists, err := txRoleExists(ctx, tx, key)
		if err != nil {
			return Role{}, err
		}
		if !exists {
			return Role{}, ErrRoleNotFound
		}
	}
	if in.Permissions != nil {
		if err := replacePerms(ctx, tx, key, *in.Permissions); err != nil {
			return Role{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Role{}, fmt.Errorf("rbac: commit: %w", err)
	}
	return r.GetRole(ctx, key)
}

func (r *pgRepository) DeleteRole(ctx context.Context, key string) error {
	ct, err := r.pool.Exec(ctx, `DELETE FROM rbac_roles WHERE key = $1 AND is_builtin = FALSE`, key)
	if err != nil {
		return fmt.Errorf("rbac: delete role: %w", err)
	}
	if ct.RowsAffected() == 0 {
		// Either it doesn't exist or it's built-in. Disambiguate for a clear error.
		exists, exErr := r.RoleExists(ctx, key)
		if exErr != nil {
			return exErr
		}
		if exists {
			return ErrRoleBuiltin
		}
		return ErrRoleNotFound
	}
	return nil
}

func (r *pgRepository) CountUsersWithRole(ctx context.Context, key string) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = $1`, key).Scan(&n); err != nil {
		return 0, fmt.Errorf("rbac: count users with role: %w", err)
	}
	return n, nil
}

// --- helpers ---------------------------------------------------------------

// txQuerier is the subset of pgx.Tx used by the perm helpers.
type txQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// replacePerms sets a role's permission set to exactly perms (full replacement).
func replacePerms(ctx context.Context, tx txQuerier, key string, perms []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM rbac_role_permissions WHERE role_key = $1`, key); err != nil {
		return fmt.Errorf("rbac: clear role permissions: %w", err)
	}
	for _, p := range dedupSorted(perms) {
		if _, err := tx.Exec(ctx,
			`INSERT INTO rbac_role_permissions (role_key, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			key, p); err != nil {
			return fmt.Errorf("rbac: insert role permission: %w", err)
		}
	}
	return nil
}

func txRoleExists(ctx context.Context, tx txQuerier, key string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rbac_roles WHERE key = $1)`, key).Scan(&exists); err != nil {
		return false, fmt.Errorf("rbac: role exists: %w", err)
	}
	return exists, nil
}

func dedupSorted(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
