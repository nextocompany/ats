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

// The public API exposes only title/level, so role content is intentionally
// generic, reassuring copy (no fabricated job-specific requirements).
const ABOUT = [
  "ร่วมเป็นส่วนหนึ่งของทีมที่ใส่ใจการบริการและการเติบโตไปด้วยกัน เรามองหาผู้ที่มีความตั้งใจ ใจรักงานบริการ และพร้อมเรียนรู้สิ่งใหม่",
];
const OFFER = [
  "ค่าตอบแทนและโบนัสตามผลงาน",
  "ประกันสุขภาพและสวัสดิการพนักงาน",
  "เส้นทางก้าวหน้าในสายอาชีพที่ชัดเจน",
  "การอบรมและพัฒนาทักษะอย่างต่อเนื่อง",
];
const STEPS = ["กดปุ่ม “สมัครงาน” แล้วให้ความยินยอม PDPA", "กรอกข้อมูลและแนบเรซูเม่", "ยืนยันตัวตนด้วย LINE แล้วส่งใบสมัคร"];

export default function JobDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: position, isLoading, isError } = usePublicPosition(id);

  return (
    <PortalShell backHref="/jobs">
      {isLoading ? (
        <div className="space-y-4">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="h-48 w-full rounded-2xl" />
        </div>
      ) : null}

      {isError || (!isLoading && !position) ? (
        <div className="mx-auto max-w-md space-y-4 rounded-2xl border border-border bg-card p-8 text-center">
          <p className="text-sm text-muted-foreground">ไม่พบตำแหน่งงานนี้</p>
          <Link href="/jobs" className={buttonVariants({ variant: "outline", size: "tap" })}>
            กลับไปดูตำแหน่งงานทั้งหมด
          </Link>
        </div>
      ) : null}

      {position ? (
        <div className="grid gap-10 lg:grid-cols-3 lg:gap-12">
          {/* Content */}
          <article className="space-y-10 lg:col-span-2">
            <header className="space-y-4">
              {position.level ? (
                <span className="inline-flex items-center gap-2 rounded-full bg-brand-soft px-3 py-1 text-xs font-semibold text-accent">
                  {LEVEL_LABELS[position.level.toLowerCase()] ?? position.level}
                </span>
              ) : null}
              <h1 className="text-[length:var(--text-display)] font-bold leading-tight tracking-tight">
                {position.title_th}
              </h1>
              {position.title_en ? <p className="text-base text-muted-foreground">{position.title_en}</p> : null}
            </header>

            <section className="space-y-3">
              <h2 className="text-lg font-semibold">เกี่ยวกับตำแหน่งนี้</h2>
              {ABOUT.map((p) => (
                <p key={p} className="leading-relaxed text-foreground/80">
                  {p}
                </p>
              ))}
            </section>

            <section className="space-y-4">
              <h2 className="text-lg font-semibold">สิ่งที่เรามอบให้</h2>
              <ul className="grid gap-3 sm:grid-cols-2">
                {OFFER.map((o) => (
                  <li key={o} className="flex items-start gap-3 rounded-xl border border-border bg-card p-4 text-sm">
                    <span className="mt-0.5 grid size-5 shrink-0 place-content-center rounded-full bg-accent/10 text-accent" aria-hidden="true">
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                        <path d="M5 13l4 4L19 7" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </span>
                    <span className="text-foreground/80">{o}</span>
                  </li>
                ))}
              </ul>
            </section>

            <section className="space-y-4">
              <h2 className="text-lg font-semibold">ขั้นตอนการสมัคร</h2>
              <ol className="space-y-3">
                {STEPS.map((s, i) => (
                  <li key={s} className="flex items-start gap-3 text-sm text-foreground/80">
                    <span className="grid size-6 shrink-0 place-content-center rounded-full bg-primary text-xs font-semibold text-primary-foreground">
                      {i + 1}
                    </span>
                    {s}
                  </li>
                ))}
              </ol>
            </section>
          </article>

          {/* Apply card — sticky on desktop, inline on mobile */}
          <aside className="lg:col-span-1">
            <div className="space-y-4 rounded-2xl border border-border bg-card p-6 lg:sticky lg:top-24">
              <div className="space-y-1">
                <p className="text-sm text-muted-foreground">สนใจตำแหน่งนี้?</p>
                <p className="text-lg font-semibold">สมัครได้เลยวันนี้</p>
              </div>
              <Link href={`/jobs/${position.id}/apply`} className={cn(buttonVariants({ size: "tap" }), "w-full")}>
                สมัครงาน
              </Link>
              <p className="text-center text-xs text-muted-foreground">ใช้เวลาไม่กี่นาที · ทีม HR จะติดต่อกลับ</p>
            </div>
          </aside>
        </div>
      ) : null}
    </PortalShell>
  );
}
