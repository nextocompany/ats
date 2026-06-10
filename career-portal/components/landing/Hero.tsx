import Link from "next/link";

import { buttonVariants } from "@/components/ui/button";
import { Container } from "@/components/Container";
import { cn } from "@/lib/utils";

// Hero is the landing's opening statement: a large display headline, supporting
// copy, dual CTA, and a decorative atmosphere panel (pure CSS/SVG — no imagery
// dependency). CP Axtra: dramatic scale, generous whitespace, blue + yellow + dots.
export function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-border/60">
      {/* soft blue atmosphere wash */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(120%_90%_at_85%_-10%,var(--brand-soft),transparent_55%)]"
      />
      {/* CP Axtra dot signature — scattered in the corner of the hero */}
      <div
        aria-hidden="true"
        className="dot-cluster pointer-events-none absolute right-5 top-6 hidden opacity-90 sm:block lg:right-10 lg:top-10"
      />
      <Container className="grid items-center gap-12 py-16 sm:py-24 lg:grid-cols-12 lg:py-32">
        <div className="reveal space-y-7 lg:col-span-7">
          <p className="inline-flex items-center gap-2 rounded-full border border-border bg-card px-4 py-1.5 text-xs font-medium tracking-wide text-muted-foreground">
            <span className="size-1.5 rounded-full bg-accent" aria-hidden="true" />
            เปิดรับสมัครหลายตำแหน่งทั่วประเทศ
          </p>
          <h1 className="text-[length:var(--text-hero)] font-bold leading-[1.05] tracking-tight">
            ร่วมเติบโต
            <br className="hidden sm:block" />
            ไปกับเรา
          </h1>
          <p className="max-w-xl text-lg leading-relaxed text-muted-foreground">
            เราเชื่อว่าคนคือหัวใจขององค์กร มาเป็นส่วนหนึ่งของทีมค้าปลีกที่ใส่ใจการพัฒนา
            มอบโอกาสก้าวหน้า และดูแลคุณอย่างเท่าเทียม
          </p>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Link href="/jobs" className={cn(buttonVariants({ size: "tap" }), "sm:px-8")}>
              ดูตำแหน่งงานทั้งหมด
            </Link>
            <Link
              href="/status"
              className={cn(buttonVariants({ variant: "outline", size: "tap" }), "sm:px-8")}
            >
              ตรวจสอบสถานะใบสมัคร
            </Link>
          </div>
        </div>

        {/* Atmosphere panel — layered cards, dot rule, geometric depth. */}
        <div className="reveal relative hidden lg:col-span-5 lg:block" aria-hidden="true">
          <div className="relative mx-auto aspect-[4/5] w-full max-w-sm">
            <div className="absolute inset-0 translate-x-6 translate-y-6 rounded-3xl border border-border bg-secondary/50" />
            <div className="absolute inset-0 rounded-3xl border border-border bg-card shadow-[0_30px_80px_-30px_oklch(46%_0.18_264/0.3)]">
              <div className="flex h-full flex-col justify-between p-8">
                <div className="dot-rule" />
                <div className="space-y-4">
                  <div className="size-12 rounded-2xl bg-accent/10" />
                  <div className="h-3 w-3/4 rounded-full bg-muted" />
                  <div className="h-3 w-1/2 rounded-full bg-muted" />
                </div>
                <div className="space-y-3">
                  <div className="h-2.5 w-full rounded-full bg-muted" />
                  <div className="h-2.5 w-5/6 rounded-full bg-muted" />
                  <div className="h-2.5 w-2/3 rounded-full bg-muted" />
                </div>
                <div className="flex items-center gap-2">
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-brand-soft px-3 py-1 text-xs font-semibold text-accent">
                    <span className="size-1.5 rounded-full bg-accent" />
                    เปิดรับสมัคร
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </section>
  );
}
