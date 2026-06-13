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
    return <span className="size-8 animate-pulse rounded-full bg-muted" aria-hidden="true" />;
  }

  if (isAuthenticated && candidate) {
    const label = candidate.full_name?.trim() || "บัญชีของฉัน";
    return (
      <Link
        href="/account"
        className="inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-foreground/80 transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <span className="grid size-7 place-content-center rounded-full bg-accent/15 text-xs font-bold text-accent">
          {label.charAt(0).toUpperCase()}
        </span>
        {!compact ? <span className="max-w-[8rem] truncate">{label}</span> : null}
      </Link>
    );
  }

  return (
    <Link href="/login" className={cn(buttonVariants({ size: compact ? "sm" : "tap", variant: "outline" }))}>
      เข้าสู่ระบบ
    </Link>
  );
}
