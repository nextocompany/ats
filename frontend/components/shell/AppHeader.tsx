"use client";

import { usePathname } from "next/navigation";

import { NAV } from "./nav-config";

// Slim desktop context bar above the page — orients the operator without
// duplicating the sidebar's navigation landmark. Hidden on mobile (MobileBar covers it).
export function AppHeader() {
  const pathname = usePathname();
  const active = NAV.find((n) => pathname.startsWith(n.href));

  const now = new Date();
  const stamp = now.toLocaleDateString("en-GB", {
    weekday: "short",
    day: "2-digit",
    month: "short",
  });

  return (
    <div className="hidden h-16 items-center justify-between border-b border-hairline px-8 lg:flex">
      <div className="flex items-baseline gap-2 text-sm">
        <span className="text-muted-foreground">Console</span>
        <span className="text-muted-foreground/50">/</span>
        <span className="font-medium text-foreground">{active?.label ?? "Overview"}</span>
      </div>
      <div className="flex items-center gap-4">
        <span className="flex items-center gap-2 text-xs text-muted-foreground tabular-nums">
          <span className="inline-block size-1.5 rounded-full bg-brand" aria-hidden />
          Live · {stamp}
        </span>
      </div>
    </div>
  );
}
