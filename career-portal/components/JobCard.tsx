import Link from "next/link";

import type { PublicPosition } from "@/lib/types";

const LEVEL_LABELS: Record<string, string> = {
  entry: "ระดับเริ่มต้น",
  experienced: "มีประสบการณ์",
  senior: "ระดับอาวุโส",
  management: "ระดับบริหาร",
};

function levelLabel(level: string): string {
  return LEVEL_LABELS[level.toLowerCase()] ?? level;
}

// JobCard is a fully-tappable card linking to the position detail. Works as a
// row on mobile and a grid cell on larger screens; the whole card is the touch
// target (well over 44px). CP Axtra: white surface, hairline border, a
// blue hover lift, and a yellow corner accent on hover.
export function JobCard({ position }: { position: PublicPosition }) {
  return (
    <Link
      href={`/jobs/${position.id}`}
      className="group relative flex h-full flex-col gap-4 overflow-hidden rounded-2xl border border-border bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:border-accent/40 hover:shadow-[0_12px_40px_-12px_oklch(46%_0.18_264/0.22)] active:translate-y-0 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none sm:p-6"
    >
      {/* hairline yellow top-accent revealed on hover */}
      <span
        aria-hidden="true"
        className="absolute inset-x-0 top-0 h-px scale-x-0 bg-gold transition-transform duration-300 group-hover:scale-x-100"
      />
      <div className="flex items-start justify-between gap-3">
        {position.level ? (
          <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            {levelLabel(position.level)}
          </span>
        ) : (
          <span />
        )}
        <svg
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
          className="shrink-0 text-muted-foreground/60 transition-all group-hover:translate-x-0.5 group-hover:text-accent"
        >
          <path d="M9 6l6 6-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>

      <h2 className="text-lg font-semibold leading-snug text-card-foreground sm:text-xl">{position.title_th}</h2>

      <div className="mt-auto flex items-center gap-2 pt-1">
        <span className="inline-flex items-center gap-1.5 rounded-full bg-brand-soft px-3 py-1 text-xs font-semibold text-accent">
          <span className="size-1.5 rounded-full bg-accent" aria-hidden="true" />
          เปิดรับ {position.open_count} อัตรา
        </span>
      </div>
    </Link>
  );
}
