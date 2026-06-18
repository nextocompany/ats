import {
  LayoutDashboard,
  Inbox,
  Users,
  UserCog,
  UploadCloud,
  Search,
  BarChart3,
  LineChart,
  ClipboardCheck,
  CheckSquare,
  Settings,
  type LucideIcon,
} from "lucide-react";

import { canAccessApprovals, canBulkUpload, canViewExecutive, isLineManager } from "@/lib/roles";

export interface NavItem {
  href: string;
  label: string; // English fallback; UI renders t(`nav.${key}`)
  key: string; // i18n key under the "nav" namespace
  icon: LucideIcon;
}

// Single source of truth for primary navigation across sidebar + drawer.
export const NAV: NavItem[] = [
  { href: "/dashboard", label: "Overview", key: "overview", icon: LayoutDashboard },
  { href: "/applications", label: "Inbox", key: "inbox", icon: Inbox },
  { href: "/candidates", label: "Candidates", key: "candidates", icon: Users },
  { href: "/search", label: "Search", key: "search", icon: Search },
  { href: "/analytics", label: "Analytics", key: "analytics", icon: BarChart3 },
];

// Bulk upload is for HR roles that add candidates (super_admin/hr_manager/sgm/
// hr_staff) — gated via canBulkUpload, mirroring the backend allowlist.
export const BULK_NAV: NavItem = { href: "/applications/bulk", label: "Bulk upload", key: "bulkUpload", icon: UploadCloud };

// Executive is the company-wide leadership overview — super_admin/regional_director/
// auditor (KindAll roles), gated via canViewExecutive mirroring the backend allowlist.
export const EXECUTIVE_NAV: NavItem = { href: "/executive", label: "Executive", key: "executive", icon: LineChart };

// Shortlist is the Line Manager's Top-5 review queue — gated to sgm (store GM).
export const SHORTLIST_NAV: NavItem = { href: "/shortlist", label: "Shortlist", key: "shortlist", icon: ClipboardCheck };

// Approvals is the multi-level hiring sign-off queue — any chain role (hr_staff/
// hr_manager/sgm/regional_director) + super_admin, gated via canAccessApprovals.
export const APPROVALS_NAV: NavItem = { href: "/approvals", label: "Approvals", key: "approvals", icon: CheckSquare };

// Members is super_admin + hr_manager — career-portal member management.
export const MEMBERS_NAV: NavItem = { href: "/members", label: "Members", key: "members", icon: UserCog };

// Admin is super_admin-only — appended via navForRole, never in the base NAV.
export const ADMIN_NAV: NavItem = { href: "/admin", label: "Admin", key: "admin", icon: Settings };

// navForRole returns the nav items visible to a given role. Bulk upload for HR
// uploader roles; super_admin + hr_manager see Members; super_admin also sees Admin.
export function navForRole(role?: string): NavItem[] {
  const items = [...NAV];
  if (canViewExecutive(role)) items.push(EXECUTIVE_NAV);
  if (isLineManager(role)) items.push(SHORTLIST_NAV);
  if (canAccessApprovals(role)) items.push(APPROVALS_NAV);
  if (canBulkUpload(role)) items.push(BULK_NAV);
  if (role === "super_admin" || role === "hr_manager") items.push(MEMBERS_NAV);
  if (role === "super_admin") items.push(ADMIN_NAV);
  return items;
}

// ALL_NAV is every possible item, for pathname→item lookups (e.g. header title).
export const ALL_NAV: NavItem[] = [...NAV, EXECUTIVE_NAV, SHORTLIST_NAV, APPROVALS_NAV, BULK_NAV, MEMBERS_NAV, ADMIN_NAV];

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
