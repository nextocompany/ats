"use client";

import Link from "next/link";
import { use, useEffect } from "react";
import { useRouter } from "next/navigation";

import { ApplyStepper } from "@/components/ApplyStepper";
import { PortalShell } from "@/components/PortalShell";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { usePublicPosition } from "@/lib/queries";
import { useCandidate } from "@/lib/session";

// Apply is account-first: an unauthenticated visitor is sent to /login (returning
// here after); a logged-in member sees the prefilled apply flow.
export default function ApplyPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const { data: position, isLoading, isError } = usePublicPosition(id);
  const { candidate, isAuthenticated, isLoading: authLoading } = useCandidate();

  const applyPath = `/jobs/${id}/apply`;
  useEffect(() => {
    if (!authLoading && !isAuthenticated) {
      router.replace(`/login?return=${encodeURIComponent(applyPath)}`);
    }
  }, [authLoading, isAuthenticated, applyPath, router]);

  const loading = isLoading || authLoading || (!isAuthenticated && !isError);

  return (
    <PortalShell backHref={`/jobs/${id}`} narrow>
      {loading ? (
        <div className="space-y-4">
          <Skeleton className="h-6 w-1/2" />
          <Skeleton className="h-2 w-full" />
          <Skeleton className="h-48 w-full rounded-2xl" />
        </div>
      ) : null}

      {isError || (!isLoading && !position) ? (
        <div className="space-y-4 rounded-2xl border border-border bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">ไม่พบตำแหน่งงานนี้</p>
          <Link href="/jobs" className={buttonVariants({ variant: "outline", size: "tap" })}>
            กลับไปดูตำแหน่งงานทั้งหมด
          </Link>
        </div>
      ) : null}

      {position && isAuthenticated && candidate ? (
        <div className="rounded-2xl border border-border bg-card p-6 sm:p-8">
          <ApplyStepper positionId={position.id} positionTitle={position.title_th} account={candidate} />
        </div>
      ) : null}
    </PortalShell>
  );
}
