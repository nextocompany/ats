"use client";

import Link from "next/link";
import { use, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

import { ApplyStepper, type ApplyPrefill } from "@/components/ApplyStepper";
import { PortalShell } from "@/components/PortalShell";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { usePublicPosition } from "@/lib/queries";
import { useCandidate } from "@/lib/session";

// Reads "Apply with SEEK" deep-link params (?seek_name=&seek_email=&seek_phone=
// &seek_province=) into a prefill. Blank values are dropped.
function readPrefill(sp: URLSearchParams): ApplyPrefill | undefined {
  const pick = (k: string) => sp.get(k)?.trim() || undefined;
  const prefill: ApplyPrefill = {
    fullName: pick("seek_name"),
    email: pick("seek_email"),
    phone: pick("seek_phone"),
    province: pick("seek_province"),
  };
  return Object.values(prefill).some(Boolean) ? prefill : undefined;
}

// Apply is account-first: an unauthenticated visitor is sent to /login (returning
// here after); a logged-in member sees the prefilled apply flow.
export default function ApplyPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: position, isLoading, isError } = usePublicPosition(id);
  const { candidate, isAuthenticated, isLoading: authLoading } = useCandidate();

  const prefill = readPrefill(searchParams);
  // Preserve the SEEK params across the login bounce so prefill survives the
  // round-trip back to this page.
  const search = searchParams.toString();
  const applyPath = search ? `/jobs/${id}/apply?${search}` : `/jobs/${id}/apply`;
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
        <div className="space-y-4 rounded-xl border border-line bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">ไม่พบตำแหน่งงานนี้</p>
          <Link href="/jobs" className={buttonVariants({ variant: "outline", size: "tap" })}>
            กลับไปดูตำแหน่งงานทั้งหมด
          </Link>
        </div>
      ) : null}

      {position && isAuthenticated && candidate ? (
        <div className="rounded-xl border border-line bg-card p-6 sm:p-8">
          <ApplyStepper positionId={position.id} positionTitle={position.title_th} account={candidate} prefill={prefill} />
        </div>
      ) : null}
    </PortalShell>
  );
}
