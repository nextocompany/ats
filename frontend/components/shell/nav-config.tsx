import {
  LayoutDashboard,
  Inbox,
  Users,
  UserCog,
  Search,
  BarChart3,
  Settings,
  type LucideIcon,
} from "lucide-react";

export interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
}

// Single source of truth for primary navigation across sidebar + drawer.
export const NAV: NavItem[] = [
  { href: "/dashboard", label: "Overview", icon: LayoutDashboard },
  { href: "/applications", label: "Inbox", icon: Inbox },
  { href: "/candidates", label: "Candidates", icon: Users },
  { href: "/search", label: "Search", icon: Search },
  { href: "/analytics", label: "Analytics", icon: BarChart3 },
];

// Members is super_admin + hr_manager — career-portal member management.
export const MEMBERS_NAV: NavItem = { href: "/members", label: "Members", icon: UserCog };

// Admin is super_admin-only — appended via navForRole, never in the base NAV.
export const ADMIN_NAV: NavItem = { href: "/admin", label: "Admin", icon: Settings };

// navForRole returns the nav items visible to a given role. super_admin + hr_manager
// see Members; super_admin also sees Admin; everyone else gets the base workspace nav.
export function navForRole(role?: string): NavItem[] {
  const base = role === "super_admin" || role === "hr_manager" ? [...NAV, MEMBERS_NAV] : NAV;
  return role === "super_admin" ? [...base, ADMIN_NAV] : base;
}

// ALL_NAV is every possible item, for pathname→item lookups (e.g. header title).
export const ALL_NAV: NavItem[] = [...NAV, MEMBERS_NAV, ADMIN_NAV];

// Brand lockup — text-only institutional wordmark, no monogram tile or dot mark.
// "CP AXTRA" tracked uppercase over an "ATS Console" line (HSBC/JPM register),
// matching the careers Wordmark.
export function BrandMark({ tone = "light" }: { tone?: "light" | "dark" }) {
  const ink = tone === "dark" ? "text-sidebar-foreground" : "text-foreground";
  const muted = tone === "dark" ? "text-sidebar-foreground/60" : "text-muted-foreground";
  return (
    <span className={`flex flex-col leading-none ${ink}`}>
      <span className="text-[0.625rem] font-semibold uppercase tracking-[0.22em] text-brand">
        CP&nbsp;Axtra
      </span>
      <span className={`mt-1 text-[0.95rem] font-semibold tracking-tight ${muted}`}>
        ATS Console
      </span>
    </span>
  );
}
