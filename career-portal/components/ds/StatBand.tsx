import { cn } from "@/lib/utils";

export interface Stat {
  // The large tabular figure, e.g. "150,000" or "2,600+".
  value: string;
  // The quiet label below, e.g. "ล้านบาท / ปี".
  label: string;
  // Optional small note (e.g. an English gloss or qualifier).
  note?: string;
}

interface StatBandProps {
  stats: Stat[];
  className?: string;
  tone?: "default" | "invert";
}

// StatBand is a row of large quiet figures (tabular Anuphan numerals) divided by
// hairlines — institutional proof-of-scale. No icons, no color fills; the numbers
// carry the weight. Stacks on mobile, divides on a row at md+.
export function StatBand({ stats, className, tone = "default" }: StatBandProps) {
  const invert = tone === "invert";
  return (
    <dl
      className={cn(
        "grid grid-cols-1 gap-px overflow-hidden sm:grid-cols-2 lg:grid-cols-4",
        invert ? "bg-primary-foreground/15" : "bg-line",
        className,
      )}
    >
      {stats.map((stat) => (
        <div
          key={`${stat.value}-${stat.label}`}
          className={cn(
            "flex flex-col gap-1.5 px-6 py-8 sm:py-10",
            invert ? "bg-primary" : "bg-background",
          )}
        >
          <dd
            className={cn(
              "num [font-size:var(--text-stat)] font-semibold leading-none tracking-tight",
              invert ? "text-primary-foreground" : "text-foreground",
            )}
          >
            {stat.value}
          </dd>
          <dt
            className={cn(
              "text-sm font-medium",
              invert ? "text-primary-foreground/80" : "text-foreground/80",
            )}
          >
            {stat.label}
          </dt>
          {stat.note ? (
            <p className={cn("text-xs", invert ? "text-primary-foreground/55" : "text-muted-foreground")}>
              {stat.note}
            </p>
          ) : null}
        </div>
      ))}
    </dl>
  );
}
