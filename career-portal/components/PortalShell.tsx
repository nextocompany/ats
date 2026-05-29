import Link from "next/link";

import { cn } from "@/lib/utils";

interface PortalShellProps {
  children: React.ReactNode;
  // showBack renders a back chevron linking to `backHref`.
  backHref?: string;
  className?: string;
}

// PortalShell is the single-column mobile chrome shared by every page: a warm
// branded header bar and a width-capped main column. Keeps the layout consistent
// and the LINE in-app browser comfortable on 320–768px.
export function PortalShell({ children, backHref, className }: PortalShellProps) {
  return (
    <div className="flex min-h-dvh flex-col">
      <header className="sticky top-0 z-10 border-b border-border/70 bg-background/85 backdrop-blur">
        <div className="mx-auto flex h-14 w-full max-w-screen-sm items-center gap-3 px-4">
          {backHref ? (
            <Link
              href={backHref}
              aria-label="ย้อนกลับ"
              className="-ml-1 inline-flex size-11 items-center justify-center rounded-full text-foreground/70 transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
            >
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M15 18l-6-6 6-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </Link>
          ) : null}
          <Link href="/jobs" className="flex items-center gap-2 font-semibold tracking-tight">
            <span className="grid size-7 place-content-center rounded-lg bg-primary text-primary-foreground text-sm">N</span>
            <span>ร่วมงานกับเรา</span>
          </Link>
        </div>
      </header>
      <main className={cn("mx-auto w-full max-w-screen-sm flex-1 px-4 py-6", className)}>{children}</main>
      <footer className="mx-auto w-full max-w-screen-sm px-4 py-8 text-center text-xs text-muted-foreground">
        ข้อมูลของคุณได้รับการคุ้มครองตาม พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล (PDPA)
      </footer>
    </div>
  );
}
