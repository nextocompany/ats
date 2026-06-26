// Frontend capability gates for dynamic RBAC. The backend is the real gate; these
// only decide what to render. Every check resolves against the caller's resolved
// permission set (Me.permissions, from GET /users/me) instead of hardcoded role
// lists — so changing a role's permissions in the Admin console takes effect here
// without a code change. Permission keys mirror the Go catalog in internal/rbac.

// PermHolder is anything carrying a resolved permission set (typically `Me`).
type PermHolder = { permissions?: string[]; role?: string } | undefined | null;

// PERMS mirrors the backend permission catalog (internal/rbac/permissions.go).
export const PERMS = {
  settingsAdmin: "settings.admin",
  usersAdmin: "users.admin",
  rbacAdmin: "rbac.admin",
  executiveView: "executive.view",
  reportsView: "reports.view",
  reportsExport: "reports.export",
  reengageTrigger: "reengage.trigger",
  membersAdmin: "members.admin",
  membersErase: "members.erase",
  bulkUpload: "bulk.upload",
  assignmentWrite: "assignment.write",
  offerWrite: "offer.write",
  onboardingWrite: "onboarding.write",
  letterWrite: "letter.write",
  scorecardTa: "scorecard.ta",
  scorecardLm: "scorecard.lm",
  approvalSubmit: "approval.submit",
  requisitionManage: "requisition.manage",
  requisitionApprove: "requisition.approve",
  breachManage: "breach.manage",
  pdpaAdmin: "pdpa.admin",
  areaAdmin: "area.admin",
} as const;

// can reports whether the user holds a permission key.
export function can(me: PermHolder, perm: string): boolean {
  return !!me?.permissions?.includes(perm);
}

// isSuperAdmin is a role-identity check (super_admin holds every permission via a
// backend code bypass). Kept role-based for the admin-console gate.
export function isSuperAdmin(me: PermHolder): boolean {
  return me?.role === "super_admin";
}

// isLineManager gates the Shortlist (the store GM's own ranked view). This is a
// role identity, not a permission — the shortlist has no permission key.
export function isLineManager(me: PermHolder): boolean {
  return me?.role === "sgm";
}

export function isMemberAdmin(me: PermHolder): boolean {
  return can(me, PERMS.membersAdmin);
}

// canManageUsers gates the user-account console (assign role + scope to any
// account, SSO or local). super_admin always; any role granted users.admin
// (e.g. a CPO/CHRO role) qualifies too, so account management is not hardcoded
// to super_admin.
export function canManageUsers(me: PermHolder): boolean {
  return can(me, PERMS.usersAdmin);
}

// canEraseMember gates the irreversible PDPA anonymize.
export function canEraseMember(me: PermHolder): boolean {
  return can(me, PERMS.membersErase);
}

export function canBulkUpload(me: PermHolder): boolean {
  return can(me, PERMS.bulkUpload);
}

export function canViewExecutive(me: PermHolder): boolean {
  return can(me, PERMS.executiveView);
}

// canEditExecCost gates the ROI cost-assumptions editor (stricter than viewing).
export function canEditExecCost(me: PermHolder): boolean {
  return can(me, PERMS.settingsAdmin);
}

export function canViewReports(me: PermHolder): boolean {
  return can(me, PERMS.reportsView);
}

export function canReassignPlacement(me: PermHolder): boolean {
  return can(me, PERMS.assignmentWrite);
}

export function canManageOffer(me: PermHolder): boolean {
  return can(me, PERMS.offerWrite);
}

export function canManageLetters(me: PermHolder): boolean {
  return can(me, PERMS.letterWrite);
}

export function canManageOnboarding(me: PermHolder): boolean {
  return can(me, PERMS.onboardingWrite);
}

export function canRecordTaScorecard(me: PermHolder): boolean {
  return can(me, PERMS.scorecardTa);
}

export function canRecordLmScorecard(me: PermHolder): boolean {
  return can(me, PERMS.scorecardLm);
}

export function canRecordInterviewFeedback(me: PermHolder): boolean {
  return can(me, PERMS.scorecardTa) || can(me, PERMS.scorecardLm);
}

// canViewInterviews gates the HR interview calendar: HR who schedule (assignment
// writers) plus the interviewers who record feedback (TA / line manager).
export function canViewInterviews(me: PermHolder): boolean {
  return can(me, PERMS.assignmentWrite) || canRecordInterviewFeedback(me);
}

// canManageRequisitions gates opening/editing/closing position openings.
export function canManageRequisitions(me: PermHolder): boolean {
  return can(me, PERMS.requisitionManage);
}

// canCompareCandidates gates the per-position Compare Candidates view — a hiring
// decision aid. Generous on purpose (decision-makers + HR who handle the funnel:
// line managers, placement writers, interviewers, requisition managers); the
// backend RBAC scope is the real boundary on which candidates are visible.
export function canCompareCandidates(me: PermHolder): boolean {
  return (
    isLineManager(me) ||
    isSuperAdmin(me) ||
    can(me, PERMS.assignmentWrite) ||
    canRecordInterviewFeedback(me) ||
    can(me, PERMS.requisitionManage)
  );
}

// canApproveRequisitions gates approving a pending requisition into 'open'.
export function canApproveRequisitions(me: PermHolder): boolean {
  return can(me, PERMS.requisitionApprove);
}

// canAdminPdpa gates the PDPA/DPO console (DSAR queue, consent lookup, overview).
export function canAdminPdpa(me: PermHolder): boolean {
  return can(me, PERMS.pdpaAdmin);
}

// canManageScoring gates the per-position screening-weights settings page.
export function canManageScoring(me: PermHolder): boolean {
  return can(me, PERMS.settingsAdmin);
}

// canManageAreas gates the area-management console (dynamic store groupings +
// area_hr assignment) backing the area visibility scope.
export function canManageAreas(me: PermHolder): boolean {
  return can(me, PERMS.areaAdmin);
}

// canSubmitApproval gates opening a hiring-approval request.
export function canSubmitApproval(me: PermHolder): boolean {
  return can(me, PERMS.approvalSubmit);
}

// canDecideApprovalLevel gates the per-level decision (approval.decide.l1..l4).
export function canDecideApprovalLevel(me: PermHolder, level: number): boolean {
  return can(me, `approval.decide.l${level}`);
}

// canAccessApprovals: anyone who can submit or decide any level may open the queue.
export function canAccessApprovals(me: PermHolder): boolean {
  return (
    can(me, PERMS.approvalSubmit) ||
    canDecideApprovalLevel(me, 1) ||
    canDecideApprovalLevel(me, 2) ||
    canDecideApprovalLevel(me, 3) ||
    canDecideApprovalLevel(me, 4)
  );
}
