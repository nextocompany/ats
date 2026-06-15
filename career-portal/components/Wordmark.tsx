import Link from "next/link";

import { cn } from "@/lib/utils";

interface WordmarkProps {
  // invert renders the wordmark for dark (navy/blue) surfaces, e.g. the footer CTA.
  invert?: boolean;
  className?: string;
}

// Wordmark is the text-based CP Axtra Careers identity — no dot mark, no logo
// graphic. "CP AXTRA" in display weight with a hairline divider and a quiet
// "Careers" label, the institutional convention (HSBC/JPM/GIC register).
export function Wordmark({ invert, className }: WordmarkProps) {
  return (
    <Link
      href="/"
      aria-label="CP Axtra Careers — หน้าแรก"
      className={cn(
        "group/wordmark inline-flex items-center gap-2.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:ring-offset-4 focus-visible:ring-offset-background rounded-sm",
        className,
      )}
    >
      <span
        className={cn(
          "font-heading text-[1.05rem] font-bold leading-none tracking-[0.04em]",
          invert ? "text-primary-foreground" : "text-foreground",
        )}
      >
        CP&nbsp;AXTRA
      </span>
      <span aria-hidden="true" className={cn("h-4 w-px", invert ? "bg-primary-foreground/30" : "bg-line")} />
      <span
        className={cn(
          "text-sm font-medium tracking-wide",
          invert ? "text-primary-foreground/70" : "text-muted-foreground",
        )}
      >
        Careers
      </span>
    </Link>
  );
}
