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
