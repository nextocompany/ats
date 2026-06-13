import Link from "next/link";

import { AccountNav } from "@/components/AccountNav";
import { Container } from "@/components/Container";

interface SiteHeaderProps {
  // backHref renders a back chevron (inner pages); when set, the desktop nav is
  // hidden to keep the focused flow uncluttered.
  backHref?: string;
}

const NAV = [
  { href: "/jobs", label: "ตำแหน่งงาน" },
  { href: "/status", label: "ตรวจสอบสถานะ" },
];

// SiteHeader is the responsive top chrome: brand on the left, desktop nav + CTA
// on the right, compact on mobile. Sticky with a hairline border and blur.
export function SiteHeader({ backHref }: SiteHeaderProps) {
  return (
    <header className="sticky top-0 z-30 border-b border-border/70 bg-background/80 backdrop-blur-md">
      <Container className="flex h-16 items-center gap-3">
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

        <Link href="/" className="flex items-center gap-2.5 font-semibold tracking-tight">
          <span className="grid size-8 place-content-center rounded-lg bg-accent text-sm font-bold text-accent-foreground">
            N
          </span>
          <span className="text-[0.95rem]">ร่วมงานกับเรา</span>
        </Link>

        {!backHref ? (
          <nav aria-label="เมนูหลัก" className="ml-auto hidden items-center gap-1 md:flex">
            {NAV.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className="rounded-lg px-3 py-2 text-sm font-medium text-foreground/70 transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                {item.label}
              </Link>
            ))}
            <AccountNav />
          </nav>
        ) : null}

        {/* Mobile: jobs quick link + account affordance (no menu drawer needed). */}
        {!backHref ? (
          <div className="ml-auto flex items-center gap-1 md:hidden">
            <Link
              href="/jobs"
              className="rounded-lg px-3 py-2 text-sm font-medium text-foreground/70 transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
            >
              ตำแหน่งงาน
            </Link>
            <AccountNav compact />
          </div>
        ) : null}
      </Container>
    </header>
  );
}
