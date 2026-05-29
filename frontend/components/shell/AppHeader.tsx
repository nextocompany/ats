"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";

import { signOut } from "@/lib/auth";
import { useMe } from "@/lib/queries";
import { Button } from "@/components/ui/button";

const NAV = [
  { href: "/dashboard", label: "Overview" },
  { href: "/applications", label: "Inbox" },
  { href: "/candidates", label: "Candidates" },
  { href: "/search", label: "Search" },
  { href: "/analytics", label: "Analytics" },
];

export function AppHeader() {
  const pathname = usePathname();
  const router = useRouter();
  const { data: me } = useMe();

  return (
    <header className="sticky top-0 z-20 border-b bg-background/95 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-[1400px] items-center gap-6 px-4">
        <Link href="/applications" className="text-sm font-bold tracking-tight">
          HR<span className="text-[var(--color-accent)]">·</span>ATS
        </Link>
        <nav aria-label="Main navigation" className="flex items-center gap-1 text-sm">
          {NAV.map((item) => {
            const active = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`rounded-md px-3 py-1.5 transition-colors ${
                  active ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
        <div className="ml-auto flex items-center gap-3">
          <span className="hidden text-xs text-muted-foreground sm:inline">{me?.email ?? "…"}</span>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              signOut();
              router.push("/login");
            }}
          >
            Sign out
          </Button>
        </div>
      </div>
    </header>
  );
}
