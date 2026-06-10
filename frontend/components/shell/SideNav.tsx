"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { LogOut } from "lucide-react";

import { signOut } from "@/lib/auth";
import { useMe } from "@/lib/queries";
import { NAV, BrandMark } from "./nav-config";

// Persistent left sidebar — the deep navy-blue spine of the console (desktop ≥1024).
export function SideNav() {
  const pathname = usePathname();
  const router = useRouter();
  const { data: me } = useMe();

  return (
    <aside className="sticky top-0 hidden h-dvh w-64 shrink-0 flex-col overflow-hidden bg-sidebar text-sidebar-foreground lg:flex">
      {/* Faint brand dot-dither — atmosphere on the navy spine, never input */}
      <span
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-[0.05]"
        style={{
          backgroundImage:
            "radial-gradient(oklch(100% 0 0 / 0.9) 0.7px, transparent 0.7px)",
          backgroundSize: "7px 7px",
        }}
      />
      {/* Brass top hairline — a quiet premium keyline at the crown of the spine */}
      <span aria-hidden className="absolute inset-x-0 top-0 h-px bg-sidebar-primary/40" />

      {/* Brand */}
      <div className="relative flex h-16 items-center px-5">
        <BrandMark tone="dark" />
      </div>

      <div className="relative mx-5 h-px bg-sidebar-border" />

      {/* Primary nav */}
      <nav aria-label="Main navigation" className="relative flex-1 px-3 py-5">
        <p className="px-3 pb-2.5 text-[0.625rem] font-semibold uppercase tracking-[0.18em] text-sidebar-foreground/45">
          Workspace
        </p>
        <ul className="flex flex-col gap-1">
          {NAV.map((item) => {
            const active = pathname.startsWith(item.href);
            const Icon = item.icon;
            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  aria-current={active ? "page" : undefined}
                  className={`group relative flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring ${
                    active
                      ? "font-medium text-sidebar-accent-foreground"
                      : "text-sidebar-foreground/75 hover:bg-sidebar-accent/45 hover:text-sidebar-accent-foreground"
                  }`}
                >
                  {/* Active pill — a raised inset surface so the current section
                      reads as a lifted tab, not just a flat tint */}
                  {active && (
                    <span
                      aria-hidden
                      className="absolute inset-0 rounded-lg bg-sidebar-accent"
                      style={{ boxShadow: "inset 0 0 0 1px oklch(100% 0 0 / 0.06), 0 1px 0 oklch(0% 0 0 / 0.25)" }}
                    />
                  )}
                  {/* Brass active indicator — the signature accent moment */}
                  <span
                    aria-hidden
                    className={`absolute left-0 top-1/2 h-6 w-[3px] -translate-y-1/2 rounded-r-full bg-sidebar-primary transition-all duration-200 ${
                      active ? "opacity-100" : "opacity-0 -translate-x-1"
                    }`}
                  />
                  <Icon
                    className={`relative size-[1.05rem] shrink-0 transition-colors ${
                      active ? "text-sidebar-primary" : "text-sidebar-foreground/55 group-hover:text-sidebar-foreground"
                    }`}
                    strokeWidth={active ? 2.25 : 1.75}
                  />
                  <span className="relative">{item.label}</span>
                </Link>
              </li>
            );
          })}
        </ul>

        {/* Signature dot-rule — the CP Axtra motif quietly closing the nav block */}
        <div className="dot-rule mt-7 ml-3 opacity-70" aria-hidden />
      </nav>

      {/* Identity + sign-out */}
      <div className="relative mx-5 h-px bg-sidebar-border" />
      <div className="relative p-3">
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
