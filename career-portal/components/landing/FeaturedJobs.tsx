"use client";

import Link from "next/link";

import { JobCard } from "@/components/JobCard";
import { SectionHeading } from "@/components/ds";
import { Skeleton } from "@/components/ui/skeleton";
import { usePublicPositions } from "@/lib/queries";

// FeaturedJobs is the only live (client) part of the landing — the top open
// positions with a link to the full browse. Loading/empty states mirror /jobs so
// the strip never renders broken.
export function FeaturedJobs() {
  const { data: positions, isLoading } = usePublicPositions();
  const featured = positions?.slice(0, 6) ?? [];

  return (
    <div className="flex flex-col gap-10">
      <div className="flex flex-wrap items-end justify-between gap-6">
        <SectionHeading eyebrow="ตำแหน่งที่เปิดรับ" heading="โอกาสร่วมงานล่าสุด" />
        <Link
          href="/jobs"
          className="text-sm font-medium text-primary underline-offset-4 transition-colors hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:rounded-lg"
        >
          ดูทั้งหมด &rarr;
        </Link>
      </div>

      {isLoading ? (
        <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3" aria-hidden="true">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-[168px] w-full" />
          ))}
        </div>
      ) : featured.length > 0 ? (
        <ul className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {featured.map((p) => (
            <li key={p.id}>
              <JobCard position={p} />
            </li>
          ))}
        </ul>
      ) : (
        <p className="rounded-xl border border-line bg-card p-10 text-center text-sm text-muted-foreground">
          ขณะนี้ยังไม่มีตำแหน่งที่เปิดรับ โปรดกลับมาตรวจสอบอีกครั้ง
        </p>
      )}
    </div>
  );
}
