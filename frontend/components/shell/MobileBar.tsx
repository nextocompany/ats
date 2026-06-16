"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { Menu, X, LogOut } from "lucide-react";

import { signOut } from "@/lib/auth";
import { useMe } from "@/lib/queries";
import { navForRole, BrandMark } from "./nav-config";

// Mobile / tablet chrome (<1024): top bar + slide-in drawer.
export function MobileBar() {
  const pathname = usePathname();
  const router = useRouter();
  const tNav = useTranslations("nav");
  const { data: me } = useMe();
  const [open, setOpen] = useState(false);

  return (
    <div className="lg:hidden">
      <header className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-hairline bg-card/90 px-4 backdrop-blur">
        <button
          type="button"
          aria-label="Open menu"
          aria-expanded={open}
          onClick={() => setOpen(true)}
          className="grid size-9 place-items-center rounded-md text-foreground transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Menu className="size-5" />
        </button>
        <BrandMark />
      </header>

      {/* Drawer */}
      {open && (
        <div className="fixed inset-0 z-40">
          <div
            className="absolute inset-0 bg-foreground/40 backdrop-blur-sm"
            onClick={() => setOpen(false)}
            aria-hidden
          />
          <div className="absolute inset-y-0 left-0 flex w-72 max-w-[85vw] flex-col bg-sidebar text-sidebar-foreground shadow-2xl settle">
            <div className="flex h-14 items-center justify-between px-5">
              <BrandMark tone="dark" />
              <button
                type="button"
                aria-label="Close menu"
                onClick={() => setOpen(false)}
                className="grid size-8 place-items-center rounded-md text-sidebar-foreground/70 hover:bg-sidebar-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
              >
                <X className="size-5" />
              </button>
            </div>
            <div className="mx-5 h-px bg-sidebar-border" />
            <nav aria-label="Main navigation" className="flex-1 px-3 py-4">
              <ul className="flex flex-col gap-0.5">
                {navForRole(me?.role).map((item) => {
                  const active = pathname.startsWith(item.href);
                  const Icon = item.icon;
                  return (
                    <li key={item.href}>
                      <Link
                        href={item.href}
                        aria-current={active ? "page" : undefined}
                        onClick={() => setOpen(false)}
                        className={`flex items-center gap-3 rounded-md px-3 py-2.5 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring ${
                          active
                            ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                            : "text-sidebar-foreground/75 hover:bg-sidebar-accent/50"
                        }`}
                      >
                        <Icon
                          className={`size-[1.05rem] ${active ? "text-sidebar-primary" : "text-sidebar-foreground/55"}`}
                          strokeWidth={active ? 2.25 : 1.75}
                        />
                        {tNav(item.key)}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            </nav>
            <div className="mx-5 h-px bg-sidebar-border" />
            <div className="flex items-center gap-3 p-4">
              <span className="grid size-8 place-items-center rounded-full bg-sidebar-accent text-xs font-semibold uppercase text-sidebar-primary">
                {(me?.email ?? "HR").slice(0, 2)}
              </span>
              <p className="min-w-0 flex-1 truncate text-xs text-sidebar-foreground">{me?.email ?? "…"}</p>
              <button
                type="button"
                aria-label="Sign out"
                onClick={() => {
                  signOut();
                  router.push("/login");
                }}
                className="grid size-7 place-items-center rounded-md text-sidebar-foreground/55 hover:bg-sidebar-accent hover:text-sidebar-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
              >
                <LogOut className="size-[0.95rem]" />
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
