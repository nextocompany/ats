import {
  LayoutDashboard,
  Inbox,
  Users,
  Search,
  BarChart3,
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

// Brand lockup — blue monogram + wordmark, shared portal identity.
export function BrandMark({ tone = "light" }: { tone?: "light" | "dark" }) {
  const ink = tone === "dark" ? "text-sidebar-foreground" : "text-foreground";
  return (
    <div className="flex items-center gap-2.5">
      <span
        aria-hidden
        className="grid size-8 shrink-0 place-items-center rounded-md bg-brand font-semibold text-brand-foreground"
        style={{ boxShadow: "inset 0 0 0 1px oklch(100% 0 0 / 0.12)" }}
      >
        <span className="text-[0.95rem] leading-none tracking-tight">HR</span>
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
