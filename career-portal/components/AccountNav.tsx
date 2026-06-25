"use client";

import Link from "next/link";

import { buttonVariants } from "@/components/ui/button";
import { useCandidate } from "@/lib/session";
import { cn } from "@/lib/utils";

// AccountNav is the auth-aware header affordance: an account link when logged in,
// otherwise a sign-in link. Rendered inside the (server) SiteHeader.
export function AccountNav({ compact }: { compact?: boolean }) {
  const { candidate, isAuthenticated, isLoading } = useCandidate();

  if (isLoading) {
    return <span className="size-9 animate-pulse rounded-full bg-surface-muted" aria-hidden="true" />;
  }

  if (isAuthenticated && candidate) {
    const label = candidate.display_name?.trim() || candidate.full_name?.trim() || "บัญชีของฉัน";
    return (
      <Link
        href="/account"
        className="inline-flex items-center gap-2 rounded-lg px-2.5 py-2 text-sm font-medium text-foreground/80 transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
      >
        <span className="grid size-8 place-content-center rounded-full border border-line bg-secondary text-xs font-semibold text-foreground">
          {label.charAt(0).toUpperCase()}
        </span>
        {!compact ? <span className="max-w-[8rem] truncate">{label}</span> : null}
      </Link>
    );
  }

  return (
    <Link href="/login" className={cn(buttonVariants({ size: compact ? "sm" : "default", variant: "outline" }))}>
      เข้าสู่ระบบ
    </Link>
  );
}
