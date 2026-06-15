import Link from "next/link";

import { levelLabel } from "@/lib/levels";
import type { PublicPosition } from "@/lib/types";

// JobCard is a flat, fully-tappable role row linking to the position detail. The
// whole card is the touch target (well over 44px). Institutional: white surface,
// hairline frame, a quiet border-hover, no decorative lift or color flourish —
// the one blue is reserved for the open-count figure and the focus ring.
export function JobCard({ position }: { position: PublicPosition }) {
  return (
    <Link
      href={`/jobs/${position.id}`}
      className="group flex h-full flex-col gap-4 rounded-xl border border-line bg-card p-5 transition-colors duration-150 hover:border-foreground/25 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:ring-offset-2 focus-visible:ring-offset-background sm:p-6"
    >
      <div className="flex items-start justify-between gap-3">
        {position.level ? (
          <span className="text-xs font-medium uppercase tracking-[0.12em] text-muted-foreground">
            {levelLabel(position.level)}
          </span>
        ) : (
          <span />
        )}
        <svg
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
          className="shrink-0 text-muted-foreground/50 transition-all duration-150 group-hover:translate-x-0.5 group-hover:text-primary"
        >
          <path d="M9 6l6 6-6 6" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </div>

      <h3 className="text-balance text-lg font-semibold leading-snug text-card-foreground">
        {position.title_th}
      </h3>

      <div className="mt-auto flex items-baseline gap-1.5 border-t border-line pt-4">
        <span className="num text-base font-semibold text-primary">{position.open_count}</span>
        <span className="text-sm text-muted-foreground">อัตราที่เปิดรับ</span>
      </div>
    </Link>
  );
}
