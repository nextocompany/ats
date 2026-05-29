// Package rbac derives query-level data scoping from the authenticated user.
// It is the single place role→visibility rules live, so every list/report
// endpoint scopes consistently.
package rbac

import "fmt"

// Scope kinds, from broadest to narrowest.
const (
	KindAll       = "all"
	KindSubregion = "subregion"
	KindStore     = "store"
)

// Scope is the visibility a user has over candidate/application data.
type Scope struct {
	Role      string
	StoreID   *int
	Subregion string
}

// New builds a Scope from authenticated-user attributes.
func New(role string, storeID *int, subregion string) Scope {
	return Scope{Role: role, StoreID: storeID, Subregion: subregion}
}

// Kind classifies the scope. Unknown roles fall through to the most restrictive
// (store) kind, so a misconfigured role never widens visibility.
func (s Scope) Kind() string {
	switch s.Role {
	case "super_admin", "regional_director", "auditor":
		return KindAll
	case "operation_director":
		return KindSubregion
	default: // sgm, hr_manager, hr_staff, unknown
		return KindStore
	}
}

// ApplicationsClause returns a SQL condition scoping the applications table,
// with placeholders numbered from argStart. An empty clause means no scoping.
func (s Scope) ApplicationsClause(argStart int) (string, []any) {
	switch s.Kind() {
	case KindSubregion:
		return fmt.Sprintf("assigned_store_id IN (SELECT store_no FROM stores WHERE subregion = $%d)", argStart), []any{s.Subregion}
	case KindStore:
		if s.StoreID == nil {
			return "1=0", nil // store-scoped user without a store sees nothing
		}
		return fmt.Sprintf("assigned_store_id = $%d", argStart), []any{*s.StoreID}
	default:
		return "", nil
	}
}

// CandidatesClause returns a SQL condition scoping the candidates table.
func (s Scope) CandidatesClause(argStart int) (string, []any) {
	switch s.Kind() {
	case KindSubregion:
		return fmt.Sprintf("subregion = $%d", argStart), []any{s.Subregion}
	case KindStore:
		if s.StoreID == nil {
			return "1=0", nil
		}
		return fmt.Sprintf("id IN (SELECT candidate_id FROM applications WHERE assigned_store_id = $%d)", argStart), []any{*s.StoreID}
	default:
		return "", nil
	}
}
