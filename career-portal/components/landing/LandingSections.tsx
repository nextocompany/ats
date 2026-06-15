import Link from "next/link";

import { Container, MediaBlock, SectionHeading, StatBand, type Stat } from "@/components/ds";
import { FeaturedJobs } from "@/components/landing/FeaturedJobs";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// Illustrative scale figures — flagged as such in the note line, not audited
// disclosures. They establish institutional proof-of-scale without overclaiming.
const STATS: Stat[] = [
  { value: "150,000", label: "ล้านบาท / ปี", note: "รายได้รวมโดยประมาณ" },
  { value: "2,600+", label: "สาขาทั่วประเทศ", note: "Makro และ Lotus's" },
  { value: "50,000+", label: "พนักงาน", note: "ทั่วทุกภูมิภาค" },
  { value: "SET", label: "บริษัทจดทะเบียน", note: "ตลาดหลักทรัพย์ฯ" },
];

// MediaBlock sections alternate image-slot side for an editorial rhythm. Copy is
// generic-but-true (no fabricated specifics), Thai-first.
const BLOCKS = [
  {
    eyebrow: "เส้นทางอาชีพ",
    heading: "เติบโตในสายอาชีพอย่างมีทิศทาง",
    body: "เราออกแบบเส้นทางความก้าวหน้าที่ชัดเจนในทุกสายงาน พร้อมการอบรม โค้ชชิ่ง และการหมุนเวียนงานเพื่อให้คุณพัฒนาได้เต็มศักยภาพ",
    points: ["แผนพัฒนารายบุคคล (IDP)", "โอกาสเลื่อนตำแหน่งจากภายใน", "หลักสูตรอบรมและทุนพัฒนาทักษะ"],
    imageCaption: "ภาพการอบรมพนักงาน",
    imageSide: "right" as const,
  },
  {
    eyebrow: "สวัสดิการและการดูแล",
    heading: "ดูแลคุณภาพชีวิตของพนักงานทุกคน",
    body: "ตั้งแต่ประกันสุขภาพ โบนัสตามผลงาน ไปจนถึงสิทธิประโยชน์ที่ครอบคลุมครอบครัว เราเชื่อว่าพนักงานที่ได้รับการดูแลคือรากฐานของบริการที่ดี",
    points: ["ประกันสุขภาพและประกันชีวิต", "โบนัสและกองทุนสำรองเลี้ยงชีพ", "สิทธิพนักงานในการซื้อสินค้า"],
    imageCaption: "ภาพสวัสดิการพนักงาน",
    imageSide: "left" as const,
  },
  {
    eyebrow: "วัฒนธรรมองค์กร",
    heading: "วัฒนธรรมที่ให้เกียรติและร่วมมือ",
    body: "เราสร้างที่ทำงานที่เปิดรับความหลากหลาย เคารพซึ่งกันและกัน และส่งเสริมการทำงานเป็นทีม เพื่อให้ทุกคนรู้สึกเป็นเจ้าขององค์กรร่วมกัน",
    points: ["ความหลากหลายและการมีส่วนร่วม", "ผู้นำที่เข้าถึงได้", "การสื่อสารที่โปร่งใส"],
    imageCaption: "ภาพทีมงาน CP Axtra",
    imageSide: "right" as const,
  },
  {
    eyebrow: "ความยั่งยืน",
    heading: "ขับเคลื่อนธุรกิจอย่างรับผิดชอบ",
    body: "ในฐานะองค์กรค้าปลีกขนาดใหญ่ เรามุ่งมั่นลดผลกระทบต่อสิ่งแวดล้อม สนับสนุนชุมชน และดำเนินธุรกิจตามหลักบรรษัทภิบาล (ESG) อย่างจริงจัง",
    points: ["เป้าหมายลดการปล่อยคาร์บอน", "สนับสนุนผู้ประกอบการรายย่อย", "บริหารงานตามหลัก ESG"],
    imageCaption: "ภาพโครงการเพื่อสังคม",
    imageSide: "left" as const,
  },
];

// LandingSections holds the static institutional sections (server-rendered) and
// the live FeaturedJobs client island.
export function LandingSections() {
  return (
    <>
      {/* Scale band — plain-number institutional proof. */}
      <section aria-label="ภาพรวมองค์กร" className="border-b border-line">
        <Container className="py-[var(--space-section)]">
          <div className="flex flex-col gap-10">
            <SectionHeading
              eyebrow="องค์กรของเรา"
              heading="ขนาดที่มาพร้อมโอกาส"
              lead="ด้วยเครือข่ายค้าปลีกที่ครอบคลุมทั่วประเทศ ทุกตำแหน่งคือโอกาสในการสร้างผลกระทบจริง"
            />
            <StatBand stats={STATS} />
          </div>
        </Container>
      </section>

      {/* Alternating image + text editorial blocks. */}
      <section aria-label="ทำไมต้องร่วมงานกับเรา" className="bg-background">
        <Container className="flex flex-col gap-[var(--space-section)] py-[var(--space-section)]">
          {BLOCKS.map((block) => (
            <MediaBlock
              key={block.heading}
              eyebrow={block.eyebrow}
              heading={block.heading}
              body={block.body}
              points={block.points}
              imageCaption={block.imageCaption}
              imageSide={block.imageSide}
            />
          ))}
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
