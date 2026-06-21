"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { LogOut } from "lucide-react";

import { signOut } from "@/lib/auth";
import { useMe } from "@/lib/queries";
import { navForRole, BrandMark, PRIVACY_NAV } from "./nav-config";
import type { Me } from "@/lib/types";

const ROLE_LABELS: Record<string, string> = {
  superadmin: "Super Admin",
  admin: "Administrator",
  hr: "HR Specialist",
  recruiter: "Recruiter",
  manager: "Hiring Manager",
  staff: "Staff",
};

// Derive a presentable identity from the raw account — a Title-Cased human name
// from the email local-part (drops the domain so the spine never shows a
// "name@lo…" truncation), plus a friendly role label and avatar initials.
function deriveIdentity(me: Me | undefined): { name: string; role: string; initials: string } {
  if (!me) return { name: "Loading…", role: "Staff", initials: "HR" };
  const local = (me.email ?? "").split("@")[0] ?? "";
  const words = local.split(/[._-]+/).filter(Boolean);
  const name = words.length
    ? words.map((w) => w[0].toUpperCase() + w.slice(1)).join(" ")
    : me.email || "HR User";
  const initials =
    (words[0]?.[0] ?? "H").toUpperCase() + (words[1]?.[0] ?? words[0]?.[1] ?? "R").toUpperCase();
  const roleKey = (me.role ?? "").toLowerCase();
  const role = ROLE_LABELS[roleKey] ?? (me.role ? me.role[0].toUpperCase() + me.role.slice(1) : "Staff");
  return { name, role, initials };
}

// Persistent left sidebar — the deep navy-blue spine of the console (desktop ≥1024).
export function SideNav() {
  const pathname = usePathname();
  const router = useRouter();
  const tNav = useTranslations("nav");
  const { data: me } = useMe();
  const identity = deriveIdentity(me);

  return (
    <aside className="sticky top-0 hidden h-dvh w-64 shrink-0 flex-col overflow-hidden bg-sidebar text-sidebar-foreground lg:flex print:hidden">
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
          {navForRole(me).map((item) => {
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
                  <span className="relative">{tNav(item.key)}</span>
                </Link>
              </li>
            );
          })}
        </ul>

        {/* Hairline closing the nav block */}
        <div className="mt-7 ml-3 h-px w-10 bg-sidebar-border" aria-hidden />

        {/* Secondary: privacy notice + DPO reference, open to all. A quiet footer
            link, deliberately outside the primary Workspace list. */}
        <Link
          href={PRIVACY_NAV.href}
          aria-current={pathname.startsWith(PRIVACY_NAV.href) ? "page" : undefined}
          className="mt-3 flex items-center gap-3 rounded-lg px-3 py-2 text-xs text-sidebar-foreground/55 transition-colors outline-none hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring"
        >
          <PRIVACY_NAV.icon className="size-[0.95rem] shrink-0" strokeWidth={1.75} />
          <span>{tNav(PRIVACY_NAV.key)}</span>
        </Link>
      </nav>

      {/* Identity + sign-out */}
      <div className="relative mx-5 h-px bg-sidebar-border" />
      <div className="relative p-3">
        <div className="flex items-center gap-3 rounded-md px-3 py-2">
          <span
            aria-hidden
            className="grid size-8 shrink-0 place-items-center rounded-full bg-sidebar-accent text-xs font-semibold uppercase text-sidebar-primary"
          >
            {identity.initials}
          </span>
          <div className="min-w-0 flex-1">
            {/* Primary line: a clean human name derived from the email local-part,
                so the spine never reads as a truncated "dev.superadmin@lo…" blob. */}
            <p className="truncate text-xs font-semibold text-sidebar-foreground" title={me?.email ?? undefined}>
              {identity.name}
            </p>
            <p
              className="truncate text-[0.625rem] uppercase tracking-wide text-sidebar-foreground/45"
              title={me?.email ?? undefined}
            >
              {identity.role}
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
