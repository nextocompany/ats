package rbac

// Legacy fallback matrix — the original compile-time role→permission and
// role→scope rules, captured in one place. It is used ONLY when no dynamic
// authorizer is installed (SetDefault never called): unit tests, and as a
// fail-static safety net if the DB hasn't been migrated/seeded yet. This makes
// the system behave EXACTLY as it did before dynamic RBAC in those cases, and
// guarantees it is never fail-open. Once SetDefault installs a DB-backed
// authorizer (production), the dynamic matrix takes over completely.
//
// This is also the single source of truth the seed-matrix parity test checks
// against the independently-transcribed legacy allowlists.

// legacyRoleScope mirrors the original switch in scope.go (pre-cutover) plus the
// new 2-axis roles seeded additively in migration 000042. Used only as the
// fail-static fallback when no DB authorizer is installed.
var legacyRoleScope = map[string]string{
	RoleSuperAdmin:       KindAll,
	"regional_director":  KindAll,
	"auditor":            KindAll,
	"operation_director": KindSubregion,
	"sgm":                KindStore,
	"hr_manager":         KindStore,
	"hr_staff":           KindStore,
	// New role model (see rbac-role-redesign-analysis.plan.md).
	"hr_store":             KindStore,
	"area_hr":              KindArea,
	"hiring_manager_store": KindRequisition,
	"hiring_manager_ho":    KindRequisition,
	"ta":                   KindAll,
}

// legacyRolePerms mirrors the original hardcoded allowlists, super_admin omitted
// (it is a code bypass that holds everything). Matches migration 000028's seed.
var legacyRolePerms = map[string][]string{
	"regional_director":  {PermExecutiveView, PermReportsView, PermReportsExport, PermReengageTrigger, PermApprovalDecideL4},
	"auditor":            {PermExecutiveView, PermReportsView},
	"operation_director": {PermReportsView, PermReengageTrigger},
	"sgm":                {PermReportsView, PermBulkUpload, PermAssignmentWrite, PermOnboardingWrite, PermLetterWrite, PermScorecardLM, PermApprovalDecideL3},
	"hr_manager":         {PermMembersAdmin, PermReportsView, PermBulkUpload, PermAssignmentWrite, PermOfferWrite, PermOnboardingWrite, PermLetterWrite, PermScorecardTA, PermApprovalDecideL2},
	"hr_staff":           {PermReportsView, PermBulkUpload, PermOnboardingWrite, PermLetterWrite, PermScorecardTA, PermApprovalSubmit, PermApprovalDecideL1},
	// New role model (mirrors migration 000042's seed). hiring_manager_* are
	// read-only (scope only, no permissions until the approval-chain remap).
	"hr_store": {PermReportsView, PermBulkUpload, PermAssignmentWrite, PermOfferWrite, PermOnboardingWrite, PermLetterWrite, PermScorecardTA},
	"area_hr":  {PermReportsView, PermBulkUpload, PermAssignmentWrite, PermOfferWrite, PermOnboardingWrite, PermLetterWrite, PermScorecardTA},
	"ta":       {PermReportsView, PermReportsExport, PermBulkUpload, PermAssignmentWrite, PermOfferWrite, PermOnboardingWrite, PermLetterWrite, PermScorecardTA},
}

func legacyCan(role, perm string) bool {
	if role == RoleSuperAdmin {
		return true
	}
	for _, p := range legacyRolePerms[role] {
		if p == perm {
			return true
		}
	}
	return false
}

func legacyScopeKind(role string) string {
	if k, ok := legacyRoleScope[role]; ok {
		return k
	}
	return KindStore // unknown role → most restrictive
}

func legacyPermissions(role string) []string {
	if role == RoleSuperAdmin {
		out := make([]string, len(AllPermissions))
		copy(out, AllPermissions)
		return out
	}
	src := legacyRolePerms[role]
	out := make([]string, len(src))
	copy(out, src)
	return out
}
