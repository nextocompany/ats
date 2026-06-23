import {
  LayoutDashboard,
  Inbox,
  Users,
  UploadCloud,
  Search,
  BarChart3,
  LineChart,
  FileBarChart,
  ClipboardCheck,
  ClipboardList,
  CheckSquare,
  ShieldCheck,
  Lock,
  Settings,
  SlidersHorizontal,
  type LucideIcon,
} from "lucide-react";

import {
  canAccessApprovals,
  canAdminPdpa,
  canBulkUpload,
  canManageRequisitions,
  canManageScoring,
  canViewExecutive,
  canViewReports,
  isLineManager,
  isSuperAdmin,
} from "@/lib/roles";
import type { Me } from "@/lib/types";

export interface NavItem {
  href: string;
  label: string; // English fallback; UI renders t(`nav.${key}`)
  key: string; // i18n key under the "nav" namespace
  icon: LucideIcon;
}

// Single source of truth for primary navigation across sidebar + drawer.
export const NAV: NavItem[] = [
  { href: "/dashboard", label: "Overview", key: "overview", icon: LayoutDashboard },
  { href: "/applications", label: "Applications", key: "inbox", icon: Inbox },
  { href: "/candidates", label: "Candidates", key: "candidates", icon: Users },
  { href: "/search", label: "Search", key: "search", icon: Search },
  { href: "/analytics", label: "Analytics", key: "analytics", icon: BarChart3 },
];

// Bulk upload is for HR roles that add candidates (super_admin/hr_manager/sgm/
// hr_staff) — gated via canBulkUpload, mirroring the backend allowlist.
export const BULK_NAV: NavItem = { href: "/applications/bulk", label: "Bulk upload", key: "bulkUpload", icon: UploadCloud };

// Reports is the HR-facing ATS Reports page (recruitment-funnel metrics, RBAC-scoped) —
// most HR roles, gated via canViewReports mirroring the backend allowlist.
export const REPORTS_NAV: NavItem = { href: "/reports", label: "Reports", key: "reports", icon: FileBarChart };

// Executive is the company-wide leadership overview — super_admin/regional_director/
// auditor (KindAll roles), gated via canViewExecutive mirroring the backend allowlist.
export const EXECUTIVE_NAV: NavItem = { href: "/executive", label: "Executive", key: "executive", icon: LineChart };

// Shortlist is the Line Manager's Top-5 review queue — gated to sgm (store GM).
export const SHORTLIST_NAV: NavItem = { href: "/shortlist", label: "Shortlist", key: "shortlist", icon: ClipboardCheck };

// Approvals is the multi-level hiring sign-off queue — any chain role (hr_staff/
// hr_manager/sgm/regional_director) + super_admin, gated via canAccessApprovals.
export const APPROVALS_NAV: NavItem = { href: "/approvals", label: "Approvals", key: "approvals", icon: CheckSquare };

// Requisitions is the position-opening management queue — manage roles
// (super_admin/regional_director/operation_director/sgm/hr_manager), gated via
// canManageRequisitions mirroring the backend allowlist.
export const REQUISITIONS_NAV: NavItem = { href: "/requisitions", label: "Requisitions", key: "requisitions", icon: ClipboardList };

// PDPA is the DPO/PDPA console (DSAR queue, consent lookup, compliance overview):
// gated via canAdminPdpa (pdpa.admin), mirroring the backend allowlist.
export const PDPA_NAV: NavItem = { href: "/pdpa", label: "PDPA", key: "pdpa", icon: ShieldCheck };

// Admin is super_admin-only — appended via navForRole, never in the base NAV.
export const ADMIN_NAV: NavItem = { href: "/admin", label: "Admin", key: "admin", icon: Settings };

// Scoring weights — per-position screening-weight config, gated via canManageScoring
// (settings.admin), mirroring the backend allowlist.
export const SCORING_NAV: NavItem = { href: "/scoring", label: "Scoring", key: "scoring", icon: SlidersHorizontal };

// Privacy is the internal privacy-notice + DPO-contact reference, open to every
// authenticated user. It is a SECONDARY (footer) link, not part of the primary
// Workspace nav, so it is in ALL_NAV (for breadcrumb resolution) but not navForRole.
export const PRIVACY_NAV: NavItem = { href: "/privacy", label: "Privacy", key: "privacy", icon: Lock };

// navForRole returns the nav items visible to a given role. Bulk upload for HR
// uploader roles; super_admin + hr_manager see Members; super_admin also sees Admin.
export function navForRole(me?: Me): NavItem[] {
  const items = [...NAV];
  if (canViewReports(me)) items.push(REPORTS_NAV);
  if (canViewExecutive(me)) items.push(EXECUTIVE_NAV);
  if (isLineManager(me)) items.push(SHORTLIST_NAV);
  if (canAccessApprovals(me)) items.push(APPROVALS_NAV);
  if (canBulkUpload(me)) items.push(BULK_NAV);
  if (canManageRequisitions(me)) items.push(REQUISITIONS_NAV);
  if (canAdminPdpa(me)) items.push(PDPA_NAV);
  if (canManageScoring(me)) items.push(SCORING_NAV);
  if (isSuperAdmin(me)) items.push(ADMIN_NAV);
  return items;
}

// ALL_NAV is every possible item, for pathname→item lookups (e.g. header title).
export const ALL_NAV: NavItem[] = [...NAV, REPORTS_NAV, EXECUTIVE_NAV, SHORTLIST_NAV, APPROVALS_NAV, BULK_NAV, REQUISITIONS_NAV, PDPA_NAV, SCORING_NAV, ADMIN_NAV, PRIVACY_NAV];

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
