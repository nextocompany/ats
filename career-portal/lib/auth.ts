// Candidate membership API helpers (internal/candidateauth). Identity is the
// httpOnly session cookie sent by lib/api (credentials:'include'). OAuth flows
// (LINE/Google) are top-level navigations — see lib/line.ts.
import { api } from "./api";
import type { Account, ProfileInput } from "./types";

export { lineLoginUrl, googleLoginUrl } from "./line";

// startEmailOtp requests a one-time code. Always succeeds server-side
// (enumeration-safe), so the UI advances to the code step regardless.
export function startEmailOtp(email: string): Promise<void> {
  return api.post("/api/v1/public/auth/email/start", { email }).then(() => undefined);
}

// verifyEmailOtp exchanges the code for a session (cookie set by the response).
export function verifyEmailOtp(email: string, code: string): Promise<Account> {
  return api.post<Account>("/api/v1/public/auth/email/verify", { email, code }).then((r) => r.data);
}

// getMe returns the current account; throws ApiError(401) when logged out.
export function getMe(): Promise<Account> {
  return api.get<Account>("/api/v1/public/auth/me").then((r) => r.data);
}

// logout revokes the session and clears the cookie.
export function logout(): Promise<void> {
  return api.post("/api/v1/public/auth/logout").then(() => undefined);
}

// updateProfile saves profile fields (+ optional PDPA consent).
export function updateProfile(input: ProfileInput): Promise<Account> {
  return api.patch<Account>("/api/v1/public/auth/profile", input).then((r) => r.data);
}

// uploadResume stores the account's reusable resume.
export function uploadResume(file: File): Promise<Account> {
  const form = new FormData();
  form.set("resume", file);
  return api.postForm<Account>("/api/v1/public/auth/resume", form).then((r) => r.data);
}
