"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { ArrowLeft, ShieldAlert } from "lucide-react";
import { toast } from "sonner";

import { InitialChip } from "@/components/people/PeopleBits";
import { MemberStatusBadge } from "@/components/people/MemberStatusBadge";
import { MemberActions } from "@/components/members/MemberActions";
import { NotesPanel } from "@/components/members/NotesPanel";
import { TagEditor } from "@/components/members/TagEditor";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
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

export default function MemberDetailPage() {
  const t = useTranslations("members");
  const locale = useLocale();
  const dateLocale = locale === "th" ? "th-TH" : "en-GB";
  const { id } = useParams<{ id: string }>();
  const { data: me, isLoading: meLoading } = useMe();
  const allowed = isMemberAdmin(me?.role);
  const { data: m, isLoading, isError } = useMember(allowed ? id : "");

  const viewResume = async () => {
    try {
      const { data } = await api.get<{ url: string }>(`/api/v1/admin/members/${id}/resume`);
      window.open(data.url, "_blank", "noopener,noreferrer");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("resumeOpenFailed"));
    }
  };

  if (meLoading) return <Skeleton className="h-96 w-full rounded-xl" />;
  if (!allowed) {
    return (
      <div className="settle flex items-start gap-3 rounded-xl bg-card p-6 ring-1 ring-hairline">
        <ShieldAlert className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">
          {t.rich("restricted", {
            b: (chunks) => <span className="font-medium text-foreground">{chunks}</span>,
          })}
        </p>
      </div>
    );
  }

  const providers = [
    m?.line_linked && "LINE",
    m?.google_linked && "Google",
    m?.email_linked && "Email",
  ].filter(Boolean) as string[];

  return (
    <div className="settle space-y-5">
      <Link
        href="/members"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
      >
        <ArrowLeft className="size-4" /> {t("back")}
      </Link>

      {isLoading && <Skeleton className="h-[60vh] w-full rounded-xl" />}
      {isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {t("notFound")}
        </div>
      )}

      {m && (
        <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
          <div className="space-y-6">
            <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
              <div className="flex items-start gap-4">
                <InitialChip name={m.full_name || m.email || "?"} size="lg" />
                <div className="min-w-0 flex-1">
                  <p className="eyebrow brass-underline inline-block">{t("memberEyebrow")}</p>
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
                <Row label={t("fieldEmail")} value={m.email ? `${m.email}${m.email_verified ? " ✓" : ""}` : "—"} />
                <Row label={t("fieldPhone")} value={m.phone || "—"} />
                <Row label={t("fieldProvince")} value={m.province || "—"} />
                <Row label={t("fieldLoginProviders")} value={providers.length ? providers.join(" · ") : "—"} />
                <Row
                  label={t("fieldResume")}
                  value={
                    m.has_resume ? (
                      <Button size="xs" variant="outline" onClick={viewResume}>
                        {t("viewResume", { type: m.resume_file_type || "file" })}
                      </Button>
                    ) : (
                      "—"
                    )
                  }
                />
                <Row
                  label={t("fieldPdpa")}
                  value={m.pdpa_consent ? t("pdpaConsented", { version: m.pdpa_version || "?" }) : t("pdpaNotConsented")}
                />
              </div>
            </section>

            <section>
              <h2 className="eyebrow mb-3">{t("applicationsHeading", { count: m.applications_count })}</h2>
              <div className="rounded-xl bg-card p-5 text-sm text-muted-foreground ring-1 ring-hairline">
                {m.applications_count === 0
                  ? t("noApplications")
                  : t("hasApplications", { count: m.applications_count })}
              </div>
            </section>

            <NotesPanel memberId={m.id} />
          </div>

          <aside aria-label="Account" className="space-y-6">
            <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
              <h2 className="eyebrow mb-3">{t("account")}</h2>
              <Row label={t("activeSessions")} value={<span className="tabular-nums">{m.active_sessions}</span>} />
              <Row
                label={t("lastLogin")}
                value={m.last_login_at ? new Date(m.last_login_at).toLocaleString(dateLocale) : "—"}
              />
            </div>
            <TagEditor memberId={m.id} />
            <MemberActions member={m} role={me?.role} />
          </aside>
        </div>
      )}
    </div>
  );
}
