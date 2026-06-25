"use client";

import { useLocale, useTranslations } from "next-intl";

import type { CompareItem, ScoreBreakdown } from "@/lib/types";

import { RecommendationBadge } from "./RecommendationBadge";

// Dimension order + caps mirror the Go scorer / ScoreBreakdown.tsx.
const DIMENSIONS: { key: keyof ScoreBreakdown; labelKey: string; max: number }[] = [
  { key: "experience", labelKey: "dim_experience", max: 30 },
  { key: "location", labelKey: "dim_location", max: 20 },
  { key: "skills", labelKey: "dim_skills", max: 20 },
  { key: "education", labelKey: "dim_education", max: 10 },
  { key: "language", labelKey: "dim_language", max: 10 },
];

// max ignoring nulls; null when no candidate has a value (so nothing is starred).
function maxOf(values: (number | null)[]): number | null {
  const nums = values.filter((v): v is number => v != null);
  return nums.length ? Math.max(...nums) : null;
}

export function CompareGrid({ items }: { items: CompareItem[] }) {
  const t = useTranslations("compare");
  const locale = useLocale();
  const multi = items.length > 1;

  // Pre-compute the best value per numeric row so the winner can be marked.
  const bestComposite = maxOf(items.map((it) => it.composite));
  const bestScreening = maxOf(items.map((it) => it.screening_score));
  const bestInterview = maxOf(items.map((it) => it.interview_score));
  const bestDim: Record<string, number | null> = {};
  for (const d of DIMENSIONS) bestDim[d.key] = maxOf(items.map((it) => it.breakdown?.[d.key] ?? null));

  const fmtDate = (iso: string) => {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return "-";
    return d.toLocaleDateString(locale === "th" ? "th-TH" : "en-US", { day: "numeric", month: "short", year: "numeric" });
  };

  return (
    <section className="space-y-3">
      <div>
        <h2 className="text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
          {t("gridTitle")}
        </h2>
        <p className="mt-1 text-xs text-muted-foreground">{t("gridHint")}</p>
      </div>

      <div className="overflow-x-auto rounded-xl ring-1 ring-hairline">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="border-b border-hairline bg-muted/40">
              <th className="sticky left-0 z-10 bg-muted/40 px-4 py-3 text-left text-xs font-medium text-muted-foreground" />
              {items.map((it) => (
                <th key={it.application_id} className="min-w-[9rem] px-4 py-3 text-left">
                  <span className="block truncate text-sm font-semibold text-foreground">
                    {it.candidate_name || "ผู้สมัคร"}
                  </span>
                  {it.store_name ? (
                    <span className="block truncate text-xs font-normal text-muted-foreground">{it.store_name}</span>
                  ) : null}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            <ScoreRow label={t("composite")} items={items} value={(it) => it.composite} best={multi ? bestComposite : null} strong />
            <ScoreRow label={t("screening")} items={items} value={(it) => it.screening_score} best={multi ? bestScreening : null} />
            <ScoreRow label={t("interview")} items={items} value={(it) => it.interview_score} best={multi ? bestInterview : null} />

            <Row label={t("rowRecommendation")} items={items}>
              {(it) => <RecommendationBadge recommendation={it.recommendation} />}
            </Row>

            {DIMENSIONS.map((d) => (
              <ScoreRow
                key={d.key}
                label={t(d.labelKey)}
                items={items}
                value={(it) => it.breakdown?.[d.key] ?? null}
                best={multi ? bestDim[d.key] : null}
                suffix={` / ${d.max}`}
              />
            ))}

            <Row label={t("rowMustHave")} items={items}>
              {(it) =>
                it.must_have_passed == null ? (
                  <span className="text-muted-foreground">-</span>
                ) : it.must_have_passed ? (
                  <span className="font-medium text-[var(--score-high)]">{t("mustHaveYes")}</span>
                ) : (
                  <span className="font-medium text-[var(--score-low)]">{t("mustHaveNo")}</span>
                )
              }
            </Row>

            <Row label={t("rowRedFlags")} items={items}>
              {(it) => {
                const flags = (it.ai_red_flags ?? "").split(";").map((s) => s.trim()).filter(Boolean);
                if (flags.length === 0) return <span className="text-xs text-muted-foreground">{t("none")}</span>;
                return (
                  <ul className="space-y-0.5">
                    {flags.map((f, i) => (
                      <li key={i} className="flex gap-1.5 text-xs text-foreground/80">
                        <span aria-hidden className="mt-1.5 size-1 shrink-0 rounded-full bg-brass" />
                        <span>{f}</span>
                      </li>
                    ))}
                  </ul>
                );
              }}
            </Row>

            <Row label={t("rowApplied")} items={items}>
              {(it) => <span className="text-xs text-muted-foreground">{fmtDate(it.applied_at)}</span>}
            </Row>
          </tbody>
        </table>
      </div>
    </section>
  );
}

// Row is the generic two-part row: a sticky label cell + one cell per candidate.
function Row({
  label,
  items,
  children,
}: {
  label: string;
  items: CompareItem[];
  children: (it: CompareItem) => React.ReactNode;
}) {
  return (
    <tr className="border-b border-hairline last:border-0">
      <th scope="row" className="sticky left-0 z-10 bg-card px-4 py-3 text-left align-top text-xs font-medium text-muted-foreground">
        {label}
      </th>
      {items.map((it) => (
        <td key={it.application_id} className="px-4 py-3 align-top">
          {children(it)}
        </td>
      ))}
    </tr>
  );
}

// ScoreRow renders a numeric metric per candidate, starring the row's best.
function ScoreRow({
  label,
  items,
  value,
  best,
  suffix = "",
  strong = false,
}: {
  label: string;
  items: CompareItem[];
  value: (it: CompareItem) => number | null;
  best: number | null;
  suffix?: string;
  strong?: boolean;
}) {
  return (
    <Row label={label} items={items}>
      {(it) => {
        const v = value(it);
        if (v == null) return <span className="text-muted-foreground">-</span>;
        const isBest = best != null && v === best;
        return (
          <span
            className={`num tabular-nums ${isBest ? "font-semibold text-[var(--score-high)]" : strong ? "font-semibold text-foreground" : "text-foreground"}`}
          >
            {v}
            {suffix ? <span className="text-muted-foreground">{suffix}</span> : null}
            {isBest ? <span aria-hidden className="ml-1 text-[var(--score-high)]">★</span> : null}
          </span>
        );
      }}
    </Row>
  );
}
