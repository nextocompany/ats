"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, ApiError, buildQuery } from "./api";
import type {
  AdminSettings,
  Application,
  ApplicationFilter,
  Candidate,
  FitAnalysis,
  Funnel,
  InterviewInviteResult,
  InterviewView,
  KPI,
  Me,
  OpenRole,
  ReportExport,
  SearchFilter,
  SearchHit,
  Source,
  StoreLoad,
  TimelineEntry,
} from "./types";

export function useMe() {
  return useQuery({ queryKey: ["me"], queryFn: () => api.get<Me>("/api/v1/users/me").then((r) => r.data) });
}

// Admin system settings (super_admin only). The query 403s for other roles, so
// gate the UI on me.role before rendering.
export function useAdminSettings(enabled = true) {
  return useQuery({
    queryKey: ["admin-settings"],
    queryFn: () => api.get<AdminSettings>("/api/v1/admin/settings").then((r) => r.data),
    enabled,
  });
}

export function useUpdateAdminSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (settings: AdminSettings) =>
      api.patch<AdminSettings>("/api/v1/admin/settings", settings).then((r) => r.data),
    onSuccess: (data) => qc.setQueryData(["admin-settings"], data),
  });
}

export function useApplications(filter: ApplicationFilter) {
  return useQuery({
    queryKey: ["applications", filter],
    queryFn: () =>
      api.get<Application[]>(
        "/api/v1/applications" +
          buildQuery({
            status: filter.status,
            min_score: filter.min_score,
            store_id: filter.store_id,
            source_channel: filter.source_channel,
            page: filter.page,
            limit: filter.limit,
          }),
      ),
  });
}

export function useApplication(id: string) {
  return useQuery({
    queryKey: ["application", id],
    queryFn: () => api.get<Application>(`/api/v1/applications/${id}`).then((r) => r.data),
    enabled: !!id,
  });
}

export function useResumeUrl(id: string) {
  return useQuery({
    queryKey: ["resume", id],
    queryFn: () => api.get<{ url: string }>(`/api/v1/applications/${id}/resume`).then((r) => r.data.url),
    enabled: !!id,
    staleTime: 10 * 60 * 1000,
    retry: false,
  });
}

export function useCandidates(page: number) {
  return useQuery({
    queryKey: ["candidates", page],
    queryFn: () => api.get<Candidate[]>("/api/v1/candidates" + buildQuery({ page })),
  });
}

export function useCandidate(id: string) {
  return useQuery({
    queryKey: ["candidate", id],
    queryFn: () =>
      api.get<{ candidate: Candidate; applications: Application[] }>(`/api/v1/candidates/${id}`).then((r) => r.data),
    enabled: !!id,
  });
}

export function useTimeline(id: string) {
  return useQuery({
    queryKey: ["timeline", id],
    queryFn: () => api.get<TimelineEntry[]>(`/api/v1/candidates/${id}/timeline`).then((r) => r.data),
    enabled: !!id,
  });
}

export function useFunnel() {
  return useQuery({ queryKey: ["funnel"], queryFn: () => api.get<Funnel>("/api/v1/reports/funnel").then((r) => r.data) });
}
export function useKpi() {
  return useQuery({ queryKey: ["kpi"], queryFn: () => api.get<KPI>("/api/v1/reports/kpi").then((r) => r.data) });
}
export function useSources() {
  return useQuery({ queryKey: ["sources"], queryFn: () => api.get<Source[]>("/api/v1/reports/sources").then((r) => r.data) });
}
export function useWaitingByStore() {
  return useQuery({ queryKey: ["by-store"], queryFn: () => api.get<StoreLoad[]>("/api/v1/reports/by-store").then((r) => r.data) });
}
export function useOpenRoles() {
  return useQuery({ queryKey: ["open-roles"], queryFn: () => api.get<OpenRole[]>("/api/v1/reports/open-roles").then((r) => r.data) });
}

export function useCandidateSearch(filter: SearchFilter) {
  return useQuery({
    queryKey: ["candidate-search", filter],
    queryFn: () =>
      api.get<SearchHit[]>(
        "/api/v1/candidates/search" +
          buildQuery({ q: filter.q, status: filter.status, page: filter.page, limit: filter.limit }),
      ),
    enabled: filter.q.trim().length > 0,
  });
}

export function useReportExports() {
  return useQuery({
    queryKey: ["report-exports"],
    queryFn: () => api.get<ReportExport[]>("/api/v1/reports/exports").then((r) => r.data),
  });
}

export function useTriggerExport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post("/api/v1/reports/exports"),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["report-exports"] }),
  });
}

export function useSetStatus(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (status: string) => api.patch(`/api/v1/applications/${id}/status`, { status }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["application", id] });
      qc.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}

// useInterview loads the AI pre-interview session for an application. A 404 (no
// interview yet) resolves to null rather than an error so the panel can render a
// neutral "not invited" state.
export function useInterview(id: string) {
  return useQuery({
    queryKey: ["interview-session", id],
    queryFn: async () => {
      try {
        return (await api.get<InterviewView>(`/api/v1/applications/${id}/interview`)).data;
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
    enabled: !!id,
    retry: false,
  });
}

// useInviteInterview triggers (or re-fetches, idempotently) the AI pre-interview
// for an application and refreshes the interview view.
export function useInviteInterview(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<InterviewInviteResult>(`/api/v1/applications/${id}/interview`).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["interview-session", id] }),
  });
}

// useFitAnalysis loads the cross-position fit analysis for an application. A 404
// (not generated yet) resolves to null so the panel can render a "generate" CTA.
export function useFitAnalysis(id: string) {
  return useQuery({
    queryKey: ["fit-analysis", id],
    queryFn: async () => {
      try {
        return (await api.get<{ analysis: FitAnalysis }>(`/api/v1/applications/${id}/fit-analysis`)).data.analysis;
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
    enabled: !!id,
    retry: false, // a 404 (not generated yet) is an expected, non-retryable state
  });
}

// useGenerateFitAnalysis runs (or re-runs) the AI cross-position fit analysis and
// refreshes the fit view. Surfaces the backend's 409 message when pre-conditions
// (scored + interview completed) are not met.
export function useGenerateFitAnalysis(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      api.post<{ analysis: FitAnalysis }>(`/api/v1/applications/${id}/fit-analysis`).then((r) => r.data.analysis),
    onSuccess: (analysis) => {
      // Seed the cache from the mutation response so the panel updates instantly,
      // then revalidate in the background.
      qc.setQueryData(["fit-analysis", id], analysis);
      void qc.invalidateQueries({ queryKey: ["fit-analysis", id] });
    },
  });
}

export function useBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { ids: string[]; action: string; value: string }) =>
      api.post("/api/v1/applications/bulk", vars),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
  });
}
