// Package rbac derives query-level data scoping from the authenticated user.
// It is the single place role→visibility rules live, so every list/report
// endpoint scopes consistently.
package rbac

import "fmt"

// Scope kinds, from broadest to narrowest.
const (
	KindAll         = "all"
	KindSubregion   = "subregion"
	KindArea        = "area"        // dynamic, admin-managed grouping of stores (+ central pool)
	KindRequisition = "requisition" // candidates in positions the user opened (hiring manager)
	KindStore       = "store"
)

// Scope is the visibility a user has over candidate/application data.
type Scope struct {
	Role      string
	StoreID   *int
	Subregion string
	// UserID (UUID) is required for the area and requisition scopes, which resolve
	// the user's areas / owned requisitions in SQL. Empty for those kinds fails
	// closed (matches nothing). Set it with WithUserID.
	UserID string
	all    bool // surface-specific bypass: Kind() == KindAll regardless of role
}

// New builds a Scope from authenticated-user attributes.
func New(role string, storeID *int, subregion string) Scope {
	return Scope{Role: role, StoreID: storeID, Subregion: subregion}
}

// WithUserID returns a copy of the scope carrying the user's UUID, needed by the
// area and requisition scopes. Immutable: the receiver is unchanged.
func (s Scope) WithUserID(userID string) Scope {
	s.UserID = userID
	return s
}

// poolFragment is the central-pool predicate shared by store/area scopes: a
// candidate not tied to any store (released to, or born into, the central pool)
// is visible to every store/area operator. It adds no placeholder.
const poolFragment = "(talent_pool = TRUE AND assigned_store_id IS NULL)"

// AllScope returns an unrestricted Scope (KindAll) regardless of role. Use it
// when a surface grants a role full visibility despite its default store/subregion
// scope — e.g. a member-admin who owns the company-wide member directory.
func AllScope() Scope {
	return Scope{all: true}
}

// Kind classifies the scope via the installed authorizer (or the legacy
// fallback). Unknown roles fall through to the most restrictive (store) kind, so
// a misconfigured role never widens visibility.
func (s Scope) Kind() string {
	if s.all {
		return KindAll
	}
	return ScopeKindFor(s.Role)
}

// ApplicationsClause returns a SQL condition scoping the applications table,
// with placeholders numbered from argStart. An empty clause means no scoping.
func (s Scope) ApplicationsClause(argStart int) (string, []any) {
	switch s.Kind() {
	case KindSubregion:
		return fmt.Sprintf("assigned_store_id IN (SELECT store_no FROM stores WHERE subregion = $%d)", argStart), []any{s.Subregion}
	case KindArea:
		if s.UserID == "" {
			return "1=0", nil // area-scoped user without an id sees nothing
		}
		return fmt.Sprintf(
			"(assigned_store_id IN (SELECT store_no FROM area_stores WHERE area_id IN (SELECT area_id FROM user_areas WHERE user_id = $%d::uuid)) OR %s)",
			argStart, poolFragment), []any{s.UserID}
	case KindRequisition:
		if s.UserID == "" {
			return "1=0", nil // hiring manager without an id sees nothing (default-deny)
		}
		return fmt.Sprintf(
			"vacancy_id IN (SELECT id FROM vacancies WHERE hiring_manager_user_id = $%d::uuid)", argStart), []any{s.UserID}
	case KindStore:
		if s.StoreID == nil {
			return "1=0", nil // store-scoped user without a store sees nothing
		}
		return fmt.Sprintf("(assigned_store_id = $%d OR %s)", argStart, poolFragment), []any{*s.StoreID}
	default:
		return "", nil
	}
}

// VacanciesClause returns a SQL condition scoping the vacancies table by its
// store_id column, with placeholders numbered from argStart. An empty clause
// means no scoping (KindAll). Used by the requisition-management list/detail.
func (s Scope) VacanciesClause(argStart int) (string, []any) {
	switch s.Kind() {
	case KindSubregion:
		return fmt.Sprintf("store_id IN (SELECT store_no FROM stores WHERE subregion = $%d)", argStart), []any{s.Subregion}
	case KindArea:
		if s.UserID == "" {
			return "1=0", nil
		}
		return fmt.Sprintf(
			"store_id IN (SELECT store_no FROM area_stores WHERE area_id IN (SELECT area_id FROM user_areas WHERE user_id = $%d::uuid))", argStart), []any{s.UserID}
	case KindRequisition:
		if s.UserID == "" {
			return "1=0", nil
		}
		// A hiring manager manages only the requisitions they own.
		return fmt.Sprintf("hiring_manager_user_id = $%d::uuid", argStart), []any{s.UserID}
	case KindStore:
		if s.StoreID == nil {
			return "1=0", nil // store-scoped user without a store sees nothing
		}
		return fmt.Sprintf("store_id = $%d", argStart), []any{*s.StoreID}
	default:
		return "", nil
	}
}

// CandidatesClause returns a SQL condition scoping the candidates table.
func (s Scope) CandidatesClause(argStart int) (string, []any) {
	switch s.Kind() {
	case KindSubregion:
		return fmt.Sprintf("subregion = $%d", argStart), []any{s.Subregion}
	case KindArea:
		if s.UserID == "" {
			return "1=0", nil
		}
		return fmt.Sprintf(
			"id IN (SELECT candidate_id FROM applications WHERE assigned_store_id IN (SELECT store_no FROM area_stores WHERE area_id IN (SELECT area_id FROM user_areas WHERE user_id = $%d::uuid)) OR %s)",
			argStart, poolFragment), []any{s.UserID}
	case KindRequisition:
		if s.UserID == "" {
			return "1=0", nil
		}
		return fmt.Sprintf(
			"id IN (SELECT candidate_id FROM applications WHERE vacancy_id IN (SELECT id FROM vacancies WHERE hiring_manager_user_id = $%d::uuid))", argStart), []any{s.UserID}
	case KindStore:
		if s.StoreID == nil {
			return "1=0", nil
		}
		return fmt.Sprintf("id IN (SELECT candidate_id FROM applications WHERE assigned_store_id = $%d OR %s)", argStart, poolFragment), []any{*s.StoreID}
	default:
		return "", nil
	}
}

// AccountsClause returns a SQL condition scoping the candidate_accounts table by
// the visibility a user has over the per-intake candidate rows linked to each
// account (candidates.account_id). outerAlias is the candidate_accounts alias in
// the caller's query (e.g. "a"); placeholders are numbered from argStart. An empty
// clause means no scoping (KindAll → every account, including 0-application ones).
//
// A store/subregion user sees an account only when it owns at least one linked
// candidate inside their scope — mirroring CandidatesClause so the unified
// account-keyed Candidates list scopes identically to the old candidate list.
// Accounts with no linked in-scope candidate (e.g. a portal signup who never
// applied in this store) stay invisible to scoped roles, visible to KindAll.
func (s Scope) AccountsClause(outerAlias string, argStart int) (string, []any) {
	const apPool = "(ap.talent_pool = TRUE AND ap.assigned_store_id IS NULL)"
	switch s.Kind() {
	case KindSubregion:
		return fmt.Sprintf(
			"EXISTS (SELECT 1 FROM candidates c WHERE c.account_id = %s.id AND c.subregion = $%d)",
			outerAlias, argStart), []any{s.Subregion}
	case KindArea:
		if s.UserID == "" {
			return "1=0", nil
		}
		return fmt.Sprintf(
			"EXISTS (SELECT 1 FROM candidates c JOIN applications ap ON ap.candidate_id = c.id WHERE c.account_id = %s.id AND (ap.assigned_store_id IN (SELECT store_no FROM area_stores WHERE area_id IN (SELECT area_id FROM user_areas WHERE user_id = $%d::uuid)) OR %s))",
			outerAlias, argStart, apPool), []any{s.UserID}
	case KindRequisition:
		if s.UserID == "" {
			return "1=0", nil
		}
		return fmt.Sprintf(
			"EXISTS (SELECT 1 FROM candidates c JOIN applications ap ON ap.candidate_id = c.id WHERE c.account_id = %s.id AND ap.vacancy_id IN (SELECT id FROM vacancies WHERE hiring_manager_user_id = $%d::uuid))",
			outerAlias, argStart), []any{s.UserID}
	case KindStore:
		if s.StoreID == nil {
			return "1=0", nil
		}
		return fmt.Sprintf(
			"EXISTS (SELECT 1 FROM candidates c JOIN applications ap ON ap.candidate_id = c.id WHERE c.account_id = %s.id AND (ap.assigned_store_id = $%d OR %s))",
			outerAlias, argStart, apPool), []any{*s.StoreID}
	default:
		return "", nil
	}
}
