"use client";

import Link from "next/link";

import { InstallPrompt } from "@/components/InstallPrompt";
import { JobCard } from "@/components/JobCard";
import { PortalShell } from "@/components/PortalShell";
import { Button, buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { usePublicPositions } from "@/lib/queries";

export default function JobsPage() {
  const { data: positions, isLoading, isError, refetch } = usePublicPositions();

  return (
    <PortalShell>
      <div className="space-y-6">
        <header className="space-y-2">
          <h1 className="text-2xl font-bold tracking-tight">ตำแหน่งงานที่เปิดรับ</h1>
          <p className="text-sm text-muted-foreground">เลือกตำแหน่งที่สนใจเพื่อดูรายละเอียดและสมัครงาน</p>
        </header>

        <InstallPrompt />

        {isLoading ? (
          <div className="space-y-3" aria-hidden="true">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-[76px] w-full rounded-2xl" />
            ))}
          </div>
        ) : null}

        {isError ? (
          <div className="space-y-4 rounded-2xl bg-card p-6 text-center ring-1 ring-foreground/10">
            <p className="text-sm text-muted-foreground">ไม่สามารถโหลดตำแหน่งงานได้ในขณะนี้</p>
            <Button size="tap" variant="outline" onClick={() => refetch()}>
              ลองอีกครั้ง
            </Button>
          </div>
        ) : null}

        {positions && positions.length === 0 ? (
          <div className="space-y-2 rounded-2xl bg-card p-8 text-center ring-1 ring-foreground/10">
            <p className="text-base font-medium">ยังไม่มีตำแหน่งงานที่เปิดรับ</p>
            <p className="text-sm text-muted-foreground">โปรดกลับมาตรวจสอบอีกครั้งในภายหลัง</p>
          </div>
        ) : null}

        {positions && positions.length > 0 ? (
          <ul className="space-y-3">
            {positions.map((p) => (
              <li key={p.id}>
                <JobCard position={p} />
              </li>
            ))}
          </ul>
        ) : null}

        <div className="pt-2 text-center">
          <Link href="/status" className={cn(buttonVariants({ variant: "link" }), "text-muted-foreground")}>
            ตรวจสอบสถานะใบสมัครของฉัน
          </Link>
        </div>
      </div>
    </PortalShell>
  );
}
