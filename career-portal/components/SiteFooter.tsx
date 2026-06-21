import Link from "next/link";

import { Container } from "@/components/ds";
import { Wordmark } from "@/components/Wordmark";

interface FooterLink {
  href: string;
  label: string;
  // external links open in a new tab via a plain <a> with rel="noreferrer".
  external?: boolean;
}

const NAV_GROUPS: { title: string; links: FooterLink[] }[] = [
  {
    title: "ร่วมงาน",
    links: [
      { href: "/jobs", label: "ตำแหน่งงานที่เปิดรับ" },
      { href: "/status", label: "ตรวจสอบสถานะใบสมัคร" },
      { href: "/account", label: "บัญชีของฉัน" },
    ],
  },
  {
    title: "องค์กร",
    links: [
      // Each org link has a distinct, real destination (not a placeholder "/"):
      // the corporate site, and in-page anchors to the ESG + culture sections.
      { href: "https://www.cpaxtra.com", label: "เกี่ยวกับ CP Axtra", external: true },
      { href: "/#esg", label: "ความยั่งยืน (ESG)" },
      { href: "/#culture", label: "วัฒนธรรมองค์กร" },
    ],
  },
];

// SiteFooter is the generous institutional footer: wordmark + positioning line,
// quiet credentials (SET listing, HR Asia), navigation columns, and the PDPA
// assurance row. Hairline-divided, near-monochrome, no decoration.
export function SiteFooter() {
  return (
    <footer className="mt-[var(--space-section)] border-t border-line bg-surface-muted">
      <Container className="grid gap-12 py-16 lg:grid-cols-[1.4fr_1fr_1fr]">
        <div className="flex flex-col gap-5">
          <Wordmark />
          <p className="max-w-sm text-sm leading-relaxed text-muted-foreground">
            CP Axtra เป็นผู้นำธุรกิจค้าส่ง-ค้าปลีกของไทย ภายใต้แบรนด์ Makro และ Lotus&rsquo;s
            เราเชื่อว่าการเติบโตขององค์กรเริ่มต้นจากการเติบโตของพนักงานทุกคน
          </p>
          <p className="text-xs leading-relaxed text-muted-foreground/80">
            บริษัทจดทะเบียนในตลาดหลักทรัพย์แห่งประเทศไทย (SET)
            <span aria-hidden="true" className="mx-1.5">&middot;</span>
            HR Asia Best Companies to Work for
          </p>
        </div>

        {NAV_GROUPS.map((group) => (
          <nav key={group.title} aria-label={group.title} className="flex flex-col gap-4 text-sm">
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-foreground">{group.title}</p>
            <ul className="flex flex-col gap-3 text-muted-foreground">
              {group.links.map((link) => {
                const className =
                  "transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground";
                return (
                  <li key={link.label}>
                    {link.external ? (
                      <a href={link.href} target="_blank" rel="noreferrer" className={className}>
                        {link.label}
                      </a>
                    ) : (
                      <Link href={link.href} className={className}>
                        {link.label}
                      </Link>
                    )}
                  </li>
                );
              })}
            </ul>
          </nav>
        ))}
      </Container>

      <div className="border-t border-line">
        <Container className="flex flex-col items-start justify-between gap-2 py-6 text-xs text-muted-foreground sm:flex-row sm:items-center">
          <Link
            href="/privacy"
            className="transition-colors hover:text-foreground focus-visible:text-foreground focus-visible:outline-none"
          >
            ข้อมูลของคุณได้รับการคุ้มครองตาม พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล (PDPA)
          </Link>
          <p>&copy; 2569 บริษัท ซีพี แอ็กซ์ตร้า จำกัด (มหาชน)</p>
        </Container>
      </div>
    </footer>
  );
}
