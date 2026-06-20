"use client";

import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { ALL_NAV } from "./nav-config";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";

// Slim desktop context bar above the page — orients the operator without
// duplicating the sidebar's navigation landmark. Hidden on mobile (MobileBar covers it).
// A live ticking clock + brand pulse signal a console that's reading the pipeline now.
export function AppHeader() {
  const pathname = usePathname();
  const tNav = useTranslations("nav");
  const active = ALL_NAV.find((n) => pathname.startsWith(n.href));

  const [now, setNow] = useState<Date | null>(null);
  useEffect(() => {
    setNow(new Date());
    const id = setInterval(() => setNow(new Date()), 1000);
    return () => clearInterval(id);
  }, []);

  const stamp = now?.toLocaleDateString("en-GB", {
    weekday: "short",
    day: "2-digit",
    month: "short",
  });
  const time = now?.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
  });

  return (
    <div className="sticky top-0 z-20 hidden h-16 items-center justify-between border-b border-hairline bg-background/80 px-8 backdrop-blur-md lg:flex print:hidden">
      <nav aria-label="Breadcrumb" className="flex items-baseline gap-2 text-sm">
        <span className="text-muted-foreground">Console</span>
        <span className="text-muted-foreground/40">/</span>
        <span className="font-medium text-foreground">{active ? tNav(active.key) : tNav("overview")}</span>
      </nav>
      <div className="flex items-center gap-5">
        <LocaleSwitcher />
        <span className="hidden items-center gap-2 text-xs font-medium tabular-nums text-muted-foreground xl:flex">
          {/* Pulsing brand dot — the console is live */}
          <span className="relative flex size-2" aria-hidden>
            <span className="absolute inline-flex size-full animate-ping rounded-full bg-brand opacity-60 motion-reduce:hidden" />
            <span className="relative inline-flex size-2 rounded-full bg-brand" />
          </span>
          Live
        </span>
        <span className="h-4 w-px bg-hairline" aria-hidden />
        <time className="text-xs tabular-nums text-foreground/80" suppressHydrationWarning>
          <span className="text-muted-foreground">{stamp ?? "-"}</span>
          <span className="ml-2 font-semibold">{time ?? "··:··"}</span>
        </time>
      </div>
    </div>
  );
}
