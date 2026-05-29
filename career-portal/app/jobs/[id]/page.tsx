"use client";

import Link from "next/link";
import { use } from "react";

import { PortalShell } from "@/components/PortalShell";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { usePublicPosition } from "@/lib/queries";
import { cn } from "@/lib/utils";

const LEVEL_LABELS: Record<string, string> = {
  entry: "ระดับเริ่มต้น",
  experienced: "มีประสบการณ์",
  senior: "ระดับอาวุโส",
  management: "ระดับบริหาร",
};

export default function JobDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: position, isLoading, isError } = usePublicPosition(id);

  return (
    <PortalShell backHref="/jobs">
      {isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-8 w-2/3" />
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="h-32 w-full rounded-2xl" />
        </div>
      ) : null}

      {isError || (!isLoading && !position) ? (
        <div className="space-y-4 rounded-2xl bg-card p-6 text-center ring-1 ring-foreground/10">
          <p className="text-sm text-muted-foreground">ไม่พบตำแหน่งงานนี้</p>
          <Link href="/jobs" className={buttonVariants({ variant: "outline", size: "tap" })}>
            กลับไปดูตำแหน่งงานทั้งหมด
          </Link>
        </div>
      ) : null}

      {position ? (
        <div className="flex min-h-[60dvh] flex-col">
          <div className="space-y-4">
            <div className="space-y-2">
              <h1 className="text-2xl font-bold tracking-tight">{position.title_th}</h1>
              {position.title_en ? <p className="text-sm text-muted-foreground">{position.title_en}</p> : null}
            </div>
            {position.level ? (
              <span className="inline-flex rounded-full bg-accent px-3 py-1 text-sm font-medium text-accent-foreground">
                {LEVEL_LABELS[position.level.toLowerCase()] ?? position.level}
              </span>
            ) : null}
            <div className="space-y-2 rounded-2xl bg-card p-5 text-sm leading-relaxed text-foreground/80 ring-1 ring-foreground/10">
              <p>
                ร่วมเป็นส่วนหนึ่งของทีมเรา เรามองหาผู้ที่มีความตั้งใจและพร้อมเรียนรู้
                สมัครง่าย ๆ เพียงไม่กี่ขั้นตอน แล้วทีม HR จะติดต่อกลับ
              </p>
            </div>
          </div>

          {/* Sticky-ish primary CTA at the bottom of the column */}
          <div className="mt-auto pt-8">
            <Link href={`/jobs/${position.id}/apply`} className={cn(buttonVariants({ size: "tap" }), "w-full")}>
              สมัครงาน
            </Link>
          </div>
        </div>
      ) : null}
    </PortalShell>
  );
}
