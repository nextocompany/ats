import Link from "next/link";

import { AccountNav } from "@/components/AccountNav";
import { Container } from "@/components/ds";
import { Wordmark } from "@/components/Wordmark";

interface SiteHeaderProps {
  // backHref renders a back chevron (focused inner flows like apply/login); when
  // set, the desktop nav is hidden to keep the flow uncluttered.
  backHref?: string;
}

const NAV = [
  { href: "/jobs", label: "ตำแหน่งงาน" },
  { href: "/status", label: "ตรวจสอบสถานะ" },
];

// SiteHeader is the slim institutional top chrome: a text wordmark on the left,
// a restrained nav + account affordance on the right. Sticky with a single
// hairline rule and a quiet backdrop — no shadow, no blur drama.
export function SiteHeader({ backHref }: SiteHeaderProps) {
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-background/90 backdrop-blur-sm">
      <Container className="flex h-16 items-center gap-4">
        {backHref ? (
          <Link
            href={backHref}
            aria-label="ย้อนกลับ"
            className="-ml-1.5 inline-flex size-11 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path d="M15 18l-6-6 6-6" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          </Link>
        ) : null}

        <Wordmark />

        {!backHref ? (
          <>
            <nav aria-label="เมนูหลัก" className="ml-auto hidden items-center gap-1 md:flex">
              {NAV.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  className="rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
                >
                  {item.label}
                </Link>
              ))}
              <span aria-hidden="true" className="mx-1 h-5 w-px bg-line" />
              <AccountNav />
            </nav>

            {/* Mobile: a single jobs quick link + the account affordance. */}
            <div className="ml-auto flex items-center gap-1 md:hidden">
              <Link
                href="/jobs"
                className="rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
              >
                ตำแหน่งงาน
              </Link>
              <AccountNav compact />
            </div>
          </>
        ) : null}
      </Container>
    </header>
  );
}
