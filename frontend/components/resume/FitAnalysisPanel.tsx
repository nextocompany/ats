"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { Application, FitAnalysis } from "@/lib/types";
import { useFitAnalysis, useGenerateFitAnalysis } from "@/lib/queries";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

function overallTone(fit: FitAnalysis["overall_fit"]): string {
  if (fit === "strong") return "var(--score-high)";
  if (fit === "moderate") return "var(--score-mid)";
  if (fit === "weak") return "var(--score-mid)";
  return "var(--score-low)"; // none
}

function scoreTone(score: number): string {
  if (score >= 75) return "var(--score-high)";
  if (score >= 50) return "var(--score-mid)";
  return "var(--score-low)";
}

export function FitAnalysisPanel({ applicationId, app }: { applicationId: string; app: Application }) {
  const t = useTranslations("resume");
  const { data, isLoading, isError } = useFitAnalysis(applicationId);
  const gen = useGenerateFitAnalysis(applicationId);

  const generate = () =>
    gen.mutate(undefined, {
      onSuccess: () => toast.success(t("fitDone")),
      onError: (e) => toast.error(e instanceof Error ? e.message : t("fitFailed")),
    });

  if (isLoading) return null;

  const header = (
    <div className="flex items-center justify-between">
      <div>
        <p className="eyebrow">{t("fitEyebrow")}</p>
        <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">{t("fitTitle")}</h2>
      </div>
      {data && (
        <Badge variant="secondary" className="capitalize">
          {t(`fit_${data.overall_fit}`)}
        </Badge>
      )}
    </div>
  );

  // Not generated yet (404 → null) or a load error → show the generate CTA.
  if (!data) {
    return (
      <div className="mt-6 space-y-4 border-t border-hairline pt-6" aria-busy={gen.isPending}>
        {header}
        {isError ? (
          <p className="text-xs text-muted-foreground">{t("fitLoadFailed")}</p>
        ) : (
          <p className="text-sm text-muted-foreground">{t("fitNotYet")}</p>
        )}
        <Button size="sm" disabled={gen.isPending || app.ai_score === null} onClick={() => generate()} className="w-full">
          {gen.isPending ? t("fitAnalyzing") : t("fitGenerate")}
        </Button>
        {app.ai_score === null && (
          <p className="text-xs text-muted-foreground">{t("fitNeedScore")}</p>
        )}
      </div>
    );
  }

  return (
    <div className="mt-6 space-y-4 border-t border-hairline pt-6" aria-busy={gen.isPending}>
      {header}
      {data.summary && (
        <div className="rounded-lg bg-brand-soft/60 p-4 ring-1 ring-brand/10">
          <span
            className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold text-white"
            style={{ backgroundColor: overallTone(data.overall_fit) }}
          >
            {t(`fit_${data.overall_fit}`)}
          </span>
          <p className="mt-2 text-sm leading-relaxed text-foreground">{data.summary}</p>
        </div>
      )}

      {data.strengths.length > 0 && (
        <div>
          <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{t("fitStrengths")}</p>
          <ul className="space-y-1.5 text-sm text-foreground">
            {data.strengths.map((t, i) => (
              <li key={i} className="flex gap-2">
                <span className="text-[var(--score-high)]">•</span>
                <span>{t}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {data.concerns.length > 0 && (
        <div>
          <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
            {t("fitConcerns")}
          </p>
          <ul className="space-y-1.5 text-sm text-foreground">
            {data.concerns.map((t, i) => (
              <li key={i} className="flex gap-2">
                <span className="text-[var(--score-low)]">•</span>
                <span>{t}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {data.overall_fit === "none" ? (
        <div className="rounded-lg bg-[var(--score-low)]/10 p-4 text-sm text-foreground ring-1 ring-[var(--score-low)]/20">
          <p className="font-medium">{t("fit_none")}</p>
          {data.no_match_reason && <p className="mt-1 text-muted-foreground">{data.no_match_reason}</p>}
        </div>
      ) : (
        data.recommended.length > 0 && (
          <div>
            <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
              {t("fitRecommended")}
            </p>
            <ul className="space-y-3">
              {data.recommended.map((r) => (
                <li key={r.position_id} className="rounded-lg bg-card p-3 ring-1 ring-hairline">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-sm font-medium text-foreground">{r.title}</span>
                    <span
                      className="grid size-9 shrink-0 place-items-center rounded-md text-sm font-semibold tabular-nums text-white"
                      style={{ backgroundColor: scoreTone(r.fit_score) }}
                      aria-label={t("fitScoreAria", { score: r.fit_score })}
                    >
                      {r.fit_score}
                    </span>
                  </div>
                  {r.reasons.length > 0 && (
                    <ul className="mt-2 space-y-1 text-xs leading-relaxed text-muted-foreground">
                      {r.reasons.map((reason, i) => (
                        <li key={`${r.position_id}-${i}`} className="flex gap-2">
                          <span className="text-[var(--score-high)]">•</span>
                          <span>{reason}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </li>
              ))}
            </ul>
          </div>
        )
      )}

      <button
        type="button"
        onClick={() => generate()}
        disabled={gen.isPending}
        className="text-xs font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm disabled:opacity-50"
      >
        {gen.isPending ? t("fitAnalyzing") : t("fitRegenerate")}
      </button>
    </div>
  );
}
