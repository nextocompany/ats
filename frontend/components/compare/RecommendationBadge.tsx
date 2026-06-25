"use client";

import { useTranslations } from "next-intl";

// Tone per AI-interview recommendation. Green-ish brand for strong/recommend,
// quiet neutral for neutral, clay (warning) for caution — matching the app's
// score band semantics (clay = real signal, never the mid band).
const TONE: Record<string, string> = {
  strong_recommend: "bg-[var(--score-high)] text-white ring-white/15",
  recommend: "bg-brand-soft text-brand ring-hairline",
  neutral: "bg-secondary text-foreground/70 ring-hairline",
  caution: "bg-[var(--score-low)] text-white ring-white/15",
};

const KNOWN = new Set(["strong_recommend", "recommend", "neutral", "caution"]);

export function RecommendationBadge({ recommendation }: { recommendation: string }) {
  const t = useTranslations("compare");
  if (!recommendation) return <span className="text-xs text-muted-foreground">-</span>;
  const cls = TONE[recommendation] ?? "bg-secondary text-foreground/70 ring-hairline";
  const label = KNOWN.has(recommendation) ? t(`rec_${recommendation}`) : recommendation;
  return (
    <span className={`inline-flex items-center rounded-md px-1.5 py-0.5 text-[0.6875rem] font-semibold ring-1 ring-inset ${cls}`}>
      {label}
    </span>
  );
}
