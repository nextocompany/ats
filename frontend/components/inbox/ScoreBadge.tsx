interface ScoreBadgeProps {
  score: number | null;
}

// Maps the AI score to the semantic ramp (the single accent system of the console).
export function ScoreBadge({ score }: ScoreBadgeProps) {
  if (score === null || score === undefined) {
    return <span className="text-xs text-muted-foreground">—</span>;
  }
  const color = score >= 75 ? "var(--score-high)" : score >= 50 ? "var(--score-mid)" : "var(--score-low)";
  return (
    <span
      className="inline-flex min-w-9 items-center justify-center rounded px-1.5 py-0.5 text-xs font-semibold tabular-nums text-white"
      style={{ backgroundColor: color }}
      aria-label={`AI score ${Math.round(score)}`}
    >
      {Math.round(score)}
    </span>
  );
}
