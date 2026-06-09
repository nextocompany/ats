"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { LogOut } from "lucide-react";

import { signOut } from "@/lib/auth";
import { useMe } from "@/lib/queries";
import { NAV, BrandMark } from "./nav-config";

// Persistent left sidebar — the deep-emerald spine of the console (desktop ≥1024).
export function SideNav() {
  const pathname = usePathname();
  const router = useRouter();
  const { data: me } = useMe();

  return (
    <aside className="sticky top-0 hidden h-dvh w-64 shrink-0 flex-col bg-sidebar text-sidebar-foreground lg:flex">
      {/* Brand */}
      <div className="flex h-16 items-center px-5">
        <BrandMark tone="dark" />
      </div>

      <div className="mx-5 h-px bg-sidebar-border" />

      {/* Primary nav */}
      <nav aria-label="Main navigation" className="flex-1 px-3 py-5">
        <p className="px-3 pb-2 text-[0.625rem] font-semibold uppercase tracking-[0.18em] text-sidebar-foreground/45">
          Workspace
        </p>
        <ul className="flex flex-col gap-0.5">
          {NAV.map((item) => {
            const active = pathname.startsWith(item.href);
            const Icon = item.icon;
            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  aria-current={active ? "page" : undefined}
                  className={`group relative flex items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring ${
                    active
                      ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                      : "text-sidebar-foreground/75 hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground"
                  }`}
                >
                  {/* Brass active indicator — the signature accent moment */}
                  <span
                    aria-hidden
                    className={`absolute left-0 top-1/2 h-5 w-0.5 -translate-y-1/2 rounded-full bg-sidebar-primary transition-opacity ${
                      active ? "opacity-100" : "opacity-0"
                    }`}
                  />
                  <Icon
                    className={`size-[1.05rem] shrink-0 transition-colors ${
                      active ? "text-sidebar-primary" : "text-sidebar-foreground/55 group-hover:text-sidebar-foreground"
                    }`}
                    strokeWidth={active ? 2.25 : 1.75}
                  />
                  {item.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>

      {/* Identity + sign-out */}
      <div className="mx-5 h-px bg-sidebar-border" />
      <div className="p-3">
        <div className="flex items-center gap-3 rounded-md px-3 py-2">
          <span
            aria-hidden
            className="grid size-8 shrink-0 place-items-center rounded-full bg-sidebar-accent text-xs font-semibold uppercase text-sidebar-primary"
          >
            {(me?.email ?? "HR").slice(0, 2)}
          </span>
          <div className="min-w-0 flex-1">
            <p className="truncate text-xs font-medium text-sidebar-foreground">
              {me?.email ?? "Loading…"}
            </p>
            <p className="text-[0.625rem] uppercase tracking-wide text-sidebar-foreground/45">
              {me?.role ?? "Staff"}
            </p>
          </div>
          <button
            type="button"
            aria-label="Sign out"
            onClick={() => {
              signOut();
              router.push("/login");
            }}
            className="grid size-7 shrink-0 place-items-center rounded-md text-sidebar-foreground/55 transition-colors hover:bg-sidebar-accent hover:text-sidebar-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
          >
            <LogOut className="size-[0.95rem]" />
          </button>
        </div>
      </div>
    </aside>
  );
}
