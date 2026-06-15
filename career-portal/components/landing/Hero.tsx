import Link from "next/link";

import { Container } from "@/components/ds";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// Hero is the landing's opening statement: a type-led institutional masthead — a
// quiet oversized Anuphan headline, one supporting paragraph, ONE blue primary CTA
// (+ a plain secondary link). Confidence through restraint: no image, no gradient,
// no second colour. Big but quiet.
export function Hero() {
  return (
    <section aria-labelledby="hero-heading" className="border-b border-line">
      <Container className="py-20 sm:py-24 lg:py-32">
        <div className="reveal flex max-w-3xl flex-col gap-7">
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
          <p className="max-w-2xl [font-size:var(--text-lead)] leading-relaxed text-muted-foreground">
            ร่วมเป็นส่วนหนึ่งของทีมที่ขับเคลื่อนธุรกิจค้าส่ง-ค้าปลีกทั่วประเทศ
            ภายใต้แบรนด์ Makro และ Lotus&rsquo;s เราลงทุนกับการพัฒนาคน
            เปิดเส้นทางก้าวหน้า และดูแลพนักงานอย่างเป็นธรรม
          </p>
          <div className="flex flex-col gap-3 pt-1 sm:flex-row sm:items-center">
            <Link href="/jobs" className={cn(buttonVariants({ size: "tap" }), "sm:px-8")}>
              ดูตำแหน่งงานที่เปิดรับ
            </Link>
            <Link
              href="/status"
              className="inline-flex h-12 items-center justify-center px-2 text-base font-medium text-foreground underline-offset-4 transition-colors hover:text-primary hover:underline focus-visible:rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            >
              ตรวจสอบสถานะใบสมัคร
            </Link>
          </div>
        </div>
      </Container>
    </section>
  );
}
