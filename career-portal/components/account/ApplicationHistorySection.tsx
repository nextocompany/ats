"use client";

// The logged-in member's application history: one row per position applied to,
// each linking to the full status view (/status?token=...). Self-contained — it
// owns its AccountSection wrapper so the account page just drops it in.

import Link from "next/link";
import { ChevronRight, ClipboardList } from "lucide-react";

import { AccountSection } from "@/components/account/AccountSection";
import { Skeleton } from "@/components/ui/skeleton";
import { metaFor, formatThaiDate, TONE_CLASS } from "@/lib/applicationStatus";
import { useMyApplications } from "@/lib/queries";

export function ApplicationHistorySection() {
  const { data, isLoading, isError } = useMyApplications();
  const items = data ?? [];

  return (
    <AccountSection
      eyebrow="ใบสมัครของฉัน"
      title="ประวัติการสมัครงาน"
      lead="ตำแหน่งทั้งหมดที่คุณสมัคร แตะแต่ละรายการเพื่อดูสถานะล่าสุด"
      action={
        items.length > 0 ? (
          <span className="num text-sm font-semibold tabular-nums text-muted-foreground">
            {items.length} รายการ
          </span>
        ) : undefined
      }
      padded={false}
    >
      {isLoading ? (
        <div className="space-y-2 p-5 sm:p-6">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-xl" />
          ))}
        </div>
      ) : isError ? (
        <p className="px-5 py-8 text-center text-sm text-muted-foreground sm:px-6">
          ไม่สามารถโหลดประวัติการสมัครได้ กรุณาลองใหม่อีกครั้ง
        </p>
      ) : items.length === 0 ? (
        <div className="flex flex-col items-center gap-3 px-5 py-10 text-center sm:px-6">
          <span aria-hidden className="grid size-12 place-items-center rounded-2xl bg-secondary text-muted-foreground">
            <ClipboardList className="size-6" strokeWidth={1.75} />
          </span>
          <p className="text-sm text-muted-foreground">คุณยังไม่ได้สมัครงานกับเรา</p>
          <Link
            href="/jobs"
            className="text-sm font-semibold text-primary underline-offset-4 hover:underline"
          >
            ดูตำแหน่งงานที่เปิดรับ
          </Link>
        </div>
      ) : (
        <ul className="divide-y divide-line">
          {items.map((a, i) => {
            const meta = metaFor(a.status);
            const Row = (
              <>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-foreground">
                    {a.position_title || "ตำแหน่งงาน"}
                  </span>
                  <span className="mt-0.5 block text-xs text-muted-foreground">
                    สมัครเมื่อ {formatThaiDate(a.applied_at)}
                  </span>
                </span>
                <span className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-medium ${TONE_CLASS[meta.tone]}`}>
                  {meta.label}
                </span>
              </>
            );
            // Rows with a status token deep-link to the detailed status view; a
            // legacy row without one still shows its status (non-clickable).
            return a.status_token ? (
              <li key={i}>
                <Link
                  href={`/status?token=${encodeURIComponent(a.status_token)}`}
                  className="flex items-center gap-3 px-5 py-4 transition-colors hover:bg-secondary/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring sm:px-6"
                >
                  {Row}
                  <ChevronRight className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                </Link>
              </li>
            ) : (
              <li key={i} className="flex items-center gap-3 px-5 py-4 sm:px-6">
                {Row}
              </li>
            );
          })}
        </ul>
      )}
    </AccountSection>
  );
}
