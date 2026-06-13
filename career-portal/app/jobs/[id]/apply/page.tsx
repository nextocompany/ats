"use client";

import Link from "next/link";
import { use, useSyncExternalStore } from "react";

import { ApplyStepper } from "@/components/ApplyStepper";
import { LineGate } from "@/components/LineGate";
import { PortalShell } from "@/components/PortalShell";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { usePublicPosition } from "@/lib/queries";

// After LINE auth the backend redirects back with the id-token (or an error) in
// the URL fragment. Read it as an external store — no server roundtrip, no
// effect-driven setState — matching the interview page's hash-token pattern.
function subscribeHash(onChange: () => void): () => void {
  window.addEventListener("hashchange", onChange);
  return () => window.removeEventListener("hashchange", onChange);
}
function hashParam(key: string): string | null {
  return new URLSearchParams(window.location.hash.replace(/^#/, "")).get(key);
}

export default function ApplyPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: position, isLoading, isError } = usePublicPosition(id);

  const lineToken = useSyncExternalStore<string | null>(subscribeHash, () => hashParam("line_id_token"), () => null);
  const lineError = useSyncExternalStore<string | null>(subscribeHash, () => hashParam("line_error"), () => null);

  return (
    <PortalShell backHref={`/jobs/${id}`} narrow>
      {isLoading ? (
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

      {position ? (
        <div className="rounded-2xl border border-border bg-card p-6 sm:p-8">
          {lineToken ? (
            <ApplyStepper positionId={position.id} positionTitle={position.title_th} lineIdToken={lineToken} />
          ) : (
            <LineGate error={lineError} />
          )}
        </div>
      ) : null}
    </PortalShell>
  );
}
