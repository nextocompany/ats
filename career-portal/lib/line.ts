// OAuth entrypoints for candidate membership. Clicking these navigates the
// browser (top-level) to the backend, which redirects to LINE/Google (real) or
// bounces straight back (mock). On return the backend has set the httpOnly session
// cookie (account-first) and redirects to `returnUrl`. The backend owns all
// secrets; the portal never talks to LINE/Google directly.

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// lineLoginUrl builds the LINE login/link entrypoint. mode="link" attaches LINE
// to the already-logged-in account (for email/Google signups) so push works.
export function lineLoginUrl(returnUrl: string, mode?: "link"): string {
  const q = new URLSearchParams({ return: returnUrl });
  if (mode) q.set("mode", mode);
  return `${API_BASE}/api/v1/public/line/login?${q.toString()}`;
}

// googleLoginUrl builds the Google login entrypoint.
export function googleLoginUrl(returnUrl: string): string {
  return `${API_BASE}/api/v1/public/auth/google/login?return=${encodeURIComponent(returnUrl)}`;
}
