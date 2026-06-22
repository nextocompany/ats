// Candidate membership API helpers (internal/candidateauth). Identity is the
// httpOnly session cookie sent by lib/api (credentials:'include'). OAuth flows
// (LINE/Google) are top-level navigations — see lib/line.ts.
import { api } from "./api";
import type { Account, AccountResume, Letter, Offer, OfferResponseInput, OnboardingStatus, ProfileInput } from "./types";

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

// reacceptConsent re-consents to the current notice version (Phase 2 reconsent).
export function reacceptConsent(): Promise<void> {
  return api.post("/api/v1/public/auth/consent/accept").then(() => undefined);
}

// updateProfile saves profile fields (+ optional PDPA consent).
export function updateProfile(input: ProfileInput): Promise<Account> {
  return api.patch<Account>("/api/v1/public/auth/profile", input).then((r) => r.data);
}

// RESUME_LIMIT mirrors the server cap (candidateauth.MaxResumes).
export const RESUME_LIMIT = 5;

// uploadResume adds a CV to the account's library and returns the updated history
// (newest first). Throws ApiError(409) when the library is already full.
export function uploadResume(file: File): Promise<AccountResume[]> {
  const form = new FormData();
  form.set("resume", file);
  return api
    .postForm<{ resumes: AccountResume[] }>("/api/v1/public/auth/resume", form)
    .then((r) => r.data.resumes);
}

// getResumes lists the account's CV history (newest first).
export function getResumes(): Promise<AccountResume[]> {
  return api.get<{ resumes: AccountResume[] }>("/api/v1/public/auth/resumes").then((r) => r.data.resumes);
}

// setDefaultResume marks one resume the default used for quick-apply.
export function setDefaultResume(id: string): Promise<AccountResume[]> {
  return api
    .post<{ resumes: AccountResume[] }>(`/api/v1/public/auth/resumes/${id}/default`)
    .then((r) => r.data.resumes);
}

// deleteResume removes a CV; deleting the default promotes the newest remaining.
export function deleteResume(id: string): Promise<AccountResume[]> {
  return api.del<{ resumes: AccountResume[] }>(`/api/v1/public/auth/resumes/${id}`).then((r) => r.data.resumes);
}

// getMyOffers lists the member's offers (Module-3 3.6).
export function getMyOffers(): Promise<Offer[]> {
  return api.get<Offer[]>("/api/v1/public/auth/offers").then((r) => r.data);
}

// respondToOffer accepts or declines an offer (decline requires a reason).
export function respondToOffer(id: string, input: OfferResponseInput): Promise<Offer> {
  return api.post<Offer>(`/api/v1/public/auth/offers/${id}/respond`, input).then((r) => r.data);
}

// getMyLetters lists the member's letters (interview/offer) with signed URLs.
export function getMyLetters(): Promise<Letter[]> {
  return api.get<Letter[]>("/api/v1/public/auth/letters").then((r) => r.data);
}

// getMyOnboarding returns the member's onboarding checklist + progress (Module-3
// 3.8). Throws ApiError(404) when there is no hired application / onboarding.
export function getMyOnboarding(): Promise<OnboardingStatus> {
  return api.get<OnboardingStatus>("/api/v1/public/auth/onboarding").then((r) => r.data);
}
