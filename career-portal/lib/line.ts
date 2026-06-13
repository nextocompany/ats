// LINE Login (OAuth 2.1 web flow). The candidate authenticates BEFORE filling the
// apply form — clicking the gate navigates to the backend /line/login entrypoint,
// which redirects to LINE (real) or bounces straight back (mock). On return the
// backend puts the id-token in the URL fragment (#line_id_token=…), which the apply
// page reads and sends as X-LINE-IdToken. The backend handles all secrets; the
// portal never sees the channel secret or talks to LINE directly.

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// lineLoginUrl builds the backend LINE-login entrypoint, returning the candidate
// to `returnUrl` (the apply page) after authentication.
export function lineLoginUrl(returnUrl: string): string {
  return `${API_BASE}/api/v1/public/line/login?return=${encodeURIComponent(returnUrl)}`;
}
