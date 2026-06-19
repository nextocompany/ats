"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { Download, ShieldAlert } from "lucide-react";
import { toast } from "sonner";

import { InitialChip } from "@/components/people/PeopleBits";
import { MemberStatusBadge } from "@/components/people/MemberStatusBadge";
import { MemberBulkBar } from "@/components/members/MemberBulkBar";
import { PageHeader } from "@/components/shell/PageHeader";
import { SummaryStrip } from "@/components/shell/SummaryStrip";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { downloadFile, buildQuery } from "@/lib/api";
import { useMe, useMembers, useMemberStats } from "@/lib/queries";
import { isMemberAdmin } from "@/lib/roles";
import type { Member } from "@/lib/types";

const LIMIT = 20;

type T = ReturnType<typeof useTranslations>;

function joinedAgo(iso: string, t: T, locale: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";
  const days = Math.floor((Date.now() - then) / 86400000);
  if (days < 1) return t("agoToday");
  if (days < 7) return t("agoDays", { n: days });
  if (days < 30) return t("agoWeeks", { n: Math.floor(days / 7) });
  return new Date(iso).toLocaleDateString(locale === "th" ? "th-TH" : "en-GB", {
    day: "numeric",
    month: "short",
    year: "2-digit",
  });
}

function ProviderChips({ m }: { m: Member }) {
  const ps = [
    m.line_linked && "LINE",
    m.google_linked && "Google",
    m.email_linked && "Email",
  ].filter(Boolean) as string[];
  if (ps.length === 0) return <span className="text-xs text-muted-foreground">—</span>;
  return (
    <span className="flex flex-wrap gap-1">
      {ps.map((p) => (
        <span key={p} className="rounded-full bg-brand-soft px-1.5 py-0.5 text-[10px] font-medium text-brand">
          {p}
        </span>
      ))}
    </span>
  );
}

function MembersInner() {
  const t = useTranslations("members");
  const locale = useLocale();
  const params = useSearchParams();
  const router = useRouter();
  const { data: me, isLoading: meLoading } = useMe();
  const allowed = isMemberAdmin(me);

  const search = params.get("search") ?? "";
  const provider = params.get("provider") ?? "";
  const status = params.get("status") ?? "";
  const tag = params.get("tag") ?? "";
  const page = Math.max(1, Number(params.get("page") ?? "1"));

  const [selected, setSelected] = useState<string[]>([]);
  const [exporting, setExporting] = useState(false);

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params.toString());
    if (value) next.set(key, value);
    else next.delete(key);
    if (key !== "page") next.delete("page");
    router.replace(`/members?${next.toString()}`);
    setSelected([]); // a filter/page change invalidates the current selection
  };

  const { data, isLoading, isError, error } = useMembers({
    search: search || undefined,
    provider: provider || undefined,
    status: status || undefined,
    tag: tag || undefined,
    page,
    limit: LIMIT,
  }, allowed && !meLoading);
  const { data: stats } = useMemberStats(allowed);

  const items = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / LIMIT));
  const allChecked = items.length > 0 && items.every((m) => selected.includes(m.id));

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));
  const toggleAll = () =>
    setSelected((s) => (items.every((m) => s.includes(m.id)) ? [] : items.map((m) => m.id)));

  const exportCsv = async () => {
    setExporting(true);
    try {
      const qs = buildQuery({ search, provider, status, tag });
      await downloadFile(`/api/v1/admin/members/export.csv${qs}`, "members.csv");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("csvFailed"));
    } finally {
      setExporting(false);
    }
  };

  if (meLoading) return <Skeleton className="h-40 w-full rounded-xl" />;

  if (!allowed) {
    return (
      <div className="settle space-y-8">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <section className="flex items-start gap-3 rounded-xl bg-card p-6 ring-1 ring-hairline">
          <ShieldAlert className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            {t.rich("restricted", {
              b: (chunks) => <span className="font-medium text-foreground">{chunks}</span>,
            })}
          </p>
        </section>
      </div>
    );
  }

  return (
    <div className="settle space-y-6">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        meta={<span className="tabular-nums">{t("metaCount", { count: total })}</span>}
        actions={
          <>
            <label className="flex flex-col gap-1 text-[0.6875rem] font-medium uppercase tracking-wide text-muted-foreground">
              {t("searchLabel")}
              <Input
                key={search}
                defaultValue={search}
                placeholder={t("searchPlaceholder")}
                className="w-48"
                onBlur={(e) => setParam("search", e.target.value.trim())}
                onKeyDown={(e) => {
                  if (e.key === "Enter") setParam("search", (e.target as HTMLInputElement).value.trim());
                }}
              />
            </label>
            <label className="flex flex-col gap-1 text-[0.6875rem] font-medium uppercase tracking-wide text-muted-foreground">
              {t("providerLabel")}
              <Select value={provider || "all"} onValueChange={(v) => setParam("provider", v && v !== "all" ? v : "")}>
                <SelectTrigger className="w-32" size="sm">
                  <SelectValue placeholder={t("all")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("all")}</SelectItem>
                  <SelectItem value="line">LINE</SelectItem>
                  <SelectItem value="google">Google</SelectItem>
                  <SelectItem value="email">Email</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="flex flex-col gap-1 text-[0.6875rem] font-medium uppercase tracking-wide text-muted-foreground">
              {t("statusLabel")}
              <Select value={status || "all"} onValueChange={(v) => setParam("status", v && v !== "all" ? v : "")}>
                <SelectTrigger className="w-32" size="sm">
                  <SelectValue placeholder={t("all")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("all")}</SelectItem>
                  <SelectItem value="active">{t("statusActive")}</SelectItem>
                  <SelectItem value="suspended">{t("statusSuspended")}</SelectItem>
                  <SelectItem value="anonymized">{t("statusAnonymized")}</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="flex flex-col gap-1 text-[0.6875rem] font-medium uppercase tracking-wide text-muted-foreground">
              {t("tagLabel")}
              <Input
                key={tag}
                defaultValue={tag}
                placeholder={t("tagFilterPlaceholder")}
                className="w-28"
                onBlur={(e) => setParam("tag", e.target.value.trim().toLowerCase())}
                onKeyDown={(e) => {
                  if (e.key === "Enter") setParam("tag", (e.target as HTMLInputElement).value.trim().toLowerCase());
                }}
              />
            </label>
            <Button variant="outline" size="sm" className="self-end" disabled={exporting} onClick={exportCsv}>
              <Download className="size-4" /> {t("exportCsv")}
            </Button>
          </>
        }
      />

      {stats && (
        <div className="space-y-3">
          <SummaryStrip
            stats={[
              { label: t("statTotal"), value: <span className="tabular-nums">{stats.total}</span>, lead: true, accent: true },
              { label: t("statActive"), value: <span className="tabular-nums">{stats.active}</span>, hint: t("statActiveHint") },
              { label: t("statWithApps"), value: <span className="tabular-nums">{stats.with_applications}</span>, hint: t("statWithAppsHint") },
              { label: t("statNew"), value: <span className="tabular-nums">{stats.new_this_week}</span>, hint: t("statNewHint") },
            ]}
          />
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span className="font-medium uppercase tracking-wide">{t("byProvider")}</span>
            {(["line", "google", "email"] as const).map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => setParam("provider", provider === p ? "" : p)}
                className="rounded-sm hover:text-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                aria-pressed={provider === p}
              >
                {p === "line" ? "LINE" : p === "google" ? "Google" : "Email"}{" "}
                <span className="tabular-nums text-foreground">{stats.by_provider?.[p] ?? 0}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {error instanceof Error ? error.message : t("loadFailed")}
        </div>
      )}

      {/* Mobile */}
      <ul className="space-y-2.5 md:hidden">
        {isLoading && Array.from({ length: 6 }).map((_, i) => (
          <li key={i} className="rounded-xl bg-card p-4 ring-1 ring-hairline"><Skeleton className="h-5 w-full" /></li>
        ))}
        {!isLoading && items.length === 0 && (
          <li className="rounded-xl bg-card px-5 py-12 text-center text-sm text-muted-foreground ring-1 ring-hairline">
            {t("empty")}
          </li>
        )}
        {items.map((m) => (
          <li key={m.id} className="rounded-xl bg-card ring-1 ring-hairline">
            <Link href={`/members/${m.id}`} className="flex items-start gap-3 p-4">
              <InitialChip name={m.full_name || m.email || "?"} />
              <div className="min-w-0 flex-1">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-foreground">{m.full_name || t("noName")}</p>
                    <p className="truncate text-xs text-muted-foreground">{m.email || m.phone || "—"}</p>
                  </div>
                  <MemberStatusBadge status={m.status} />
                </div>
                <div className="mt-2.5 flex flex-wrap items-center gap-2 border-t border-hairline pt-2.5">
                  <ProviderChips m={m} />
                  <span className="ml-auto text-xs text-muted-foreground tabular-nums">
                    {t("appsCount", { count: m.applications_count })} · {joinedAgo(m.created_at, t, locale)}
                  </span>
                </div>
              </div>
            </Link>
          </li>
        ))}
      </ul>

      {/* Desktop */}
      <div className="hidden overflow-hidden rounded-xl bg-card ring-1 ring-hairline md:block">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[820px] text-sm">
            <thead className="ledger-head sticky top-0 z-10 text-left">
              <tr>
                <th scope="col" className="w-10 pl-5 pr-2 py-3">
                  <Checkbox checked={allChecked} onCheckedChange={toggleAll} aria-label={t("selectAll")} />
                </th>
                <th scope="col" className="px-3 py-3">{t("colMember")}</th>
                <th scope="col" className="w-32 px-3 py-3">{t("colProvince")}</th>
                <th scope="col" className="w-40 px-3 py-3">{t("colProvider")}</th>
                <th scope="col" className="w-24 px-3 py-3">{t("colApplications")}</th>
                <th scope="col" className="w-28 px-3 py-3">{t("colStatus")}</th>
                <th scope="col" className="w-28 py-3 pl-3 pr-5">{t("colJoined")}</th>
              </tr>
            </thead>
            <tbody>
              {isLoading && Array.from({ length: 8 }).map((_, i) => (
                <tr key={i} className="border-b border-hairline last:border-0">
                  <td className="px-5 py-3.5" colSpan={7}><Skeleton className="h-5 w-full" /></td>
                </tr>
              ))}
              {!isLoading && items.length === 0 && (
                <tr><td colSpan={7} className="px-5 py-16 text-center text-muted-foreground">{t("empty")}</td></tr>
              )}
              {items.map((m) => (
                <tr key={m.id} data-sel={selected.includes(m.id)} className="ledger-row group border-b border-hairline last:border-0 data-[sel=true]:bg-brand-soft/30">
                  <td className="pl-5 pr-2 py-3.5">
                    <Checkbox
                      checked={selected.includes(m.id)}
                      onCheckedChange={() => toggle(m.id)}
                      aria-label={t("selectName", { name: m.full_name || m.email || "" })}
                    />
                  </td>
                  <td className="px-3 py-3.5">
                    <div className="flex items-center gap-3">
                      <InitialChip name={m.full_name || m.email || "?"} size="sm" />
                      <div className="min-w-0">
                        <Link
                          href={`/members/${m.id}`}
                          className="truncate font-medium text-foreground underline-offset-2 hover:text-brand hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
                        >
                          {m.full_name || t("noName")}
                        </Link>
                        <p className="truncate text-xs text-muted-foreground">{m.email || m.phone || "—"}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-3 py-3.5 text-muted-foreground">{m.province || "—"}</td>
                  <td className="px-3 py-3.5"><ProviderChips m={m} /></td>
                  <td className="px-3 py-3.5 tabular-nums text-muted-foreground">{m.applications_count}</td>
                  <td className="px-3 py-3.5"><MemberStatusBadge status={m.status} /></td>
                  <td className="py-3.5 pl-3 pr-5 text-muted-foreground" title={new Date(m.created_at).toLocaleString()}>
                    {joinedAgo(m.created_at, t, locale)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination page={page} pages={pages} onPage={(p) => setParam("page", String(p))} />

      <MemberBulkBar selected={selected} onDone={() => setSelected([])} />
    </div>
  );
}

export default function MembersPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <MembersInner />
    </Suspense>
  );
}
