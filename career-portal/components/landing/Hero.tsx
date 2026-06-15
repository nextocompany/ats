import Link from "next/link";

import { Container, ImageSlot } from "@/components/ds";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// Hero is the landing's opening statement: a quiet oversized Anuphan headline, a
// single supporting paragraph, ONE blue primary CTA (+ a plain secondary link),
// and a real-people photo slot. Confidence through restraint — no gradient wash,
// no decorative panel, no second color.
export function Hero() {
  return (
    <section aria-labelledby="hero-heading" className="border-b border-line">
      <Container className="grid items-center gap-12 py-16 sm:py-20 lg:grid-cols-12 lg:gap-16 lg:py-28">
        <div className="reveal flex flex-col gap-7 lg:col-span-6">
          <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
            Makro &nbsp;&middot;&nbsp; Lotus&rsquo;s &nbsp;&middot;&nbsp; CP Axtra
          </p>
          <h1
            id="hero-heading"
            className="[font-size:var(--text-display)] font-bold leading-[1.08] text-foreground"
          >
            เติบโตไปกับองค์กร
            <br className="hidden sm:block" />
            ค้าปลีกชั้นนำของไทย
          </h1>
          <p className="max-w-xl [font-size:var(--text-lead)] leading-relaxed text-muted-foreground">
            ร่วมเป็นส่วนหนึ่งของทีมที่ขับเคลื่อนธุรกิจค้าส่ง-ค้าปลีกทั่วประเทศ
            เราลงทุนกับการพัฒนาคน เปิดเส้นทางก้าวหน้า และดูแลพนักงานอย่างเป็นธรรม
          </p>
          <div className="flex flex-col gap-3 pt-1 sm:flex-row sm:items-center">
            <Link href="/jobs" className={cn(buttonVariants({ size: "tap" }), "sm:px-8")}>
              ดูตำแหน่งงานที่เปิดรับ
            </Link>
            <Link
              href="/status"
              className="inline-flex h-12 items-center justify-center px-2 text-base font-medium text-foreground underline-offset-4 transition-colors hover:text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:rounded-lg"
            >
              ตรวจสอบสถานะใบสมัคร
            </Link>
          </div>
        </div>

        <div className="reveal lg:col-span-6">
          <ImageSlot aspect="aspect-[4/3]" caption="ภาพพนักงาน CP Axtra — Makro / Lotus's" />
        </div>
      </Container>
    </section>
  );
}
