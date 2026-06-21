package rbac

// Permission keys — the FIXED catalog of gateable actions. Each constant maps 1:1
// to a code call site (a handler authorization check); a genuinely new capability
// is a new call site, so this catalog only grows via code. The role→permission
// MATRIX, by contrast, is data-driven (rbac_role_permissions) and editable at
// runtime. These constants are the single source of truth for ENFORCEMENT; the
// rbac_permissions table mirrors them (labels/grouping) for the admin UI and is
// kept in sync by migration + the AllPermissions parity test.
const (
	PermSettingsAdmin      = "settings.admin"      // system settings (allow-all-tenants)
	PermUsersAdmin         = "users.admin"         // HR user-account CRUD
	PermRBACAdmin          = "rbac.admin"          // role/permission CRUD (this feature)
	PermExecutiveView      = "executive.view"      // company-wide executive overview
	PermReportsView        = "reports.view"        // ATS reports page
	PermReportsExport      = "reports.export"      // on-demand report export
	PermReengageTrigger    = "reengage.trigger"    // manual re-engagement sweep
	PermMembersAdmin       = "members.admin"       // member-management console
	PermMembersErase       = "members.erase"       // irreversible PDPA anonymize
	PermBulkUpload         = "bulk.upload"         // bulk CV intake
	PermAssignmentWrite    = "assignment.write"    // manual branch (re)assignment
	PermOfferWrite         = "offer.write"         // compose/send offers
	PermOnboardingWrite    = "onboarding.write"    // review onboarding documents
	PermLetterWrite        = "letter.write"        // generate PDF letters
	PermScorecardTA        = "scorecard.ta"        // TA (recruiter) scorecard
	PermScorecardLM        = "scorecard.lm"        // line-manager scorecard
	PermApprovalSubmit     = "approval.submit"     // open a hiring-approval request
	PermApprovalDecideL1   = "approval.decide.l1"  // approve at level 1 (staff)
	PermApprovalDecideL2   = "approval.decide.l2"  // approve at level 2 (hr manager)
	PermApprovalDecideL3   = "approval.decide.l3"  // approve at level 3 (sgm)
	PermApprovalDecideL4   = "approval.decide.l4"  // approve at level 4 (regional)
	PermRequisitionManage  = "requisition.manage"  // create/edit/close a requisition
	PermRequisitionApprove = "requisition.approve" // approve a pending requisition → open
	PermBreachManage       = "breach.manage"       // PDPA breach register CRUD (s.37 72h)
)

// RoleSuperAdmin is the built-in role that is a hard code bypass: it implicitly
// holds every permission and the broadest scope, so a misconfigured matrix can
// never lock the system out.
const RoleSuperAdmin = "super_admin"

// AllPermissions is the canonical ordered list of catalog keys. A migration-parity
// test asserts this equals the rbac_permissions table, so code and DB never drift.
var AllPermissions = []string{
	PermSettingsAdmin, PermUsersAdmin, PermRBACAdmin,
	PermExecutiveView, PermReportsView, PermReportsExport,
	PermReengageTrigger,
	PermMembersAdmin, PermMembersErase, PermBulkUpload, PermAssignmentWrite,
	PermOfferWrite, PermOnboardingWrite, PermLetterWrite, PermScorecardTA, PermScorecardLM,
	PermApprovalSubmit, PermApprovalDecideL1, PermApprovalDecideL2, PermApprovalDecideL3, PermApprovalDecideL4,
	PermRequisitionManage, PermRequisitionApprove,
	PermBreachManage,
}

// ApprovalDecidePermForLevel returns the permission gating a given approval chain
// level (1–4), or "" for an out-of-range level.
func ApprovalDecidePermForLevel(level int) string {
	switch level {
	case 1:
		return PermApprovalDecideL1
	case 2:
		return PermApprovalDecideL2
	case 3:
		return PermApprovalDecideL3
	case 4:
		return PermApprovalDecideL4
	default:
		return ""
	}
}
