import Link from "next/link";

import { Container } from "@/components/Container";

// SiteFooter is the luxe multi-column footer: brand blurb, quick links, contact,
// and the PDPA assurance line. Collapses to a stack on mobile.
export function SiteFooter() {
  return (
    <footer className="mt-[var(--space-section)] border-t border-border/70 bg-secondary/40">
      <Container className="grid gap-10 py-14 sm:grid-cols-2 lg:grid-cols-4">
        <div className="space-y-3 lg:col-span-2">
          <Link href="/" className="flex items-center gap-2.5 font-semibold tracking-tight">
            <span className="grid size-8 place-content-center rounded-lg bg-accent text-sm font-bold text-accent-foreground">
              N
            </span>
            <span>ร่วมงานกับเรา</span>
          </Link>
          <p className="max-w-sm text-sm leading-relaxed text-muted-foreground">
            ร่วมเป็นส่วนหนึ่งของทีมค้าปลีกที่เติบโตทั่วประเทศ เราเชื่อในการพัฒนาคน
            และมอบโอกาสก้าวหน้าให้ทุกคนอย่างเท่าเทียม
          </p>
        </div>

        <nav aria-label="ลิงก์" className="space-y-3 text-sm">
          <p className="font-semibold text-foreground">เมนู</p>
          <ul className="space-y-2 text-muted-foreground">
            <li>
              <Link href="/jobs" className="transition-colors hover:text-foreground">
                ตำแหน่งงานที่เปิดรับ
              </Link>
            </li>
            <li>
              <Link href="/status" className="transition-colors hover:text-foreground">
                ตรวจสอบสถานะใบสมัคร
              </Link>
            </li>
          </ul>
        </nav>

        <div className="space-y-3 text-sm">
          <p className="font-semibold text-foreground">ติดต่อฝ่ายบุคคล</p>
          <ul className="space-y-2 text-muted-foreground">
            <li>recruit@example.co.th</li>
            <li>โทร. 0-2000-0000</li>
            <li>จันทร์–ศุกร์ 09:00–18:00</li>
          </ul>
        </div>
      </Container>

      <div className="border-t border-border/60">
        <Container className="flex flex-col items-center justify-between gap-2 py-6 text-center text-xs text-muted-foreground sm:flex-row sm:text-left">
          <p>ข้อมูลของคุณได้รับการคุ้มครองตาม พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล (PDPA)</p>
          <p>© {"2569"} บริษัทตัวอย่าง จำกัด</p>
        </Container>
      </div>
    </footer>
  );
}
