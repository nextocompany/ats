"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Search as SearchIcon, MapPin } from "lucide-react";

import { Pagination } from "@/components/ui/pagination";
import { ScoreBadge } from "@/components/inbox/ScoreBadge";
import { InitialChip, StatusPill } from "@/components/people/PeopleBits";
import { PageHeader } from "@/components/shell/PageHeader";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useCandidateSearch } from "@/lib/queries";

// Province quick-filters — a signature lookup affordance for a national roster.
const PROVINCES = ["กรุงเทพมหานคร", "เชียงใหม่", "ขอนแก่น", "ชลบุรี", "ภูเก็ต", "สงขลา"] as const;

const LIMIT = 20;
const DEBOUNCE_MS = 300;

function SearchInner() {
  const t = useTranslations("search");
  const params = useSearchParams();
  const router = useRouter();

  const q = params.get("q") ?? "";
  const page = Math.max(1, Number(params.get("page") ?? "1"));

  const [text, setText] = useState(q);
  useEffect(() => {
    const id = setTimeout(() => {
      if (text === q) return;
      const next = new URLSearchParams(params.toString());
      if (text) next.set("q", text);
      else next.delete("q");
      next.delete("page");
      router.replace(`/search?${next.toString()}`);
    }, DEBOUNCE_MS);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  const setPage = (p: number) => {
    const next = new URLSearchParams(params.toString());
    next.set("page", String(p));
    router.replace(`/search?${next.toString()}`);
  };

  const { data, isLoading, isError, error } = useCandidateSearch({ q, page, limit: LIMIT });
  const hits = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / LIMIT));

  return (
    <div className="settle space-y-6">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("meta")} />

      <div className="relative max-w-2xl">
        <SearchIcon className="pointer-events-none absolute left-4 top-1/2 size-5 -translate-y-1/2 text-brand" />
        <Input
          type="search"
          autoFocus
          placeholder={t("placeholder")}
          value={text}
          onChange={(e) => setText(e.target.value)}
          className="h-14 rounded-xl bg-card pl-12 text-lg ring-1 ring-hairline focus-visible:ring-2"
          aria-label={t("ariaLabel")}
        />
      </div>

      {!q ? (
        <section className="settle overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
          {/* Editorial prompt — a confident signature, not a dashed placeholder */}
          <div className="border-b border-hairline px-7 py-10">
            <p className="eyebrow brass-underline inline-block">{t("promptEyebrow")}</p>
            <p className="mt-4 max-w-xl font-heading text-2xl font-semibold leading-snug tracking-tight text-foreground">
              {t.rich("promptHeading", {
                name: (chunks) => <span className="text-brand">{chunks}</span>,
                province: (chunks) => <span className="text-brand">{chunks}</span>,
              })}
            </p>
            <p className="mt-2 text-sm text-muted-foreground">{t("promptBody")}</p>
          </div>
          <div className="px-7 py-6">
            <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
              {t("quickFilters")}
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              {PROVINCES.map((p) => (
                <button
                  key={p}
                  type="button"
                  onClick={() => setText(p)}
                  className="inline-flex items-center gap-1.5 rounded-full border border-hairline bg-secondary/50 px-3.5 py-1.5 text-sm font-medium text-secondary-foreground transition-colors hover:border-brand/40 hover:bg-brand-soft hover:text-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <MapPin className="size-3.5 text-muted-foreground" strokeWidth={1.75} />
                  {p}
                </button>
              ))}
            </div>
          </div>
        </section>
      ) : isError ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {error instanceof Error ? error.message : t("searchFailed")}
        </div>
      ) : (
        <>
          <p className="text-sm text-muted-foreground tabular-nums">
            {t("results", { count: total, q })}
          </p>
          <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[600px] text-sm">
                <thead className="ledger-head text-left">
                  <tr>
                    <th className="w-16 py-3 pl-5 pr-3">{t("colScore")}</th>
                    <th className="px-3 py-3">{t("colCandidate")}</th>
                    <th className="w-40 px-3 py-3">{t("colProvince")}</th>
                    <th className="w-32 py-3 pl-3 pr-5 text-right">{t("colStatus")}</th>
                  </tr>
                </thead>
                <tbody>
                  {isLoading &&
                    Array.from({ length: 6 }).map((_, i) => (
                      <tr key={i} className="border-b border-hairline last:border-0">
                        <td className="px-5 py-3.5" colSpan={4}>
                          <Skeleton className="h-5 w-full" />
                        </td>
                      </tr>
                    ))}
                  {!isLoading && hits.length === 0 && (
                    <tr>
                      <td colSpan={4} className="px-5 py-16 text-center">
                        <span
                          aria-hidden
                          className="mx-auto mb-4 grid size-11 place-items-center rounded-2xl bg-brand-soft text-brand"
                        >
                          <SearchIcon className="size-5" strokeWidth={1.75} />
                        </span>
                        <p className="text-sm font-semibold text-foreground">{t("emptyTitle", { q })}</p>
                        <p className="mx-auto mt-1 max-w-xs text-sm text-muted-foreground">
                          {t("emptyBody")}
                        </p>
                      </td>
                    </tr>
                  )}
                  {hits.map((h) => (
                    <tr key={h.candidate_id} className="ledger-row border-b border-hairline last:border-0">
                      <td className="py-3.5 pl-5 pr-3">
                        <ScoreBadge score={h.ai_score} />
                      </td>
                      <td className="px-3 py-3">
                        <Link
                          href={`/candidates/${h.candidate_id}`}
                          className="-my-1 flex items-center gap-3 rounded-md py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring"
                        >
                          <InitialChip name={h.full_name} />
                          <span className="font-semibold text-foreground">{h.full_name}</span>
                        </Link>
                      </td>
                      <td className="px-3 py-3.5 text-foreground/80">{h.province || "-"}</td>
                      <td className="py-3.5 pl-3 pr-5 text-right">
                        <StatusPill status={h.status} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          <Pagination page={page} pages={pages} onPage={setPage} />
        </>
      )}
    </div>
  );
}

export default function SearchPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <SearchInner />
    </Suspense>
  );
}
