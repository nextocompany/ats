"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";

import { CompareGrid } from "@/components/compare/CompareGrid";
import { CompareLeaderboard } from "@/components/compare/CompareLeaderboard";
import { PageHeader } from "@/components/shell/PageHeader";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useCompare, useMe, usePositions } from "@/lib/queries";
import { canCompareCandidates } from "@/lib/roles";
import type { Position } from "@/lib/types";

// How many top candidates are pre-loaded into the side-by-side grid, and the cap.
const DEFAULT_GRID = 5;
const MAX_GRID = 6;

function ComparePage() {
  const t = useTranslations("compare");
  const locale = useLocale();
  const params = useSearchParams();
  const router = useRouter();
  const { data: me } = useMe();
  const { data: positions } = usePositions();

  const positionId = params.get("position_id") ?? "";

  function setPosition(id: string) {
    const next = new URLSearchParams(params);
    if (id) next.set("position_id", id);
    else next.delete("position_id");
    router.replace(`/compare?${next.toString()}`);
  }

  const titleFor = (p: Position) => (locale === "th" ? p.title_th || p.title_en : p.title_en || p.title_th);

  if (me && !canCompareCandidates(me)) {
    return (
      <div className="settle">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <section className="mt-8 rounded-xl bg-card p-10 text-center ring-1 ring-hairline">
          <p className="text-sm font-semibold text-foreground">{t("notAvailable")}</p>
          <p className="mx-auto mt-1 max-w-sm text-xs text-muted-foreground">{t("notAvailableHint")}</p>
        </section>
      </div>
    );
  }

  return (
    <div className="settle space-y-6">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        meta={t("meta")}
        actions={
          <Select value={positionId} onValueChange={(v) => setPosition(v ?? "")}>
            <SelectTrigger className="w-64" size="sm">
              <SelectValue placeholder={t("positionPlaceholder")}>
                {(v: string | null) => {
                  const p = (positions ?? []).find((x) => x.id === v);
                  return p ? titleFor(p) : t("positionPlaceholder");
                }}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {(positions ?? []).map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {titleFor(p)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        }
      />

      {positionId === "" ? (
        <p className="rounded-xl bg-card px-5 py-12 text-center text-sm text-muted-foreground ring-1 ring-hairline">
          {t("pickPositionFirst")}
        </p>
      ) : (
        <CompareBody key={positionId} positionId={positionId} />
      )}
    </div>
  );
}

// CompareBody is keyed by positionId so its column selection resets when the
// position changes (no effect needed — the component remounts).
function CompareBody({ positionId }: { positionId: string }) {
  const t = useTranslations("compare");
  const { data, isLoading } = useCompare(positionId);
  // null = untouched (show the default top-N); an array = the user's explicit set.
  const [picked, setPicked] = useState<string[] | null>(null);

  if (isLoading || !data) return <Skeleton className="h-72 w-full rounded-xl" />;

  const candidates = data.candidates;
  if (candidates.length === 0) {
    return (
      <section className="rounded-xl bg-card p-10 text-center ring-1 ring-hairline">
        <p className="text-sm font-semibold text-foreground">{t("emptyTitle")}</p>
        <p className="mx-auto mt-1 max-w-md text-xs text-muted-foreground">{t("emptyHint")}</p>
      </section>
    );
  }

  const defaultIds = candidates.slice(0, DEFAULT_GRID).map((c) => c.application_id);
  const selectedIds = picked ?? defaultIds;

  function toggle(id: string) {
    setPicked((prev) => {
      const base = prev ?? defaultIds;
      if (base.includes(id)) return base.filter((x) => x !== id);
      if (base.length >= MAX_GRID) return base;
      return [...base, id];
    });
  }

  const gridItems = candidates.filter((c) => selectedIds.includes(c.application_id));

  return (
    <div className="space-y-8">
      <p className="text-sm text-muted-foreground">{t("eligibleCount", { count: candidates.length })}</p>
      <CompareLeaderboard items={candidates} selected={selectedIds} onToggle={toggle} />
      {gridItems.length > 0 ? <CompareGrid items={gridItems} /> : null}
    </div>
  );
}

export default function ComparePageWrapper() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <ComparePage />
    </Suspense>
  );
}
