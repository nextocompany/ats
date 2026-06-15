import Link from "next/link";

import { Container, Eyebrow, SectionHeading, StatBand, type Stat } from "@/components/ds";
import { FeaturedJobs } from "@/components/landing/FeaturedJobs";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// Institutional proof tokens — verifiable facts only (SET listing, the two retail
// brands, nationwide reach, the HR Asia award). No fabricated revenue/headcount/
// store counts on a listed company's public site.
const STATS: Stat[] = [
  { value: "SET", label: "บริษัทจดทะเบียน", note: "ตลาดหลักทรัพย์แห่งประเทศไทย" },
  { value: "2", label: "แบรนด์ค้าปลีกชั้นนำ", note: "Makro และ Lotus's" },
  { value: "ทั่วไทย", label: "เครือข่ายสาขา", note: "ครอบคลุมทุกภูมิภาค" },
  { value: "HR Asia", label: "Best Companies to Work for", note: "องค์กรน่าทำงานแห่งเอเชีย" },
];

// "Why join" content — generic-but-true (no fabricated specifics), Thai-first.
// id anchors let the footer deep-link to culture + ESG.
const BLOCKS: { id?: string; eyebrow: string; heading: string; body: string; points: string[] }[] = [
  {
    eyebrow: "เส้นทางอาชีพ",
    heading: "เติบโตในสายอาชีพอย่างมีทิศทาง",
    body: "เราออกแบบเส้นทางความก้าวหน้าที่ชัดเจนในทุกสายงาน พร้อมการอบรม โค้ชชิ่ง และการหมุนเวียนงานเพื่อให้คุณพัฒนาได้เต็มศักยภาพ",
    points: ["แผนพัฒนารายบุคคล (IDP)", "โอกาสเลื่อนตำแหน่งจากภายใน", "หลักสูตรอบรมและทุนพัฒนาทักษะ"],
  },
  {
    eyebrow: "สวัสดิการและการดูแล",
    heading: "ดูแลคุณภาพชีวิตของพนักงานทุกคน",
    body: "ตั้งแต่ประกันสุขภาพ โบนัสตามผลงาน ไปจนถึงสิทธิประโยชน์ที่ครอบคลุมครอบครัว เราเชื่อว่าพนักงานที่ได้รับการดูแลคือรากฐานของบริการที่ดี",
    points: ["ประกันสุขภาพและประกันชีวิต", "โบนัสและกองทุนสำรองเลี้ยงชีพ", "สิทธิพนักงานในการซื้อสินค้า"],
  },
  {
    id: "culture",
    eyebrow: "วัฒนธรรมองค์กร",
    heading: "วัฒนธรรมที่ให้เกียรติและร่วมมือ",
    body: "เราสร้างที่ทำงานที่เปิดรับความหลากหลาย เคารพซึ่งกันและกัน และส่งเสริมการทำงานเป็นทีม เพื่อให้ทุกคนรู้สึกเป็นเจ้าขององค์กรร่วมกัน",
    points: ["ความหลากหลายและการมีส่วนร่วม", "ผู้นำที่เข้าถึงได้", "การสื่อสารที่โปร่งใส"],
  },
  {
    id: "esg",
    eyebrow: "ความยั่งยืน",
    heading: "ขับเคลื่อนธุรกิจอย่างรับผิดชอบ",
    body: "ในฐานะองค์กรค้าปลีกขนาดใหญ่ เรามุ่งมั่นลดผลกระทบต่อสิ่งแวดล้อม สนับสนุนชุมชน และดำเนินธุรกิจตามหลักบรรษัทภิบาล (ESG) อย่างจริงจัง",
    points: ["เป้าหมายลดการปล่อยคาร์บอน", "สนับสนุนผู้ประกอบการรายย่อย", "บริหารงานตามหลัก ESG"],
  },
];

// LandingSections holds the static institutional sections (server-rendered) and
// the live FeaturedJobs client island.
export function LandingSections() {
  return (
    <>
      {/* Institutional proof band — verifiable credentials, not fabricated metrics. */}
      <section aria-label="ภาพรวมองค์กร" className="border-b border-line">
        <Container className="py-[var(--space-section)]">
          <div className="flex flex-col gap-10">
            <SectionHeading
              eyebrow="องค์กรของเรา"
              heading="ความมั่นคงที่มาพร้อมโอกาส"
              lead="ด้วยเครือข่ายค้าปลีกที่ครอบคลุมทั่วประเทศภายใต้ Makro และ Lotus's ทุกตำแหน่งคือโอกาสในการสร้างผลกระทบจริง"
            />
            <StatBand stats={STATS} />
          </div>
        </Container>
      </section>

      {/* Why join — a clean two-column text grid (no imagery). Hierarchy via scale
          and whitespace; the only colour is navy ink + the hairline + one blue dot. */}
      <section aria-label="ทำไมต้องร่วมงานกับเรา" className="bg-background">
        <Container className="py-[var(--space-section)]">
          <SectionHeading
            eyebrow="ทำไมต้องร่วมงานกับเรา"
            heading="ที่ทำงานที่ให้คุณเป็นมากกว่าพนักงาน"
            className="mb-12"
          />
          <div className="grid gap-px overflow-hidden rounded-xl border border-line bg-line sm:grid-cols-2">
            {BLOCKS.map((block) => (
              <article
                id={block.id}
                key={block.heading}
                className="flex scroll-mt-24 flex-col gap-4 bg-background p-8 lg:p-10"
              >
                <Eyebrow>{block.eyebrow}</Eyebrow>
                <h3 className="[font-size:var(--text-h3)] font-semibold leading-snug text-foreground">
                  {block.heading}
                </h3>
                <p className="[font-size:var(--text-lead)] leading-relaxed text-muted-foreground">
                  {block.body}
                </p>
                <ul className="mt-1 divide-y divide-line border-t border-line">
                  {block.points.map((p, i) => (
                    <li key={`${i}-${p}`} className="flex items-start gap-3 py-3 text-sm text-foreground/85">
                      <span aria-hidden="true" className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                      {p}
                    </li>
                  ))}
                </ul>
              </article>
            ))}
          </div>
        </Container>
      </section>

      {/* Featured roles (live). */}
      <section aria-label="ตำแหน่งงานแนะนำ" className="border-t border-line bg-surface-muted">
        <Container className="py-[var(--space-section)]">
          <FeaturedJobs />
        </Container>
      </section>

      {/* Closing CTA band — solid navy ink, a single CTA, no decoration. */}
      <section aria-label="เริ่มต้นสมัครงาน">
        <Container className="py-[var(--space-section)]">
          <div className="flex flex-col items-start gap-8 rounded-2xl bg-foreground px-8 py-14 sm:px-14 lg:flex-row lg:items-center lg:justify-between lg:py-16">
            <div className="flex max-w-xl flex-col gap-3">
              <h2 className="[font-size:var(--text-h2)] font-semibold leading-tight text-background">
                พร้อมก้าวต่อไปในเส้นทางอาชีพของคุณ
              </h2>
              <p className="[font-size:var(--text-lead)] leading-relaxed text-background/70">
                ค้นหาตำแหน่งที่ใช่ แล้วสมัครได้ในไม่กี่ขั้นตอน ทีมงานของเราพร้อมต้อนรับคุณ
              </p>
            </div>
            <Link
              href="/jobs"
              className={cn(
                buttonVariants({ size: "tap" }),
                "shrink-0 bg-background px-8 text-foreground hover:bg-background/90",
              )}
            >
              ดูตำแหน่งงานทั้งหมด
            </Link>
          </div>
        </Container>
      </section>
    </>
  );
}
