import Link from "next/link";

import { buttonVariants } from "@/components/ui/button";
import { Container } from "@/components/Container";
import { FeaturedJobs } from "@/components/landing/FeaturedJobs";
import { cn } from "@/lib/utils";

const VALUES = [
  {
    title: "เติบโตในสายอาชีพ",
    body: "เส้นทางก้าวหน้าชัดเจน พร้อมการอบรมและโค้ชชิ่งเพื่อพัฒนาทักษะอย่างต่อเนื่อง",
  },
  {
    title: "สวัสดิการที่ดูแลคุณ",
    body: "ประกันสุขภาพ โบนัสตามผลงาน วันลาพักผ่อน และสิทธิประโยชน์พนักงานครบครัน",
  },
  {
    title: "ทีมที่อบอุ่น",
    body: "วัฒนธรรมการทำงานที่เคารพกัน ช่วยเหลือเกื้อกูล และเปิดรับความหลากหลาย",
  },
  {
    title: "สาขาทั่วประเทศ",
    body: "เลือกทำงานใกล้บ้านได้จากสาขากว่า 160 แห่งทั่วทุกภูมิภาคของไทย",
  },
];

const STEPS = [
  { n: "01", title: "เลือกตำแหน่ง", body: "เลือกงานที่ใช่จากตำแหน่งที่เปิดรับ" },
  { n: "02", title: "กรอกใบสมัคร", body: "กรอกข้อมูลและแนบเรซูเม่ในไม่กี่ขั้นตอน" },
  { n: "03", title: "รอการติดต่อ", body: "ทีม HR จะติดต่อกลับ ติดตามสถานะได้ตลอดเวลา" },
];

// LandingSections holds the static marketing sections (server-rendered) plus the
// live FeaturedJobs client island.
export function LandingSections() {
  return (
    <>
      {/* Value props */}
      <section className="py-[var(--space-section)]">
        <Container className="space-y-12">
          <div className="max-w-2xl space-y-3">
            <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">ทำไมต้องร่วมงานกับเรา</p>
            <h2 className="text-[length:var(--text-display)] font-bold tracking-tight">
              ที่ทำงานที่ให้คุณเป็นมากกว่าพนักงาน
            </h2>
          </div>
          <div className="grid gap-px overflow-hidden rounded-3xl border border-border bg-border sm:grid-cols-2 lg:grid-cols-4">
            {VALUES.map((v) => (
              <div key={v.title} className="flex flex-col gap-3 bg-card p-8">
                <div className="h-px w-10 bg-gold" aria-hidden="true" />
                <h3 className="text-lg font-semibold">{v.title}</h3>
                <p className="text-sm leading-relaxed text-muted-foreground">{v.body}</p>
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* Featured jobs (live) */}
      <section className="pb-[var(--space-section)]">
        <Container>
          <FeaturedJobs />
        </Container>
      </section>

      {/* How it works */}
      <section className="border-y border-border/60 bg-secondary/30 py-[var(--space-section)]">
        <Container className="space-y-12">
          <div className="max-w-2xl space-y-3">
            <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">ขั้นตอนการสมัคร</p>
            <h2 className="text-[length:var(--text-display)] font-bold tracking-tight">สมัครง่ายใน 3 ขั้นตอน</h2>
          </div>
          <div className="grid gap-8 sm:grid-cols-3">
            {STEPS.map((s) => (
              <div key={s.n} className="space-y-3">
                <div className="font-mono text-3xl font-bold text-accent/30">{s.n}</div>
                <h3 className="text-lg font-semibold">{s.title}</h3>
                <p className="text-sm leading-relaxed text-muted-foreground">{s.body}</p>
              </div>
            ))}
          </div>
        </Container>
      </section>

      {/* CTA band */}
      <section className="py-[var(--space-section)]">
        <Container>
          <div className="relative overflow-hidden rounded-3xl bg-primary px-8 py-14 text-center sm:px-16 sm:py-20">
            <div
              aria-hidden="true"
              className="pointer-events-none absolute inset-0 bg-[radial-gradient(100%_120%_at_50%_-20%,oklch(44%_0.09_158/0.35),transparent_60%)]"
            />
            <div className="relative space-y-6">
              <h2 className="text-[length:var(--text-display)] font-bold tracking-tight text-primary-foreground">
                พร้อมเริ่มต้นแล้วหรือยัง
              </h2>
              <p className="mx-auto max-w-xl text-base text-primary-foreground/70">
                ค้นหาตำแหน่งที่ใช่สำหรับคุณ แล้วสมัครได้เลยวันนี้
              </p>
              <Link
                href="/jobs"
                className={cn(
                  buttonVariants({ size: "tap" }),
                  "bg-card px-8 text-foreground hover:bg-card/90",
                )}
              >
                ดูตำแหน่งงานทั้งหมด
              </Link>
            </div>
          </div>
        </Container>
      </section>
    </>
  );
}
