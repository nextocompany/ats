"use client";

import { useTranslations } from "next-intl";

interface ScoreBadgeProps {
  score: number | null;
}

/* Plain-language fit tier — four bands an HR reader understands without knowing
   the 0–100 scale: ไม่ผ่าน / มาตรฐาน / ดี / ดีมาก. Returns a translation key
   (resolved by FitLabel) + a colour band. Shared so inbox, shortlist, candidates,
   and search all speak "fit" the same way. */
export function fitTier(
  score: number | null | undefined,
): { key: "fail" | "standard" | "good" | "veryGood"; band: "high" | "mid" | "low" } | null {
  if (score === null || score === undefined) return null;
  if (score >= 75) return { key: "veryGood", band: "high" };
  if (score >= 60) return { key: "good", band: "high" };
  if (score >= 45) return { key: "standard", band: "mid" };
  return { key: "fail", band: "low" };
}

/* Word label that pairs with the numeric ScoreBadge — colour-matched to the
   badge band so "82 · ดีมาก" reads as one unit. */
export function FitLabel({ score }: ScoreBadgeProps) {
  const t = useTranslations("fit");
  const tier = fitTier(score);
  if (!tier) return <span className="text-xs text-muted-foreground">{t("notScored")}</span>;
  const color =
    tier.band === "high"
      ? "text-[var(--score-high)]"
      : tier.band === "low"
        ? "text-[var(--score-low)]"
        : "text-muted-foreground";
  return <span className={`text-xs font-medium ${color}`}>{t(tier.key)}</span>;
}

/* AI score chip — semantic ramp tuned so the color reads as *fit*, not caution.
   HIGH (≥75) → CP Axtra blue (strong fit). MID (50–74) → quiet ink/neutral, so a
   merely-average score never competes with the amber "review" flag. LOW (<50)
   → clay (weak fit). Amber/clay is reserved for genuine signal, never the mid band. */
export function ScoreBadge({ score }: ScoreBadgeProps) {
  const t = useTranslations("fit");
  if (score === null || score === undefined) {
    return <span className="inline-block w-9 text-center text-xs text-muted-foreground tabular-nums">—</span>;
  }

  const rounded = Math.round(score);
  const band: "high" | "mid" | "low" = score >= 75 ? "high" : score >= 50 ? "mid" : "low";

  // HIGH: solid blue fill. MID: neutral ink outline (no fill) — reads calm.
  // LOW: clay fill — a true, sparing warning.
  const cls =
    band === "high"
      ? "bg-[var(--score-high)] text-white ring-white/15"
      : band === "low"
        ? "bg-[var(--score-low)] text-white ring-white/15"
        : "bg-secondary text-foreground ring-hairline";

  return (
    <span
      className={`inline-flex min-w-9 items-center justify-center rounded-md px-1.5 py-1 text-xs font-semibold tabular-nums ring-1 ring-inset ${cls}`}
      aria-label={t("scoreAria", { score: rounded })}
    >
      {rounded}
    </span>
  );
}

/* Per-row score-distribution rail — a restrained 0–100 micro-gauge under the
   badge. The fill maps to the score's position on the scale, tinted by band so
   the screening queue reads as a designed distribution at a glance, not just a
   column of numbers. CSS-only, compositor-friendly (transform scaleX), and it
   lives below the badge so table columns + row anchors are untouched. */
export function ScoreRail({ score }: ScoreBadgeProps) {
  if (score === null || score === undefined) return null;

  const pct = Math.max(0, Math.min(100, score));
  const band: "high" | "mid" | "low" = score >= 75 ? "high" : score >= 50 ? "mid" : "low";
  const fill =
    band === "high" ? "var(--score-high)" : band === "low" ? "var(--score-low)" : "var(--muted-foreground)";

  return (
    <span
      aria-hidden
      className="mt-1.5 block h-[3px] w-9 overflow-hidden rounded-full bg-hairline"
    >
      <span
        className="block h-full origin-left rounded-full transition-transform duration-500 motion-reduce:transition-none"
        style={{
          background: fill,
          transform: `scaleX(${pct / 100})`,
          transitionTimingFunction: "var(--ease-out)",
        }}
      />
    </span>
  );
}
