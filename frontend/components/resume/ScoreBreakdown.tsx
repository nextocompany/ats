import type { ScoreBreakdown as Breakdown } from "@/lib/types";

// Dimension order + max points mirror the Go scorer (scoring.Breakdown):
// experience 30, location 20, skills 20 (LLM), education 10, language 10.
const DIMENSIONS: { key: keyof Breakdown; label: string; max: number; llm?: boolean }[] = [
  { key: "experience", label: "ประสบการณ์", max: 30 },
  { key: "location", label: "ทำเล / ภูมิภาค", max: 20 },
  { key: "skills", label: "ทักษะ", max: 20, llm: true },
  { key: "education", label: "การศึกษา", max: 10 },
  { key: "language", label: "ภาษา", max: 10 },
];

function splitLines(text: string | undefined, sep: string): string[] {
  if (!text) return [];
  return text
    .split(sep)
    .map((s) => s.trim())
    .filter(Boolean);
}

interface ScoreBreakdownProps {
  breakdown: Breakdown;
  summary?: string;
  redFlags?: string;
}

export function ScoreBreakdown({ breakdown, summary, redFlags }: ScoreBreakdownProps) {
  const strengths = splitLines(summary, "\n");
  const flags = splitLines(redFlags, ";");

  return (
    <div className="border-t border-hairline pt-5">
      <p className="mb-3 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
        Score breakdown
      </p>

      <ul className="space-y-2.5">
        {DIMENSIONS.map(({ key, label, max, llm }) => {
          const points = breakdown[key] ?? 0;
          const pct = max > 0 ? Math.min(100, (points / max) * 100) : 0;
          return (
            <li key={key} className="grid grid-cols-[7.5rem_1fr_auto] items-center gap-3">
              <span className="flex items-center gap-1.5 text-xs text-foreground">
                {label}
                {llm && (
                  <span className="rounded bg-brass-soft px-1 py-px text-[0.5625rem] font-semibold uppercase tracking-wide text-brass">
                    AI
                  </span>
                )}
              </span>
              <span className="h-2 overflow-hidden rounded-full bg-muted" role="presentation">
                <span
                  className="block h-full rounded-full bg-brand"
                  style={{ width: `${pct}%` }}
                />
              </span>
              <span className="text-right text-xs tabular-nums">
                <span className="font-semibold text-foreground">{points}</span>
                <span className="text-muted-foreground"> / {max}</span>
              </span>
            </li>
          );
        })}
      </ul>

      {strengths.length > 0 && (
        <div className="mt-4">
          <p className="mb-1.5 text-xs font-medium text-foreground">จุดแข็ง</p>
          <ul className="space-y-1">
            {strengths.map((s, i) => (
              <li key={i} className="flex gap-2 text-xs text-muted-foreground">
                <span aria-hidden className="mt-1.5 size-1 shrink-0 rounded-full bg-brand" />
                <span>{s}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {flags.length > 0 && (
        <div className="mt-3">
          <p className="mb-1.5 text-xs font-medium text-foreground">ข้อสังเกต</p>
          <ul className="space-y-1">
            {flags.map((f, i) => (
              <li key={i} className="flex gap-2 text-xs text-muted-foreground">
                <span aria-hidden className="mt-1.5 size-1 shrink-0 rounded-full bg-brass" />
                <span>{f}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
