"use client";

// Per-position screening-weights settings. Pick a position, set how much each of
// the five screening dimensions matters (must sum to 100); the AI screening score
// is weighted by these. Gated to settings.admin.
import { useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { Loader2, ShieldAlert } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/shell/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useMe, usePositions, usePosition, useUpdatePositionWeights } from "@/lib/queries";
import { canManageScoring } from "@/lib/roles";
import type { ScoreWeights } from "@/lib/types";

const DIMENSIONS = ["experience", "skills", "education", "language", "location"] as const;
type Dim = (typeof DIMENSIONS)[number];

const DEFAULT_WEIGHTS: ScoreWeights = {
  experience: 34,
  skills: 22,
  education: 11,
  language: 11,
  location: 22,
};

function errMessage(err: unknown): string | null {
  return err instanceof Error ? err.message : null;
}

export default function ScoringPage() {
  const t = useTranslations("scoring");
  const locale = useLocale();
  const { data: me, isLoading: meLoading } = useMe();
  const allowed = canManageScoring(me);

  const { data: positions } = usePositions();
  const [positionId, setPositionId] = useState("");
  const detail = usePosition(positionId, allowed && positionId !== "");
  const update = useUpdatePositionWeights();

  // Working copy of the weights, hydrated from the loaded position (render-time
  // adjust-state pattern — avoids the project's react-hooks/set-state-in-effect rule).
  const [weights, setWeights] = useState<ScoreWeights | null>(null);
  const [loadedFor, setLoadedFor] = useState("");
  if (detail.data && detail.data.id === positionId && loadedFor !== positionId) {
    setLoadedFor(positionId);
    setWeights(detail.data.score_weights);
  }

  const total = weights ? DIMENSIONS.reduce((s, d) => s + (weights[d] || 0), 0) : 0;
  const busy = update.isPending;
  const canSave = !!weights && total === 100 && !busy;

  function setDim(d: Dim, raw: string) {
    const n = raw === "" ? 0 : Math.max(0, Math.min(100, Math.round(Number(raw))));
    setWeights((w) => (w ? { ...w, [d]: n } : w));
  }

  async function save() {
    if (!canSave || !weights) return;
    await update.mutateAsync(
      { id: positionId, weights },
      {
        onSuccess: () => toast.success(t("savedToast")),
        onError: (err) => toast.error(errMessage(err) ?? t("saveFailed")),
      },
    );
  }

  if (meLoading) return <Skeleton className="h-40 w-full rounded-xl" />;

  if (!allowed) {
    return (
      <div className="settle space-y-8">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <section className="flex items-start gap-3 rounded-xl bg-card p-6 ring-1 ring-hairline">
          <ShieldAlert className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">{t("restricted")}</p>
        </section>
      </div>
    );
  }

  return (
    <div className="settle space-y-6">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("meta")} />

      <div className="max-w-xl space-y-6">
        <label className="block space-y-1.5">
          <span className="text-xs font-medium text-foreground">{t("selectPosition")}</span>
          <Select value={positionId} onValueChange={(v) => setPositionId(v ?? "")}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("positionPlaceholder")}>
                {(v: string | null) => {
                  const p = (positions ?? []).find((p) => p.id === v);
                  if (!p) return t("positionPlaceholder");
                  return locale === "th" ? p.title_th || p.title_en : p.title_en || p.title_th;
                }}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {(positions ?? []).map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {locale === "th" ? p.title_th || p.title_en : p.title_en || p.title_th}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>

        {positionId === "" ? (
          <p className="rounded-xl bg-card px-5 py-10 text-center text-sm text-muted-foreground ring-1 ring-hairline">
            {t("pickPositionFirst")}
          </p>
        ) : detail.isLoading || !weights ? (
          <Skeleton className="h-72 w-full rounded-xl" />
        ) : (
          <section className="space-y-5 rounded-xl bg-card p-6 ring-1 ring-hairline">
            <div className="space-y-4">
              {DIMENSIONS.map((d) => (
                <label key={d} className="flex items-center justify-between gap-4">
                  <span className="text-sm text-foreground">{t(`dim_${d}`)}</span>
                  <span className="flex items-center gap-1.5">
                    <Input
                      type="number"
                      min={0}
                      max={100}
                      value={String(weights[d])}
                      onChange={(e) => setDim(d, e.target.value)}
                      className="w-24 text-right"
                    />
                    <span className="text-sm text-muted-foreground">%</span>
                  </span>
                </label>
              ))}
            </div>

            <div className="flex items-center justify-between border-t border-hairline pt-4">
              <span className="text-sm font-medium text-foreground">{t("total")}</span>
              <span
                className={`num text-sm font-semibold ${total === 100 ? "text-foreground" : "text-destructive"}`}
              >
                {total} / 100
              </span>
            </div>
            {total !== 100 && <p className="text-xs text-destructive">{t("mustEqual100")}</p>}

            <div className="flex items-center justify-end gap-2">
              <Button type="button" variant="ghost" onClick={() => setWeights({ ...DEFAULT_WEIGHTS })}>
                {t("resetDefault")}
              </Button>
              <Button type="button" onClick={save} disabled={!canSave} className="gap-2">
                {busy && <Loader2 className="size-4 animate-spin" />}
                {t("save")}
              </Button>
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
