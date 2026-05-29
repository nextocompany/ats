"use client";

import { useMutation, useQuery } from "@tanstack/react-query";

import { api } from "./api";
import type { ApplyInput, ApplyResult, ApplicationStatus, PositionDetail, PublicPosition } from "./types";

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

export function useApplyMutation() {
  return useMutation<ApplyResult, Error, ApplyInput>({
    mutationFn: (input) =>
      api
        .postForm<ApplyResult>("/api/v1/public/apply", buildApplyForm(input), {
          "X-LINE-IdToken": input.lineIdToken,
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
