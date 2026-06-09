"use client";

import Link from "next/link";

import { JobCard } from "@/components/JobCard";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { usePublicPositions } from "@/lib/queries";

// FeaturedJobs is the only live (client) part of the landing — it shows the top
// open positions and links to the full list. Loading/empty states mirror the
// jobs page so the section never renders broken.
export function FeaturedJobs() {
  const { data: positions, isLoading } = usePublicPositions();
  const featured = positions?.slice(0, 6) ?? [];

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="space-y-2">
          <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">ร่วมงานกับเรา</p>
          <h2 className="text-[length:var(--text-display)] font-bold tracking-tight">ตำแหน่งงานแนะนำ</h2>
        </div>
        <Link href="/jobs" className={cn(buttonVariants({ variant: "outline" }), "h-10 px-5")}>
          ดูทั้งหมด
        </Link>
      </div>

      {isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" aria-hidden="true">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-[164px] w-full rounded-2xl" />
          ))}
        </div>
      ) : featured.length > 0 ? (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {featured.map((p) => (
            <li key={p.id}>
              <JobCard position={p} />
            </li>
          ))}
        </ul>
      ) : (
        <p className="rounded-2xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">
          ขณะนี้ยังไม่มีตำแหน่งที่เปิดรับ โปรดกลับมาตรวจสอบอีกครั้ง
        </p>
      )}
    </div>
  );
}
