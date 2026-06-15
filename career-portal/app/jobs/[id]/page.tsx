"use client";

import Link from "next/link";
import { use } from "react";

import { PortalShell } from "@/components/PortalShell";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { levelLabel } from "@/lib/levels";
import { usePublicPosition } from "@/lib/queries";
import { cn } from "@/lib/utils";

// The public API exposes only title/level, so role content is intentionally
// generic, reassuring copy (no fabricated job-specific requirements).
const ABOUT =
  "ร่วมเป็นส่วนหนึ่งของทีมที่ใส่ใจการบริการและการเติบโตไปด้วยกัน เรามองหาผู้ที่มีความตั้งใจ ใจรักงานบริการ และพร้อมเรียนรู้สิ่งใหม่ เพื่อส่งมอบประสบการณ์ที่ดีให้กับลูกค้าในทุกสาขา";
const OFFER = [
  "ค่าตอบแทนและโบนัสตามผลงาน",
  "ประกันสุขภาพและสวัสดิการพนักงาน",
  "เส้นทางก้าวหน้าในสายอาชีพที่ชัดเจน",
  "การอบรมและพัฒนาทักษะอย่างต่อเนื่อง",
];
const STEPS = [
  "เข้าสู่ระบบหรือสมัครสมาชิก แล้วให้ความยินยอม PDPA",
  "ตรวจสอบข้อมูลและแนบเรซูเม่ของคุณ",
  "ส่งใบสมัคร แล้วติดตามสถานะได้ทุกเมื่อ",
];

export default function JobDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: position, isLoading, isError } = usePublicPosition(id);

  return (
    <PortalShell backHref="/jobs">
      {isLoading ? (
        <div className="flex flex-col gap-5">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-12 w-2/3" />
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="mt-4 h-48 w-full" />
        </div>
      ) : null}

      {isError || (!isLoading && !position) ? (
        <div className="mx-auto flex max-w-md flex-col items-center gap-4 rounded-xl border border-line bg-card p-10 text-center">
          <p className="text-sm text-muted-foreground">ไม่พบตำแหน่งงานนี้</p>
          <Link href="/jobs" className={buttonVariants({ variant: "outline", size: "tap" })}>
            กลับไปดูตำแหน่งงานทั้งหมด
          </Link>
        </div>
      ) : null}

      {position ? (
        <article className="grid gap-12 lg:grid-cols-[1fr_320px] lg:gap-16">
          <div className="flex max-w-2xl flex-col gap-12">
            <header className="flex flex-col gap-4 border-b border-line pb-8">
              {position.level ? (
                <p className="text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">
                  {levelLabel(position.level)}
                </p>
              ) : null}
              <h1 className="[font-size:var(--text-display)] font-bold leading-[1.1] text-foreground">
                {position.title_th}
              </h1>
              {position.title_en ? (
                <p className="[font-size:var(--text-lead)] text-muted-foreground">{position.title_en}</p>
              ) : null}
            </header>

            <section className="flex flex-col gap-3">
              <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">เกี่ยวกับตำแหน่งนี้</h2>
              <p className="leading-relaxed text-foreground/80">{ABOUT}</p>
            </section>

            <section className="flex flex-col gap-4">
              <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">สิ่งที่เรามอบให้</h2>
              <ul className="divide-y divide-line border-y border-line">
                {OFFER.map((o) => (
                  <li key={o} className="flex items-start gap-3 py-4 text-foreground/85">
                    <span
                      aria-hidden="true"
                      className="mt-0.5 grid size-5 shrink-0 place-content-center rounded-full bg-primary/10 text-primary"
                    >
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                        <path d="M5 13l4 4L19 7" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </span>
                    {o}
                  </li>
                ))}
              </ul>
            </section>

            <section className="flex flex-col gap-4">
              <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">ขั้นตอนการสมัคร</h2>
              <ol className="flex flex-col gap-4">
                {STEPS.map((s, i) => (
                  <li key={s} className="flex items-start gap-3.5 text-foreground/85">
                    <span className="num grid size-7 shrink-0 place-content-center rounded-full border border-line bg-secondary text-sm font-semibold text-foreground">
                      {i + 1}
                    </span>
                    <span className="pt-0.5">{s}</span>
                  </li>
                ))}
              </ol>
            </section>
          </div>

          {/* Apply card — sticky on desktop, inline on mobile. */}
          <aside className="lg:col-start-2">
            <div className="flex flex-col gap-4 rounded-xl border border-line bg-card p-6 lg:sticky lg:top-24">
              <div className="flex flex-col gap-1">
                <p className="text-sm text-muted-foreground">สนใจตำแหน่งนี้?</p>
                <p className="text-lg font-semibold text-foreground">สมัครได้เลยวันนี้</p>
              </div>
              <Link href={`/jobs/${position.id}/apply`} className={cn(buttonVariants({ size: "tap" }), "w-full")}>
                สมัครงาน
              </Link>
              <p className="text-center text-xs text-muted-foreground">ใช้เวลาไม่กี่นาที &middot; ทีม HR จะติดต่อกลับ</p>
            </div>
          </aside>
        </article>
      ) : null}
    </PortalShell>
  );
}
