// Lightweight dev auth: a session cookie gates the UI. The Go API trusts a mock
// super_admin in development, so this is purely UI gating. Real Azure AD SSO
// (NextAuth/Entra) is a later sprint.
export const SESSION_COOKIE = "hr_session";

export function signIn() {
  // 12h dev session.
  document.cookie = `${SESSION_COOKIE}=dev; path=/; max-age=43200; samesite=lax`;
}

export function signOut() {
  document.cookie = `${SESSION_COOKIE}=; path=/; max-age=0; samesite=lax`;
}
