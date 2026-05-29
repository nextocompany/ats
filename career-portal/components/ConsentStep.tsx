"use client";

import { Checkbox } from "@/components/ui/checkbox";

interface ConsentStepProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
}

// ConsentStep presents the PDPA purpose/retention notice and the mandatory
// consent checkbox. Consent is required before any data is submitted (F13).
export function ConsentStep({ checked, onChange }: ConsentStepProps) {
  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <h2 className="text-lg font-semibold">ความยินยอมในการใช้ข้อมูล</h2>
        <p className="text-sm text-muted-foreground">
          ก่อนสมัครงาน เราขอความยินยอมในการเก็บและใช้ข้อมูลของคุณ
        </p>
      </div>

      <div className="space-y-3 rounded-2xl bg-muted/60 p-4 text-sm leading-relaxed text-foreground/80">
        <p>
          <span className="font-medium text-foreground">วัตถุประสงค์:</span>{" "}
          เพื่อพิจารณาใบสมัครงาน คัดกรองคุณสมบัติ และติดต่อกลับเกี่ยวกับตำแหน่งที่คุณสมัคร
        </p>
        <p>
          <span className="font-medium text-foreground">ข้อมูลที่เก็บ:</span>{" "}
          ชื่อ-นามสกุล ข้อมูลติดต่อ และเอกสารเรซูเม่ที่คุณอัปโหลด
        </p>
        <p>
          <span className="font-medium text-foreground">ระยะเวลาจัดเก็บ:</span>{" "}
          เก็บไว้ไม่เกิน 1 ปี หลังจากนั้นจะถูกลบหรือทำให้ไม่ระบุตัวตน
        </p>
      </div>

      <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-border p-4 transition-colors has-data-checked:border-primary has-data-checked:bg-brand-soft/50">
        <Checkbox
          checked={checked}
          onCheckedChange={(value) => onChange(value === true)}
          aria-label="ยินยอมให้เก็บและใช้ข้อมูลส่วนบุคคล"
          className="mt-0.5"
        />
        <span className="text-sm leading-relaxed">
          ฉัน<span className="font-medium">ยินยอม</span>ให้บริษัทเก็บรวบรวมและใช้ข้อมูลส่วนบุคคลของฉัน
          ตามวัตถุประสงค์ข้างต้น
        </span>
      </label>
    </div>
  );
}
