"use client";

import Link from "next/link";
import { use } from "react";

import { ApplyStepper } from "@/components/ApplyStepper";
import { PortalShell } from "@/components/PortalShell";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { usePublicPosition } from "@/lib/queries";

export default function ApplyPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: position, isLoading, isError } = usePublicPosition(id);

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
          <ApplyStepper positionId={position.id} positionTitle={position.title_th} />
        </div>
      ) : null}
    </PortalShell>
  );
}
