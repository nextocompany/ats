import type { ApplicationStatus } from "@/lib/types";

interface StatusMeta {
  label: string;
  detail: string;
  tone: "neutral" | "progress" | "good" | "ended";
}

// Friendly Thai labels for the backend status values (applications/model.go).
// Candidate-facing: honest but gentle; never expose internal jargon.
const STATUS_META: Record<string, StatusMeta> = {
  pending: { label: "ได้รับใบสมัครแล้ว", detail: "เรากำลังเตรียมตรวจสอบใบสมัครของคุณ", tone: "neutral" },
  parsed: { label: "กำลังตรวจสอบเอกสาร", detail: "ระบบกำลังอ่านข้อมูลจากเรซูเม่ของคุณ", tone: "progress" },
  scored: { label: "ผ่านการคัดกรองเบื้องต้น", detail: "ใบสมัครของคุณผ่านเกณฑ์และรอ HR พิจารณา", tone: "good" },
  shortlisted: { label: "เข้ารอบพิจารณา", detail: "คุณได้รับเลือกเข้าสู่รอบถัดไป HR จะติดต่อกลับ", tone: "good" },
  interview: { label: "นัดสัมภาษณ์", detail: "ทีมงานจะติดต่อเพื่อนัดหมายสัมภาษณ์", tone: "good" },
  interviewed: { label: "สัมภาษณ์เสร็จสิ้น", detail: "ทีมงานกำลังพิจารณาผลการสัมภาษณ์", tone: "good" },
  pending_approval: { label: "อยู่ระหว่างอนุมัติการจ้าง", detail: "ใบสมัครของคุณอยู่ระหว่างการอนุมัติภายใน", tone: "progress" },
  offer: { label: "คุณได้รับข้อเสนอการจ้างงาน", detail: "เข้าสู่ระบบเพื่อดูรายละเอียดและตอบรับข้อเสนอ", tone: "good" },
  hired: { label: "ยินดีด้วย! คุณได้รับการคัดเลือก", detail: "ทีม HR จะติดต่อเรื่องการเริ่มงาน", tone: "good" },
  rejected: { label: "ยังไม่ผ่านการพิจารณาในรอบนี้", detail: "ขอบคุณที่สนใจร่วมงานกับเรา เราจะเก็บข้อมูลไว้พิจารณาในโอกาสหน้า", tone: "ended" },
  invalid_resume: {
    label: "ไฟล์ที่อัปโหลดไม่ใช่เรซูเม่",
    detail:
      "ไฟล์ที่คุณอัปโหลดอาจไม่ใช่เรซูเม่/CV กรุณาสมัครใหม่อีกครั้งพร้อมแนบไฟล์เรซูเม่ของคุณ เพื่อให้เราพิจารณาใบสมัครได้",
    tone: "neutral",
  },
  failed: { label: "เกิดข้อผิดพลาดในการประมวลผล", detail: "กรุณาลองสมัครใหม่อีกครั้ง หรือติดต่อทีมงาน", tone: "ended" },
};

const TONE_CLASS: Record<StatusMeta["tone"], string> = {
  neutral: "bg-secondary text-foreground/70",
  progress: "bg-primary text-primary-foreground",
  good: "bg-accent-soft text-primary",
  ended: "bg-destructive/10 text-destructive",
};

function metaFor(status: string): StatusMeta {
  return STATUS_META[status] ?? { label: status, detail: "สถานะใบสมัครของคุณ", tone: "neutral" };
}

function formatThaiDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("th-TH", { dateStyle: "long" }).format(d);
}

export function StatusCard({ status }: { status: ApplicationStatus }) {
  const meta = metaFor(status.status);
  return (
    <div className="space-y-5 rounded-xl border border-line bg-card p-6">
      <span className={`inline-flex rounded-full px-3 py-1 text-sm font-medium ${TONE_CLASS[meta.tone]}`}>
        {meta.label}
      </span>
      <p className="text-sm leading-relaxed text-muted-foreground">{meta.detail}</p>
      <dl className="space-y-3 border-t border-line pt-4 text-sm">
        {status.position ? (
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">ตำแหน่ง</dt>
            <dd className="text-right font-medium">{status.position}</dd>
          </div>
        ) : null}
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">วันที่สมัคร</dt>
          <dd className="text-right font-medium">{formatThaiDate(status.applied_at)}</dd>
        </div>
      </dl>
    </div>
  );
}
