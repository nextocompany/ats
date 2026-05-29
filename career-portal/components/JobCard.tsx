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

// JobCard is a large, fully-tappable row linking to the position detail.
// Mobile-first: the whole card is the touch target (well over 44px tall).
export function JobCard({ position }: { position: PublicPosition }) {
  return (
    <Link
      href={`/jobs/${position.id}`}
      className="group flex items-center gap-4 rounded-2xl bg-card p-4 ring-1 ring-foreground/10 transition-all hover:ring-primary/40 active:translate-y-px focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
    >
      <div className="min-w-0 flex-1">
        <h2 className="truncate text-base font-semibold text-card-foreground">{position.title_th}</h2>
        <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
          {position.level ? <span>{levelLabel(position.level)}</span> : null}
          <span className="inline-flex items-center gap-1 rounded-full bg-brand-soft px-2 py-0.5 text-xs font-medium text-primary">
            เปิดรับ {position.open_count} อัตรา
          </span>
        </div>
      </div>
      <svg
        width="20"
        height="20"
        viewBox="0 0 24 24"
        fill="none"
        aria-hidden="true"
        className="shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5"
      >
        <path d="M9 6l6 6-6 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </Link>
  );
}
