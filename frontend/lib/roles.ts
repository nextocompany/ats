// Member-management access: who may use the /members console. Mirrors the backend
// `memberAdminRoles` allowlist in internal/members/handler.go. The server is the
// real gate; this is the UI gate.
export const MEMBER_ADMIN_ROLES = ["super_admin", "hr_manager"];

export function isMemberAdmin(role?: string): boolean {
  return !!role && MEMBER_ADMIN_ROLES.includes(role);
}

// isSuperAdmin gates the irreversible PDPA erasure (anonymize). Mirrors the
// backend `memberEraseRoles` super_admin-only allowlist.
export function isSuperAdmin(role?: string): boolean {
  return role === "super_admin";
}

// INTERVIEW_FEEDBACK_ROLES may record structured interview feedback. Mirrors the
// backend `feedbackRecordRoles` allowlist in internal/applications/feedback_handler.go
// (sgm ≈ line manager who runs the interview). The server is the real gate; this
// only decides whether to render the form.
export const INTERVIEW_FEEDBACK_ROLES = ["super_admin", "hr_manager", "sgm"];

export function canRecordInterviewFeedback(role?: string): boolean {
  return !!role && INTERVIEW_FEEDBACK_ROLES.includes(role);
}

// TA_SCORECARD_ROLES may record the TA (recruiter) scorecard; LINE_MANAGER_ROLES
// (sgm = store GM ≈ line manager) record the LM scorecard. Mirrors the backend
// taRecordRoles / lmRecordRoles allowlists in feedback_handler.go. Server is the
// real gate.
export const TA_SCORECARD_ROLES = ["super_admin", "hr_manager", "hr_staff"];
export const LINE_MANAGER_ROLES = ["sgm"];

export function canRecordTaScorecard(role?: string): boolean {
  return !!role && TA_SCORECARD_ROLES.includes(role);
}

export function isLineManager(role?: string): boolean {
  return !!role && LINE_MANAGER_ROLES.includes(role);
}

// canRecordLmScorecard: the line manager plus super_admin (who may record either).
export function canRecordLmScorecard(role?: string): boolean {
  return role === "super_admin" || isLineManager(role);
}

// BULK_UPLOAD_ROLES may bulk-upload CVs. Mirrors the backend `bulkIntakeRoles`
// allowlist in internal/applications/bulk_handler.go (auditor is read-only).
export const BULK_UPLOAD_ROLES = ["super_admin", "hr_manager", "sgm", "hr_staff"];

export function canBulkUpload(role?: string): boolean {
  return !!role && BULK_UPLOAD_ROLES.includes(role);
}

// EXECUTIVE_ROLES may view the company-wide Executive Overview. Mirrors the
// backend `executiveRolesAllowed` allowlist in internal/executive/handler.go —
// the company-wide (KindAll) roles. The server is the real gate.
export const EXECUTIVE_ROLES = ["super_admin", "regional_director", "auditor"];

export function canViewExecutive(role?: string): boolean {
  return !!role && EXECUTIVE_ROLES.includes(role);
}

// APPROVAL workflow (Module-3 3.5): the four-level hiring sign-off chain. Mirrors
// the backend approvalLevelRoles map in internal/applications/approval.go. Server
// is the real gate; these only decide what to render.
export const APPROVAL_LEVEL_ROLES: Record<number, string> = {
  1: "hr_staff",
  2: "hr_manager",
  3: "sgm",
  4: "regional_director",
};

// APPROVAL_ROLES may reach the Approvals queue (any chain role, plus super_admin).
export const APPROVAL_ROLES = ["super_admin", "hr_staff", "hr_manager", "sgm", "regional_director"];

export function canAccessApprovals(role?: string): boolean {
  return !!role && APPROVAL_ROLES.includes(role);
}

// roleLevel returns the chain level a role decides (0 if none / super_admin, which
// may decide any level).
export function roleLevel(role?: string): number {
  if (!role) return 0;
  for (const [lvl, r] of Object.entries(APPROVAL_LEVEL_ROLES)) {
    if (r === role) return Number(lvl);
  }
  return 0;
}

// canDecideApprovalLevel mirrors the backend per-level gate: the level's role, or
// super_admin (who may decide any level).
export function canDecideApprovalLevel(role: string | undefined, level: number): boolean {
  return role === "super_admin" || (!!role && APPROVAL_LEVEL_ROLES[level] === role);
}

// canSubmitApproval gates opening a request (the level-1 / Staff sign-off).
export function canSubmitApproval(role?: string): boolean {
  return role === "super_admin" || role === "hr_staff";
}

// OFFER_ROLES may compose/send offers. Mirrors the backend offerWriteRoles map in
// internal/applications/offer.go (hr_manager + super_admin).
export const OFFER_ROLES = ["super_admin", "hr_manager"];

export function canManageOffer(role?: string): boolean {
  return !!role && OFFER_ROLES.includes(role);
}

// LETTER_ROLES may generate PDF letters. Mirrors the backend letterWriteRoles map
// in internal/applications/letter.go (the candidate-managing HR roles).
export const LETTER_ROLES = ["super_admin", "hr_manager", "hr_staff", "sgm"];

export function canManageLetters(role?: string): boolean {
  return !!role && LETTER_ROLES.includes(role);
}

// HR_ROLES are the roles a local password account may hold. Mirrors the backend
// `allowedRoles` set in internal/hrauth/model.go; the label is what super_admins
// pick from when provisioning an account.
export const HR_ROLES: { value: string; label: string }[] = [
  { value: "super_admin", label: "Super admin — full access" },
  { value: "regional_director", label: "Regional director — all stores" },
  { value: "auditor", label: "Auditor — read-only, all stores" },
  { value: "operation_director", label: "Operation director — subregion" },
  { value: "sgm", label: "Store GM — single store" },
  { value: "hr_manager", label: "HR manager — single store" },
  { value: "hr_staff", label: "HR staff — single store" },
];

export function roleLabel(role: string): string {
  return HR_ROLES.find((r) => r.value === role)?.label.split(" — ")[0] ?? role;
}
