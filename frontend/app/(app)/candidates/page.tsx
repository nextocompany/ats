"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";
import { useTranslations } from "next-intl";
import { MapPin, Users } from "lucide-react";

import { Pagination } from "@/components/ui/pagination";
import { InitialChip, SourceChip, StatusPill, toneForStatus } from "@/components/people/PeopleBits";
import { PageHeader } from "@/components/shell/PageHeader";
import { SummaryStrip } from "@/components/shell/SummaryStrip";
import { Skeleton } from "@/components/ui/skeleton";
import { useCandidates } from "@/lib/queries";
import type { Candidate } from "@/lib/types";

const LIMIT = 20;

function summarize(items: Candidate[], total: number) {
  const active = items.filter((c) => toneForStatus(c.status) === "pass").length;
  const provinces = new Set(items.map((c) => c.province).filter(Boolean)).size;
  const channels = new Set(items.map((c) => c.source_channel).filter(Boolean)).size;
  return { total, active, provinces, channels };
}

function CandidatesInner() {
  const t = useTranslations("candidates");
  const params = useSearchParams();
  const router = useRouter();
  const page = Math.max(1, Number(params.get("page") ?? "1"));
  const { data, isLoading } = useCandidates(page);

  const items = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / LIMIT));
  const s = summarize(items, total);

  return (
    <div className="settle space-y-6">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        meta={<span className="tabular-nums">{t("meta", { count: total })}</span>}
      />

      {/* Header summary strip — the roster never reads as a lonely single row. */}
      <SummaryStrip
        stats={[
          { label: t("sumOnFile"), value: <span className="tabular-nums">{total}</span>, lead: true, accent: true },
          { label: t("sumActive"), value: <span className="tabular-nums">{s.active}</span>, hint: t("sumActiveHint") },
          { label: t("sumProvinces"), value: <span className="tabular-nums">{s.provinces}</span>, hint: t("sumProvincesHint") },
          { label: t("sumChannels"), value: <span className="tabular-nums">{s.channels}</span>, hint: t("sumChannelsHint") },
        ]}
      />

      {/* Mobile (<768px) — stacked card-rows. No horizontal table overflow at
          320/390: each candidate is a two-line card (avatar + name/id, then
          province + status pill), tap-target sized and fully on-screen. */}
      <ul className="space-y-2.5 md:hidden">
        {isLoading &&
          Array.from({ length: 6 }).map((_, i) => (
            <li key={i} className="rounded-xl bg-card p-4 ring-1 ring-hairline">
              <div className="flex items-center gap-3">
                <Skeleton className="size-11 shrink-0 rounded-lg" />
                <div className="flex-1 space-y-2">
                  <Skeleton className="h-3.5 w-36" />
                  <Skeleton className="h-2.5 w-24" />
                </div>
              </div>
            </li>
          ))}
        {!isLoading && items.length === 0 && (
          <li className="rounded-xl bg-card px-5 py-16 text-center ring-1 ring-hairline">
            <span
              aria-hidden
              className="mx-auto mb-5 grid size-12 place-items-center rounded-2xl bg-brand-soft text-brand"
            >
              <Users className="size-6" strokeWidth={1.75} />
            </span>
            <p className="text-base font-semibold text-foreground">{t("emptyTitle")}</p>
            <p className="mx-auto mt-1.5 max-w-xs text-sm text-muted-foreground">{t("emptyBody")}</p>
            <span className="mx-auto mt-6 block h-px w-10 bg-hairline" aria-hidden />
          </li>
        )}
        {items.map((c) => (
          <li key={c.id} className="rounded-xl bg-card ring-1 ring-hairline">
            <Link
              href={`/candidates/${c.id}`}
              className="block rounded-xl p-4 outline-none transition-colors hover:bg-brand-soft/40 focus-visible:ring-2 focus-visible:ring-ring"
            >
              <div className="flex items-center gap-3">
                <InitialChip name={c.full_name} size="lg" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-semibold text-foreground">{c.full_name}</span>
                  {c.subregion && (
                    <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                      {c.subregion}
                    </span>
                  )}
                </span>
                <SourceChip channel={c.source_channel} />
              </div>
              <div className="mt-3 flex items-center justify-between border-t border-hairline pt-3">
                {c.province ? (
                  <span className="inline-flex items-center gap-1.5 text-sm text-foreground/80">
                    <MapPin className="size-3.5 text-muted-foreground" strokeWidth={1.75} />
                    {c.province}
                  </span>
                ) : (
                  <span className="text-sm text-muted-foreground">{t("noProvince")}</span>
                )}
                <StatusPill status={c.status} />
              </div>
            </Link>
          </li>
        ))}
      </ul>

      <div className="hidden overflow-hidden rounded-xl bg-card ring-1 ring-hairline md:block">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead className="ledger-head text-left">
              <tr>
                <th className="py-3 pl-5 pr-3">{t("colCandidate")}</th>
                <th className="px-3 py-3">{t("colLocation")}</th>
                <th className="px-3 py-3">{t("colSource")}</th>
                <th className="w-36 py-3 pl-3 pr-5 text-right">{t("colStatus")}</th>
              </tr>
            </thead>
            <tbody>
              {isLoading &&
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-hairline last:border-0">
                    <td colSpan={4} className="px-5 py-3.5">
                      <div className="flex items-center gap-3">
                        <Skeleton className="size-10 shrink-0 rounded-lg" />
                        <div className="flex-1 space-y-1.5">
                          <Skeleton className="h-3.5 w-40" />
                          <Skeleton className="h-2.5 w-24" />
                        </div>
                      </div>
                    </td>
                  </tr>
                ))}
              {!isLoading && items.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-5 py-20 text-center">
                    <span
                      aria-hidden
                      className="mx-auto mb-5 grid size-12 place-items-center rounded-2xl bg-brand-soft text-brand"
                    >
                      <Users className="size-6" strokeWidth={1.75} />
                    </span>
                    <p className="text-base font-semibold text-foreground">{t("emptyTitle")}</p>
                    <p className="mx-auto mt-1.5 max-w-sm text-sm text-muted-foreground">
                      {t("emptyBodyLong")}
                    </p>
                    <span className="mx-auto mt-6 block h-px w-10 bg-hairline" aria-hidden />
                  </td>
                </tr>
              )}
              {items.map((c) => (
                <tr key={c.id} className="ledger-row border-b border-hairline last:border-0">
                  <td className="py-3 pl-5 pr-3">
                    <Link
                      href={`/candidates/${c.id}`}
                      className="-my-1 flex items-center gap-3 rounded-md py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <InitialChip name={c.full_name} size="lg" />
                      <span className="min-w-0">
                        <span className="block truncate font-semibold text-foreground">
                          {c.full_name}
                        </span>
                        {c.subregion && (
                          <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                            {c.subregion}
                          </span>
                        )}
                      </span>
                    </Link>
                  </td>
                  <td className="px-3 py-3">
                    {c.province ? (
                      <span className="inline-flex items-center gap-1.5 text-sm text-foreground/80">
                        <MapPin className="size-3.5 text-muted-foreground" strokeWidth={1.75} />
                        {c.province}
                      </span>
                    ) : (
                      <span className="text-sm text-muted-foreground">-</span>
                    )}
                  </td>
                  <td className="px-3 py-3">
                    <SourceChip channel={c.source_channel} />
                  </td>
                  <td className="py-3 pl-3 pr-5 text-right">
                    <StatusPill status={c.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination page={page} pages={pages} onPage={(p) => router.replace(`/candidates?page=${p}`)} />
    </div>
  );
}

export default function CandidatesPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <CandidatesInner />
    </Suspense>
  );
}
