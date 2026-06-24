"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";

import { ScoreBadge } from "@/components/inbox/ScoreBadge";
import { InitialChip } from "@/components/people/PeopleBits";
import { MemberStatusBadge } from "@/components/people/MemberStatusBadge";
import { LockBar } from "@/components/candidates/LockBar";
import { MemberActions } from "@/components/members/MemberActions";
import { NotesPanel } from "@/components/members/NotesPanel";
import { TagEditor } from "@/components/members/TagEditor";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { useMe, useMember } from "@/lib/queries";
import { isMemberAdmin } from "@/lib/roles";

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-hairline py-2.5 last:border-0">
      <span className="text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{label}</span>
      <span className="min-w-0 truncate text-right text-sm text-foreground">{value}</span>
    </div>
  );
}

// The unified person detail (Phase 1 of the Candidates+Members unify): account
// identity + linked providers + the applications across every linked candidate
// row + (admin-only) CRM. The :id may be an account id (from the list) or a
// per-intake candidate id (from search/inbox) — the backend resolves both.
export default function CandidateProfilePage() {
  const t = useTranslations("members"); // account fields + CRM strings
  const tc = useTranslations("candidates"); // applications-list identity
  const locale = useLocale();
  const dateLocale = locale === "th" ? "th-TH" : "en-GB";
  const { id } = useParams<{ id: string }>();
  const { data: me } = useMe();
  const isAdmin = isMemberAdmin(me); // gates CRM (notes/tags/lifecycle/sessions/resume)
  const { data: m, isLoading, isError } = useMember(id);

  const viewResume = async () => {
    try {
      const { data } = await api.get<{ url: string }>(`/api/v1/admin/members/${m?.id}/resume`);
      window.open(data.url, "_blank", "noopener,noreferrer");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("resumeOpenFailed"));
    }
  };

  const providers = [
    m?.line_linked && "LINE",
    m?.google_linked && "Google",
    m?.email_linked && "Email",
  ].filter(Boolean) as string[];
  const applications = m?.applications ?? [];

  return (
    <div className="settle space-y-5">
      <Link
        href="/candidates"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
      >
        <ArrowLeft className="size-4" /> {tc("backToList")}
      </Link>

      {isLoading && <Skeleton className="h-[60vh] w-full rounded-xl" />}
      {isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {tc("detailNotFound")}
        </div>
      )}

      {m?.candidate_id && <LockBar candidateId={m.candidate_id} me={me} />}

      {m && (
        <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
          <div className="space-y-6">
            <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
              <div className="flex items-start gap-4">
                <InitialChip name={m.full_name || m.email || "?"} size="lg" />
                <div className="min-w-0 flex-1">
                  <p className="eyebrow brass-underline inline-block">{tc("colCandidate")}</p>
                  <h1 className="mt-3 font-heading text-2xl font-semibold tracking-tight">{m.full_name || t("noName")}</h1>
                  <div className="mt-2 flex items-center gap-2">
                    <MemberStatusBadge status={m.status} />
                    <span className="text-xs text-muted-foreground">
                      {t("joinedOn", {
                        date: new Date(m.created_at).toLocaleDateString(dateLocale, { day: "numeric", month: "short", year: "numeric" }),
                      })}
                    </span>
                  </div>
                </div>
              </div>

              <div className="mt-6">
                <Row label={t("fieldEmail")} value={m.email ? `${m.email}${m.email_verified ? " ✓" : ""}` : "-"} />
                <Row label={t("fieldPhone")} value={m.phone || "-"} />
                <Row label={t("fieldProvince")} value={m.province || "-"} />
                <Row label={t("fieldLoginProviders")} value={providers.length ? providers.join(" · ") : "-"} />
                {isAdmin && (
                  <Row
                    label={t("fieldResume")}
                    value={
                      m.has_resume ? (
                        <Button size="xs" variant="outline" onClick={viewResume}>
                          {t("viewResume", { type: m.resume_file_type || "file" })}
                        </Button>
                      ) : (
                        "-"
                      )
                    }
                  />
                )}
                <Row
                  label={t("fieldPdpa")}
                  value={m.pdpa_consent ? t("pdpaConsented", { version: m.pdpa_version || "?" }) : t("pdpaNotConsented")}
                />
              </div>
            </section>

            <section>
              <h2 className="eyebrow mb-3">{tc("applicationsCount", { count: applications.length })}</h2>
              <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
                {applications.length === 0 ? (
                  <p className="px-5 py-8 text-center text-sm text-muted-foreground">{tc("noApplications")}</p>
                ) : (
                  applications.map((a, i) => (
                    <Link
                      key={a.id}
                      href={`/applications/${a.id}`}
                      className={`flex items-center justify-between gap-3 px-5 py-3.5 transition-colors hover:bg-brand-soft/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
                        i > 0 ? "border-t border-hairline" : ""
                      }`}
                    >
                      <div className="flex min-w-0 items-center gap-3">
                        <ScoreBadge score={a.ai_score} />
                        <span className="truncate text-sm font-medium text-foreground">
                          {a.position_title || tc("appUntitled")}
                        </span>
                      </div>
                      <Badge variant="secondary" className="shrink-0 capitalize">{a.status}</Badge>
                    </Link>
                  ))
                )}
              </div>
            </section>

            {isAdmin && <NotesPanel memberId={m.id} />}
          </div>

          {isAdmin && (
            <aside aria-label="Account" className="space-y-6">
              <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
                <h2 className="eyebrow mb-3">{t("account")}</h2>
                <Row label={t("activeSessions")} value={<span className="tabular-nums">{m.active_sessions}</span>} />
                <Row
                  label={t("lastLogin")}
                  value={m.last_login_at ? new Date(m.last_login_at).toLocaleString(dateLocale) : "-"}
                />
              </div>
              <TagEditor memberId={m.id} />
              <MemberActions member={m} me={me} />
            </aside>
          )}
        </div>
      )}
    </div>
  );
}
