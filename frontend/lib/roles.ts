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
