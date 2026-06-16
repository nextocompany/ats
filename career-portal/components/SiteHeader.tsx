import Link from "next/link";
import { getTranslations } from "next-intl/server";

import { AccountNav } from "@/components/AccountNav";
import { Container } from "@/components/ds";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { Wordmark } from "@/components/Wordmark";

interface SiteHeaderProps {
  // backHref renders a back chevron (focused inner flows like apply/login); when
  // set, the desktop nav is hidden to keep the flow uncluttered.
  backHref?: string;
}

// SiteHeader is the slim institutional top chrome: a text wordmark on the left,
// a restrained nav + account affordance on the right. Sticky with a single
// hairline rule and a quiet backdrop — no shadow, no blur drama.
export async function SiteHeader({ backHref }: SiteHeaderProps) {
  const t = await getTranslations("nav");
  const nav = [
    { href: "/jobs", label: t("jobs") },
    { href: "/status", label: t("status") },
  ];
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-background/90 backdrop-blur-sm">
      <Container className="flex h-16 items-center gap-4">
        {backHref ? (
          <Link
            href={backHref}
            aria-label={t("back")}
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
            <nav aria-label={t("menu")} className="ml-auto hidden items-center gap-1 md:flex">
              {nav.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  className="rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
                >
                  {item.label}
                </Link>
              ))}
              <span aria-hidden="true" className="mx-1 h-5 w-px bg-line" />
              <LocaleSwitcher />
              <AccountNav />
            </nav>

            {/* Mobile: a single jobs quick link + the account affordance. */}
            <div className="ml-auto flex items-center gap-1 md:hidden">
              <Link
                href="/jobs"
                className="rounded-lg px-3 py-2 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
              >
                {t("jobs")}
              </Link>
              <LocaleSwitcher />
              <AccountNav compact />
            </div>
          </>
        ) : null}
      </Container>
    </header>
  );
}
