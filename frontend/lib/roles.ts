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
