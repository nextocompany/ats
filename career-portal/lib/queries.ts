"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, ApiError } from "./api";
import { getMyLetters, getMyOffers, getMyOnboarding, respondToOffer } from "./auth";
import type {
  ApplyInput,
  ApplyResult,
  ApplicationStatus,
  InterviewSessionState,
  Letter,
  Offer,
  OfferResponseInput,
  OnboardingStatus,
  PortalApplication,
  PositionDetail,
  PublicPosition,
  QuickApplyResult,
} from "./types";

export const MY_OFFERS_KEY = ["my-offers"] as const;

// useMyOffers loads the logged-in member's offers (Module-3 3.6).
export function useMyOffers() {
  return useQuery<Offer[]>({ queryKey: MY_OFFERS_KEY, queryFn: getMyOffers });
}

// useRespondOffer accepts/declines an offer and refreshes the list + session.
export function useRespondOffer(id: string) {
  const qc = useQueryClient();
  return useMutation<Offer, Error, OfferResponseInput>({
    mutationFn: (input) => respondToOffer(id, input),
    onSuccess: () => qc.invalidateQueries({ queryKey: MY_OFFERS_KEY }),
  });
}

// useMyLetters loads the member's letters (interview/offer) with signed URLs.
export function useMyLetters() {
  return useQuery<Letter[]>({ queryKey: ["my-letters"], queryFn: getMyLetters });
}

export const MY_ONBOARDING_KEY = ["my-onboarding"] as const;

// useMyOnboarding loads the member's onboarding checklist + progress (Module-3
// 3.8). A 404 (not hired / no onboarding) resolves to null — a normal state for
// most members — so the section can simply render nothing.
export function useMyOnboarding() {
  return useQuery<OnboardingStatus | null>({
    queryKey: MY_ONBOARDING_KEY,
    queryFn: () =>
      getMyOnboarding().catch((e) => {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }),
    retry: false,
  });
}

// buildOnboardingDocForm assembles the multipart body the backend expects. Kept
// pure and exported so the FormData shape can be unit-tested without a network call.
export function buildOnboardingDocForm(docType: string, file: File): FormData {
  const form = new FormData();
  form.set("doc_type", docType);
  form.set("document", file);
  return form;
}

// useUploadOnboardingDoc uploads (or replaces) one required document and refreshes
// the checklist from the response.
export function useUploadOnboardingDoc() {
  const qc = useQueryClient();
  return useMutation<OnboardingStatus, Error, { docType: string; file: File }>({
    mutationFn: ({ docType, file }) =>
      api
        .postForm<OnboardingStatus>("/api/v1/public/auth/onboarding/documents", buildOnboardingDocForm(docType, file))
        .then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: MY_ONBOARDING_KEY }),
  });
}

export function usePublicPositions() {
  return useQuery({
    queryKey: ["public-positions"],
    queryFn: () => api.get<PublicPosition[]>("/api/v1/public/positions").then((r) => r.data),
  });
}

export function usePublicPosition(id: string) {
  return useQuery({
    queryKey: ["public-position", id],
    queryFn: () => api.get<PositionDetail>(`/api/v1/public/positions/${id}`).then((r) => r.data),
    enabled: !!id,
  });
}

// buildApplyForm assembles the multipart body the backend expects. Kept pure and
// exported so the FormData shape can be unit-tested without a network call.
// Identity travels in the session cookie (account-first) — no LINE header.
export function buildApplyForm(input: ApplyInput): FormData {
  const form = new FormData();
  form.set("position_id", input.positionId);
  form.set("full_name", input.fullName);
  if (input.phone) form.set("phone", input.phone);
  if (input.email) form.set("email", input.email);
  if (input.idCard) form.set("id_card", input.idCard);
  if (input.province) form.set("province", input.province);
  form.set("consent_given", "true");
  form.set("consent_version", input.consentVersion);
  form.set("resume", input.resume);
  return form;
}

// useApplyMutation submits the apply form with a (possibly new) resume. The
// session cookie identifies the member.
export function useApplyMutation() {
  return useMutation<ApplyResult, Error, ApplyInput>({
    mutationFn: (input) =>
      api.postForm<ApplyResult>("/api/v1/public/apply", buildApplyForm(input)).then((r) => r.data),
  });
}

// useQuickApply applies to a position using the member's saved profile + resume.
// consentGiven is sent for members who haven't already consented (the backend
// records + persists it on this apply); harmless when they consented at signup.
export function useQuickApply() {
  return useMutation<QuickApplyResult, Error, { positionId: string; consentGiven?: boolean }>({
    mutationFn: ({ positionId, consentGiven }) =>
      api
        .post<QuickApplyResult>("/api/v1/public/apply/quick", {
          position_id: positionId,
          consent_given: consentGiven ?? false,
        })
        .then((r) => r.data),
  });
}

export function useStatus(token: string) {
  return useQuery({
    queryKey: ["public-status", token],
    queryFn: () =>
      api.get<ApplicationStatus>(`/api/v1/public/status/${encodeURIComponent(token)}`).then((r) => r.data),
    enabled: !!token,
    retry: false,
  });
}

// useMyApplications loads the logged-in member's own application history
// (session-cookie auth). enabled lets the caller defer until auth is known.
export function useMyApplications(enabled = true) {
  return useQuery<PortalApplication[]>({
    queryKey: ["my-applications"],
    queryFn: () => api.get<PortalApplication[]>("/api/v1/public/me/applications").then((r) => r.data),
    enabled,
  });
}

// useInterviewSession loads (and on first open, seeds) the AI pre-interview chat
// for the given access token.
export function useInterviewSession(token: string) {
  return useQuery({
    queryKey: ["interview", token],
    queryFn: () =>
      api.get<InterviewSessionState>(`/api/v1/public/interview/${encodeURIComponent(token)}`).then((r) => r.data),
    enabled: !!token,
    retry: false,
    // The local chat state is authoritative once seeded; avoid focus refetches
    // that would be discarded anyway.
    staleTime: Infinity,
  });
}

// useInterviewRespond submits the candidate's answer and returns the updated
// conversation (next AI question, or the completed state).
export function useInterviewRespond(token: string) {
  return useMutation<InterviewSessionState, Error, string>({
    mutationFn: (content) =>
      api
        .post<InterviewSessionState>(`/api/v1/public/interview/${encodeURIComponent(token)}/message`, { content })
        .then((r) => r.data),
  });
}
