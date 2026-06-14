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

// Brand lockup — blue monogram + wordmark, shared portal identity.
// The monogram carries a brass corner-dot: a micro CP Axtra signature.
export function BrandMark({ tone = "light" }: { tone?: "light" | "dark" }) {
  const ink = tone === "dark" ? "text-sidebar-foreground" : "text-foreground";
  const ringColor = tone === "dark" ? "var(--sidebar)" : "var(--card)";
  return (
    <div className="flex items-center gap-2.5">
      <span
        aria-hidden
        className="relative grid size-8 shrink-0 place-items-center rounded-[0.6rem] bg-brand font-semibold text-brand-foreground"
        style={{ boxShadow: "inset 0 0 0 1px oklch(100% 0 0 / 0.14), 0 2px 6px -2px oklch(46% 0.18 264 / 0.5)" }}
      >
        <span className="text-[0.95rem] leading-none tracking-tight">HR</span>
        {/* Brass dot — the brand signature, tucked at the corner */}
        <span
          className="absolute -right-0.5 -top-0.5 size-2 rounded-full bg-brass"
          style={{ boxShadow: `0 0 0 2px ${ringColor}` }}
        />
      </span>
      <span className={`flex flex-col leading-none ${ink}`}>
        <span className="text-[0.95rem] font-semibold tracking-tight">
          ATS Console
        </span>
        <span className="mt-0.5 text-[0.625rem] font-medium uppercase tracking-[0.18em] text-brass">
          Recruitment
        </span>
      </span>
    </div>
  );
}
