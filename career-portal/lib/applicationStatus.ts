// Shared candidate-facing status vocabulary for the application lifecycle.
// Used by StatusCard (single-application view) and the account application
// history list. Friendly Thai labels for the backend status values
// (applications/model.go) — honest but gentle; never expose internal jargon.

export interface StatusMeta {
  label: string;
  detail: string;
  tone: "neutral" | "progress" | "good" | "ended";
}

export const STATUS_META: Record<string, StatusMeta> = {
  pending: { label: "ได้รับใบสมัครแล้ว", detail: "เรากำลังเตรียมตรวจสอบใบสมัครของคุณ", tone: "neutral" },
  parsed: { label: "กำลังตรวจสอบเอกสาร", detail: "ระบบกำลังอ่านข้อมูลจากเรซูเม่ของคุณ", tone: "progress" },
  scored: { label: "ผ่านการคัดกรองเบื้องต้น", detail: "ใบสมัครของคุณผ่านเกณฑ์และรอ HR พิจารณา", tone: "good" },
  shortlisted: { label: "เข้ารอบพิจารณา", detail: "คุณได้รับเลือกเข้าสู่รอบถัดไป HR จะติดต่อกลับ", tone: "good" },
  ai_interview: { label: "เชิญทำแบบสัมภาษณ์เบื้องต้น", detail: "คุณได้รับเชิญให้ทำแบบสัมภาษณ์เบื้องต้นกับผู้ช่วย AI", tone: "good" },
  ai_interviewed: { label: "ทำแบบสัมภาษณ์เบื้องต้นแล้ว", detail: "ทีมงานกำลังพิจารณาผลแบบสัมภาษณ์เบื้องต้นของคุณ", tone: "good" },
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

export const TONE_CLASS: Record<StatusMeta["tone"], string> = {
  neutral: "bg-secondary text-foreground/70",
  progress: "bg-primary text-primary-foreground",
  good: "bg-accent-soft text-primary",
  ended: "bg-destructive/10 text-destructive",
};

export function metaFor(status: string): StatusMeta {
  return STATUS_META[status] ?? { label: status, detail: "สถานะใบสมัครของคุณ", tone: "neutral" };
}

export function formatThaiDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat("th-TH", { dateStyle: "long" }).format(d);
}
