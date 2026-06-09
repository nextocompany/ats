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
      <div className="space-y-10">
        <header className="max-w-2xl space-y-3">
          <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">ร่วมงานกับเรา</p>
          <h1 className="text-[length:var(--text-display)] font-bold leading-tight tracking-tight">
            ตำแหน่งงานที่เปิดรับ
          </h1>
          <p className="text-base text-muted-foreground">
            เลือกตำแหน่งที่สนใจเพื่อดูรายละเอียดและสมัครงาน — สมัครง่ายในไม่กี่ขั้นตอน
          </p>
        </header>

        <InstallPrompt />

        {isLoading ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" aria-hidden="true">
            {[0, 1, 2, 3, 4, 5].map((i) => (
              <Skeleton key={i} className="h-[164px] w-full rounded-2xl" />
            ))}
          </div>
        ) : null}

        {isError ? (
          <div className="mx-auto max-w-md space-y-4 rounded-2xl border border-border bg-card p-8 text-center">
            <p className="text-sm text-muted-foreground">ไม่สามารถโหลดตำแหน่งงานได้ในขณะนี้</p>
            <Button size="tap" variant="outline" onClick={() => refetch()}>
              ลองอีกครั้ง
            </Button>
          </div>
        ) : null}

        {positions && positions.length === 0 ? (
          <div className="mx-auto max-w-md space-y-2 rounded-2xl border border-border bg-card p-10 text-center">
            <p className="text-base font-medium">ยังไม่มีตำแหน่งงานที่เปิดรับ</p>
            <p className="text-sm text-muted-foreground">โปรดกลับมาตรวจสอบอีกครั้งในภายหลัง</p>
          </div>
        ) : null}

        {positions && positions.length > 0 ? (
          <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
