"use client";

import { useTranslations } from "next-intl";
import { Suspense, useMemo } from "react";

import { Container } from "@/components/ds";
import { FilterTags } from "@/components/jobs/FilterTags";
import { JobFilters } from "@/components/jobs/JobFilters";
import { JobCard } from "@/components/JobCard";
import { SiteFooter } from "@/components/SiteFooter";
import { SiteHeader } from "@/components/SiteHeader";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useJobFilters } from "@/lib/useJobFilters";
import { usePublicPositions } from "@/lib/queries";
import type { PublicPosition } from "@/lib/types";

function matchesQuery(position: PublicPosition, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  return (
    position.title_th.toLowerCase().includes(needle) ||
    position.title_en.toLowerCase().includes(needle)
  );
}

function JobsBrowse() {
  const t = useTranslations("jobs");
  const { data: positions, isLoading, isError, refetch } = usePublicPositions();
  const { query, levels, hasActiveFilters, setQuery, toggleLevel, removeLevel, clearQuery, clearAll } =
    useJobFilters();

  // Per-level counts over the unfiltered set (so facet numbers stay stable as the
  // user narrows by query — they reflect availability, not the current view).
  const levelCounts = useMemo<Record<string, number>>(() => {
    const counts: Record<string, number> = {};
    for (const p of positions ?? []) {
      const key = p.level.toLowerCase();
      counts[key] = (counts[key] ?? 0) + 1;
    }
    return counts;
  }, [positions]);

  const results = useMemo(() => {
    const list = positions ?? [];
    const levelSet = new Set<string>(levels);
    return list.filter(
      (p) => matchesQuery(p, query) && (levelSet.size === 0 || levelSet.has(p.level.toLowerCase())),
    );
  }, [positions, query, levels]);

  return (
    <div className="grid gap-10 lg:grid-cols-[260px_1fr] lg:gap-14">
      {/* Left rail — sticky on desktop, stacked above results on mobile. */}
      <aside className="lg:sticky lg:top-24 lg:self-start">
        <JobFilters
          query={query}
          levels={levels}
          levelCounts={levelCounts}
          onQueryChange={setQuery}
          onToggleLevel={toggleLevel}
          onClear={clearAll}
          hasActiveFilters={hasActiveFilters}
        />
      </aside>

      {/* Results column. */}
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-4 border-b border-line pb-5">
          <p className="text-sm text-muted-foreground" aria-live="polite">
            {isLoading
              ? t("loading")
              : t.rich("countFound", {
                  count: results.length,
                  num: (chunks) => <span className="num font-semibold text-foreground">{chunks}</span>,
                })}
          </p>
          <FilterTags
            query={query}
            levels={levels}
            onRemoveQuery={clearQuery}
            onRemoveLevel={removeLevel}
          />
        </div>

        {isLoading ? (
          <div className="grid gap-5 sm:grid-cols-2" aria-hidden="true">
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-[168px] w-full" />
            ))}
          </div>
        ) : null}

        {isError ? (
          <div className="flex flex-col items-center gap-4 rounded-xl border border-line bg-card p-10 text-center">
            <p className="text-sm text-muted-foreground">{t("loadError")}</p>
            <Button size="default" variant="outline" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        ) : null}

        {!isLoading && !isError && results.length === 0 ? (
          <div className="flex flex-col items-center gap-3 rounded-xl border border-line bg-card p-12 text-center">
            <p className="text-base font-medium text-foreground">
              {hasActiveFilters ? t("emptyFiltered") : t("emptyNone")}
            </p>
            <p className="text-sm text-muted-foreground">
              {hasActiveFilters ? t("emptyFilteredHint") : t("emptyNoneHint")}
            </p>
            {hasActiveFilters ? (
              <Button size="default" variant="outline" onClick={clearAll} className="mt-1">
                {t("clearFilters")}
              </Button>
            ) : null}
          </div>
        ) : null}

        {results.length > 0 ? (
          <ul className="grid gap-5 sm:grid-cols-2">
            {results.map((p) => (
              <li key={p.id}>
                <JobCard position={p} />
              </li>
            ))}
          </ul>
        ) : null}
      </div>
    </div>
  );
}

export default function JobsPage() {
  const t = useTranslations("jobs");
  return (
    <div className="flex min-h-dvh flex-col">
      <SiteHeader />
      <main className="flex-1">
        <Container className="py-12 sm:py-16">
          <header className="mb-12 flex max-w-2xl flex-col gap-3">
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">{t("heroEyebrow")}</p>
            <h1 className="[font-size:var(--text-display)] font-bold leading-[1.1] text-foreground">
              {t("heroTitle")}
            </h1>
            <p className="[font-size:var(--text-lead)] leading-relaxed text-muted-foreground">
              {t("heroLead")}
            </p>
          </header>
          <Suspense
            fallback={<Skeleton className="h-96 w-full" />}
          >
            <JobsBrowse />
          </Suspense>
        </Container>
      </main>
      <SiteFooter />
    </div>
  );
}
