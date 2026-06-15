"use client";

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
            {isLoading ? (
              "กำลังโหลดตำแหน่งงาน…"
            ) : (
              <>
                พบ <span className="num font-semibold text-foreground">{results.length}</span> ตำแหน่ง
              </>
            )}
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
            <p className="text-sm text-muted-foreground">ไม่สามารถโหลดตำแหน่งงานได้ในขณะนี้</p>
            <Button size="default" variant="outline" onClick={() => refetch()}>
              ลองอีกครั้ง
            </Button>
          </div>
        ) : null}

        {!isLoading && !isError && results.length === 0 ? (
          <div className="flex flex-col items-center gap-3 rounded-xl border border-line bg-card p-12 text-center">
            <p className="text-base font-medium text-foreground">
              {hasActiveFilters ? "ไม่พบตำแหน่งที่ตรงกับตัวกรอง" : "ยังไม่มีตำแหน่งงานที่เปิดรับ"}
            </p>
            <p className="text-sm text-muted-foreground">
              {hasActiveFilters
                ? "ลองปรับคำค้นหาหรือเลือกระดับตำแหน่งอื่น"
                : "โปรดกลับมาตรวจสอบอีกครั้งในภายหลัง"}
            </p>
            {hasActiveFilters ? (
              <Button size="default" variant="outline" onClick={clearAll} className="mt-1">
                ล้างตัวกรอง
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
  return (
    <div className="flex min-h-dvh flex-col">
      <SiteHeader />
      <main className="flex-1">
        <Container className="py-12 sm:py-16">
          <header className="mb-12 flex max-w-2xl flex-col gap-3">
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">ร่วมงานกับ CP Axtra</p>
            <h1 className="[font-size:var(--text-display)] font-bold leading-[1.1] text-foreground">
              ตำแหน่งงานที่เปิดรับ
            </h1>
            <p className="[font-size:var(--text-lead)] leading-relaxed text-muted-foreground">
              ค้นหาและกรองตำแหน่งที่เหมาะกับคุณ แล้วสมัครได้ในไม่กี่ขั้นตอน
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
