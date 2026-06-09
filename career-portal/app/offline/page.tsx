import type { Metadata } from "next";
import Link from "next/link";

import { PortalShell } from "@/components/PortalShell";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// Static fallback shown by the service worker when an uncached navigation is
// attempted offline. Must stay fully static (no client fetch) so it is precached
// in the build manifest and renders with zero network.
export const metadata: Metadata = {
  title: "ออฟไลน์ | ร่วมงานกับเรา",
};

export default function OfflinePage() {
  return (
    <PortalShell narrow>
      <div className="flex flex-col items-center gap-5 rounded-2xl border border-border bg-card px-6 py-12 text-center">
        <span className="grid size-16 place-content-center rounded-full bg-brand-soft text-accent" aria-hidden="true">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none">
            <path d="M1 1l22 22" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            <path
              d="M5 12.5a10 10 0 0114 0M8.5 16a5 5 0 017 0M12 19.5h.01"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </span>
        <div className="space-y-2">
          <h1 className="text-xl font-bold tracking-tight">คุณกำลังออฟไลน์</h1>
          <p className="text-sm text-muted-foreground">
            ดูเหมือนว่าการเชื่อมต่ออินเทอร์เน็ตขัดข้อง โปรดตรวจสอบสัญญาณแล้วลองใหม่อีกครั้ง
          </p>
        </div>
        <Link href="/jobs" className={cn(buttonVariants({ size: "tap" }), "w-full sm:w-auto")}>
          กลับไปหน้าตำแหน่งงาน
        </Link>
      </div>
    </PortalShell>
  );
}
