"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, ApiError, buildQuery } from "./api";
import type {
  AdminSettings,
  Application,
  ApplicationFilter,
  ApprovalDecisionInput,
  ApprovalQueueItem,
  ApprovalRequest,
  BulkIntakeResult,
  Candidate,
  CreateHRUserInput,
  ExecutiveOverview,
  FitAnalysis,
  Funnel,
  HRUser,
  InterviewAppointment,
  StoreOption,
  InterviewFeedback,
  InterviewFeedbackInput,
  InterviewInviteResult,
  InterviewView,
  KPI,
  Letter,
  LetterType,
  Me,
  Member,
  Offer,
  OfferInput,
  OnboardingDoc,
  OnboardingStatus,
  MemberFilter,
  MemberNote,
  MemberStats,
  OpenRole,
  Position,
  Requisition,
  RequisitionFilter,
  RequisitionInput,
  AtsReport,
  RbacPermission,
  RbacRole,
  RbacRoleInput,
  ReportExport,
  ScorecardSummary,
  SearchFilter,
  SearchHit,
  ShortlistItem,
  Source,
  StoreLoad,
  TimelineEntry,
  UpdateHRUserInput,
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

// HR user accounts (super_admin only — the list 403s for other roles, so gate the
// UI on me.role first). These are the local username/password accounts that sign
// in alongside Entra SSO.
export function useHRUsers(enabled = true) {
  return useQuery({
    queryKey: ["hr-users"],
    queryFn: () => api.get<HRUser[]>("/api/v1/admin/users").then((r) => r.data ?? []),
    enabled,
  });
}

export function useCreateHRUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateHRUserInput) =>
      api.post<HRUser>("/api/v1/admin/users", input).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["hr-users"] }),
  });
}

export function useUpdateHRUser() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: UpdateHRUserInput }) =>
      api.patch<HRUser>(`/api/v1/admin/users/${id}`, input).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["hr-users"] }),
  });
}

// --- Dynamic RBAC (super_admin role/permission management) ---

export function useRbacPermissions(enabled = true) {
  return useQuery({
    queryKey: ["rbac", "permissions"],
    queryFn: () => api.get<RbacPermission[]>("/api/v1/admin/rbac/permissions").then((r) => r.data ?? []),
    enabled,
  });
}

export function useRbacRoles(enabled = true) {
  return useQuery({
    queryKey: ["rbac", "roles"],
    queryFn: () => api.get<RbacRole[]>("/api/v1/admin/rbac/roles").then((r) => r.data ?? []),
    enabled,
  });
}

// invalidateRbac refreshes the role list AND the caller's own permissions (a
// matrix edit can change what the current user may do/see).
function invalidateRbac(qc: ReturnType<typeof useQueryClient>) {
  qc.invalidateQueries({ queryKey: ["rbac"] });
  qc.invalidateQueries({ queryKey: ["me"] });
}

export function useCreateRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: RbacRoleInput) =>
      api.post<RbacRole>("/api/v1/admin/rbac/roles", input).then((r) => r.data),
    onSuccess: () => invalidateRbac(qc),
  });
}

export function useUpdateRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ key, input }: { key: string; input: RbacRoleInput }) =>
      api.patch<RbacRole>(`/api/v1/admin/rbac/roles/${key}`, input).then((r) => r.data),
    onSuccess: () => invalidateRbac(qc),
  });
}

export function useDeleteRole() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (key: string) =>
      api.del<{ deleted: string }>(`/api/v1/admin/rbac/roles/${key}`).then((r) => r.data),
    onSuccess: () => invalidateRbac(qc),
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

// usePositions loads the active positions for the bulk-upload picker.
export function usePositions() {
  return useQuery({
    queryKey: ["positions"],
    queryFn: () => api.get<Position[]>("/api/v1/positions").then((r) => r.data),
  });
}

// useBulkIntake uploads many resume files for one position. Builds the multipart
// body (position_id + repeated resumes) and refreshes the inbox on success.
export function useBulkIntake() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { positionId: string; files: File[] }) => {
      const form = new FormData();
      form.append("position_id", vars.positionId);
      for (const f of vars.files) form.append("resumes", f);
      return api.postForm<BulkIntakeResult>("/api/v1/applications/bulk-intake", form).then((r) => r.data);
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
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

// Executive overview (company-wide). 403s for non-leadership roles, so gate the
// page on canViewExecutive(me.role) before rendering.
export function useExecutiveOverview(enabled = true) {
  return useQuery({
    queryKey: ["executive-overview"],
    queryFn: () => api.get<ExecutiveOverview>("/api/v1/executive/overview").then((r) => r.data),
    enabled,
  });
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

// useAtsReport loads the RBAC-scoped ATS report for a date range (Module-3 3.9).
// A 403 (role not allowed) resolves to null so the page can show "not available".
export function useAtsReport(from: string, to: string) {
  return useQuery({
    queryKey: ["ats-report", from, to],
    queryFn: () =>
      api
        .get<AtsReport>("/api/v1/reports/ats" + buildQuery({ from, to }))
        .then((r) => r.data)
        .catch((e) => {
          if (e instanceof ApiError && e.status === 403) return null;
          throw e;
        }),
    retry: false,
  });
}

// useSetStatus changes the application status. `reason` is required by the backend
// when status="rejected" (stored internally, never sent to the candidate).
export function useSetStatus(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { status: string; reason?: string }) =>
      api.patch(`/api/v1/applications/${id}/status`, vars),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["application", id] });
      qc.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}

// useScheduleInterview books a human interview (status → interview). For an online
// interview the backend creates a Teams meeting + emails the candidate an invite.
export function useScheduleInterview(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { scheduled_at: string; duration_min: number; mode: "onsite" | "online"; location_text?: string }) =>
      api.post<InterviewAppointment>(`/api/v1/applications/${id}/interview-schedule`, vars).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["application", id] });
      qc.invalidateQueries({ queryKey: ["applications"] });
      qc.invalidateQueries({ queryKey: ["interview-appointments", id] });
    },
  });
}

// useStores loads the store directory for the placement/reassign picker.
export function useStores() {
  return useQuery({
    queryKey: ["stores"],
    queryFn: () => api.get<StoreOption[]>("/api/v1/stores").then((r) => r.data),
    staleTime: 5 * 60_000, // reference data — rarely changes
  });
}

// useReassign manually (re)assigns an application to a store, or moves it to the
// central pool ({ talent_pool: true }). Refreshes the application + inbox + journey.
export function useReassign(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { store_no: number } | { talent_pool: true }) =>
      api.patch(`/api/v1/applications/${id}/assignment`, vars),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["application", id] });
      qc.invalidateQueries({ queryKey: ["applications"] });
      qc.invalidateQueries({ queryKey: ["interview-appointments", id] });
    },
  });
}

// useInterviewAppointments loads every scheduled human-interview round for an
// application, ordered by round number.
export function useInterviewAppointments(id: string) {
  return useQuery({
    queryKey: ["interview-appointments", id],
    queryFn: () =>
      api.get<InterviewAppointment[]>(`/api/v1/applications/${id}/interview-appointments`).then((r) => r.data),
    enabled: !!id,
  });
}

// useInterviewFeedback loads the structured interview feedback entries (newest
// first) recorded by the hiring panel for an application.
export function useInterviewFeedback(id: string) {
  return useQuery({
    queryKey: ["interview-feedback", id],
    queryFn: () =>
      api.get<InterviewFeedback[]>(`/api/v1/applications/${id}/interview-feedback`).then((r) => r.data),
    enabled: !!id,
  });
}

// useAddInterviewFeedback records a new scorecard entry (TA or LM perspective) and
// refreshes the list + aggregate. Write access is server-gated per perspective.
export function useAddInterviewFeedback(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: InterviewFeedbackInput) =>
      api.post<InterviewFeedback>(`/api/v1/applications/${id}/interview-feedback`, vars).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["interview-feedback", id] });
      qc.invalidateQueries({ queryKey: ["scorecard-summary", id] });
      qc.invalidateQueries({ queryKey: ["shortlist"] });
    },
  });
}

// useScorecardSummary loads the combined TA + Line-Manager aggregate + composite.
export function useScorecardSummary(id: string) {
  return useQuery({
    queryKey: ["scorecard-summary", id],
    queryFn: () =>
      api.get<ScorecardSummary>(`/api/v1/applications/${id}/scorecard-summary`).then((r) => r.data),
    enabled: !!id,
  });
}

// useShortlist loads the Line Manager's Top-N shortlist (store-scoped server-side).
export function useShortlist(limit = 5) {
  return useQuery({
    queryKey: ["shortlist", limit],
    queryFn: () =>
      api.get<ShortlistItem[]>(`/api/v1/shortlist?limit=${limit}`).then((r) => r.data),
  });
}

// --- Approval workflow (Module-3 3.5) ---------------------------------------

// useApprovalForApplication loads the hiring approval request for an application.
// The backend returns null (200) when none has been opened, so the panel can show
// a "submit" CTA. queryKey is shared by the submit/decide mutations for refresh.
export function useApprovalForApplication(appId: string) {
  return useQuery({
    queryKey: ["approval", appId],
    queryFn: () =>
      api.get<ApprovalRequest | null>(`/api/v1/applications/${appId}/approval-request`).then((r) => r.data),
    enabled: !!appId,
  });
}

// useApprovalQueue loads the in-flight approvals awaiting the caller's decision
// level. Pass enabled=false to skip the call for roles that cannot approve.
export function useApprovalQueue(enabled = true) {
  return useQuery({
    queryKey: ["approval-queue"],
    queryFn: () => api.get<ApprovalQueueItem[]>("/api/v1/approvals").then((r) => r.data),
    enabled,
  });
}

// useSubmitApproval opens the approval chain for an interviewed application (the
// Staff-level sign-off). Refreshes the application + its approval state.
export function useSubmitApproval(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<ApprovalRequest>(`/api/v1/applications/${appId}/approval-request`).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["approval", appId] });
      qc.invalidateQueries({ queryKey: ["application", appId] });
      qc.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}

// useDecideApproval records an approve/reject on the active level and refreshes the
// approval state, the queue, and the application.
export function useDecideApproval(requestId: string, appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: ApprovalDecisionInput) =>
      api.post<ApprovalRequest>(`/api/v1/approval-requests/${requestId}/decide`, vars).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["approval", appId] });
      qc.invalidateQueries({ queryKey: ["approval-queue"] });
      qc.invalidateQueries({ queryKey: ["application", appId] });
      qc.invalidateQueries({ queryKey: ["applications"] });
    },
  });
}

// --- Offer management (Module-3 3.6) ----------------------------------------

// useOffer loads an application's offer (backend returns null when none exists).
export function useOffer(appId: string) {
  return useQuery({
    queryKey: ["offer", appId],
    queryFn: () => api.get<Offer | null>(`/api/v1/applications/${appId}/offer`).then((r) => r.data),
    enabled: !!appId,
  });
}

// useCreateOffer composes a draft offer; useUpdateOffer edits a still-draft offer.
export function useCreateOffer(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: OfferInput) => api.post<Offer>(`/api/v1/applications/${appId}/offer`, vars).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["offer", appId] }),
  });
}

export function useUpdateOffer(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: OfferInput) => api.patch<Offer>(`/api/v1/applications/${appId}/offer`, vars).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["offer", appId] }),
  });
}

// useSendOffer transitions a draft offer to sent and notifies the candidate.
export function useSendOffer(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post<Offer>(`/api/v1/applications/${appId}/offer/send`).then((r) => r.data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["offer", appId] });
      qc.invalidateQueries({ queryKey: ["application", appId] });
    },
  });
}

// --- Letters (Module-3 3.3) -------------------------------------------------

// useLetters loads an application's generated letters (each with a signed URL).
export function useLetters(appId: string) {
  return useQuery({
    queryKey: ["letters", appId],
    queryFn: () => api.get<Letter[]>(`/api/v1/applications/${appId}/letters`).then((r) => r.data),
    enabled: !!appId,
  });
}

// useGenerateLetter renders + stores an interview/offer letter and refreshes the list.
export function useGenerateLetter(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (type: LetterType) =>
      api.post<Letter>(`/api/v1/applications/${appId}/letters`, { type }).then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["letters", appId] }),
  });
}

// --- Onboarding documents (Module-3 3.8) ------------------------------------

// useOnboarding loads an application's onboarding checklist + progress. A 404
// (the application is not hired / no checklist) resolves to null rather than error.
export function useOnboarding(appId: string) {
  return useQuery({
    queryKey: ["onboarding", appId],
    queryFn: () =>
      api
        .get<OnboardingStatus>(`/api/v1/applications/${appId}/onboarding`)
        .then((r) => r.data)
        .catch((e) => {
          if (e instanceof ApiError && e.status === 404) return null;
          throw e;
        }),
    enabled: !!appId,
    retry: false,
  });
}

// useReviewOnboardingDoc approves/rejects a single document and refreshes the list.
export function useReviewOnboardingDoc(appId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { docId: string; decision: "approve" | "reject"; reason?: string }) =>
      api
        .post<OnboardingDoc>(`/api/v1/applications/${appId}/onboarding/documents/${vars.docId}/review`, {
          decision: vars.decision,
          reason: vars.reason,
        })
        .then((r) => r.data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["onboarding", appId] }),
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

// useMembers loads the career-portal member directory (paginated). The query fn
// keeps the full {data, meta} wrapper (no .then unwrap) so the page can read
// meta.total — same convention as useApplications. Pass enabled=false to skip the
// call for non-admin users (avoids a spurious 403 in the query cache).
export function useMembers(filter: MemberFilter, enabled = true) {
  return useQuery({
    queryKey: ["members", filter],
    queryFn: () =>
      api.get<Member[]>(
        "/api/v1/admin/members" +
          buildQuery({
            search: filter.search,
            provider: filter.provider,
            status: filter.status,
            tag: filter.tag,
            has_resume: filter.has_resume === undefined ? undefined : String(filter.has_resume),
            page: filter.page,
            limit: filter.limit,
          }),
      ),
    enabled,
  });
}

export function useMember(id: string) {
  return useQuery({
    queryKey: ["member", id],
    queryFn: () => api.get<Member>(`/api/v1/admin/members/${id}`).then((r) => r.data),
    enabled: !!id,
  });
}

export function useMemberStats(enabled = true) {
  return useQuery({
    queryKey: ["member-stats"],
    queryFn: () => api.get<MemberStats>("/api/v1/admin/members/stats").then((r) => r.data),
    enabled,
  });
}

export function useBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { ids: string[]; action: string; value: string; reason?: string }) =>
      api.post("/api/v1/applications/bulk", vars),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["applications"] }),
  });
}

// invalidateMember refreshes a member's detail plus the directory/stats so the
// list badges and summary strip reflect a lifecycle change immediately.
function invalidateMember(qc: ReturnType<typeof useQueryClient>, id: string) {
  void qc.invalidateQueries({ queryKey: ["member", id] });
  void qc.invalidateQueries({ queryKey: ["members"] });
  void qc.invalidateQueries({ queryKey: ["member-stats"] });
}

// useSetMemberStatus suspends ('suspended') or reactivates ('active') a member.
// Suspending also force-logs-out the account server-side.
export function useSetMemberStatus(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (status: "active" | "suspended") =>
      api.patch(`/api/v1/admin/members/${id}/status`, { status }),
    onSuccess: () => invalidateMember(qc, id),
  });
}

// useForceLogout revokes every active session for the member (without changing status).
export function useForceLogout(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(`/api/v1/admin/members/${id}/force-logout`),
    onSuccess: () => invalidateMember(qc, id),
  });
}

// useUpdateMember applies a sparse admin profile edit.
export function useUpdateMember(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (fields: Partial<Pick<Member, "full_name" | "phone" | "province" | "email">>) =>
      api.patch<Member>(`/api/v1/admin/members/${id}`, fields).then((r) => r.data),
    onSuccess: () => invalidateMember(qc, id),
  });
}

// useAnonymizeMember runs the irreversible PDPA erasure (super_admin only — the
// server enforces the role; this hook is used behind a super_admin-gated button).
export function useAnonymizeMember(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.post(`/api/v1/admin/members/${id}/anonymize`),
    onSuccess: () => invalidateMember(qc, id),
  });
}

// ── CRM (Phase C): notes, tags, bulk, CSV export ────────────────────────────

export function useMemberNotes(id: string) {
  return useQuery({
    queryKey: ["member-notes", id],
    queryFn: () => api.get<MemberNote[]>(`/api/v1/admin/members/${id}/notes`).then((r) => r.data),
    enabled: !!id,
  });
}

export function useAddNote(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: string) => api.post<MemberNote>(`/api/v1/admin/members/${id}/notes`, { body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["member-notes", id] }),
  });
}

export function useMemberTags(id: string) {
  return useQuery({
    queryKey: ["member-tags", id],
    queryFn: () => api.get<string[]>(`/api/v1/admin/members/${id}/tags`).then((r) => r.data),
    enabled: !!id,
  });
}

export function useAddTag(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (tag: string) => api.post(`/api/v1/admin/members/${id}/tags`, { tag }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["member-tags", id] });
      void qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

export function useRemoveTag(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (tag: string) => api.del(`/api/v1/admin/members/${id}/tags?tag=${encodeURIComponent(tag)}`),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["member-tags", id] });
      void qc.invalidateQueries({ queryKey: ["members"] });
    },
  });
}

// useMemberBulk applies one action (tag/suspend/reactivate) to many members.
export function useMemberBulk() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { ids: string[]; action: string; value?: string }) =>
      api.post<{ updated: number; failed: number }>("/api/v1/admin/members/bulk", vars).then((r) => r.data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["members"] });
      void qc.invalidateQueries({ queryKey: ["member-stats"] });
    },
  });
}

// ── Requisitions (manage position openings) ─────────────────────────────────

// useRequisitions keeps the {data, meta} wrapper so the page reads meta.total.
export function useRequisitions(filter: RequisitionFilter, enabled = true) {
  return useQuery({
    queryKey: ["requisitions", filter],
    queryFn: () =>
      api.get<Requisition[]>(
        "/api/v1/requisitions" +
          buildQuery({
            status: filter.status,
            store_id: filter.store_id === undefined ? undefined : String(filter.store_id),
            position_id: filter.position_id,
            page: filter.page,
            limit: filter.limit,
          }),
      ),
    enabled,
  });
}

function invalidateRequisitions(qc: ReturnType<typeof useQueryClient>) {
  void qc.invalidateQueries({ queryKey: ["requisitions"] });
}

export function useCreateRequisition() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: RequisitionInput) =>
      api.post<Requisition>("/api/v1/requisitions", input).then((r) => r.data),
    onSuccess: () => invalidateRequisitions(qc),
  });
}

export function useUpdateRequisition() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: RequisitionInput }) =>
      api.patch<Requisition>(`/api/v1/requisitions/${id}`, input).then((r) => r.data),
    onSuccess: () => invalidateRequisitions(qc),
  });
}

export function useApproveRequisition() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api.post<Requisition>(`/api/v1/requisitions/${id}/approve`).then((r) => r.data),
    onSuccess: () => invalidateRequisitions(qc),
  });
}

export function useCloseRequisition() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api.post<Requisition>(`/api/v1/requisitions/${id}/close`).then((r) => r.data),
    onSuccess: () => invalidateRequisitions(qc),
  });
}
