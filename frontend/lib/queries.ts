"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, buildQuery } from "./api";
import type {
  Application,
  ApplicationFilter,
  Candidate,
  Funnel,
  KPI,
  Me,
  ReportExport,
  SearchFilter,
  SearchHit,
  Source,
  TimelineEntry,
} from "./types";

export function useMe() {
  return useQuery({ queryKey: ["me"], queryFn: () => api.get<Me>("/api/v1/users/me").then((r) => r.data) });
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

export function useBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { ids: string[]; action: string; value: string }) =>
      api.post("/api/v1/applications/bulk", vars),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
  });
}
