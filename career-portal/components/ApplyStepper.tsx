"use client";

import Link from "next/link";
import { useState } from "react";

import { ConsentStep } from "@/components/ConsentStep";
import { LineLoginButton } from "@/components/LineLoginButton";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useApplyMutation } from "@/lib/queries";
import { cn } from "@/lib/utils";

const CONSENT_VERSION = "1.0";
const MAX_RESUME_BYTES = 10 * 1024 * 1024;
const ACCEPTED_TYPES = new Set([
  "application/pdf",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "image/jpeg",
  "image/png",
]);

const STEP_LABELS = ["ยินยอม", "ข้อมูล", "เรซูเม่"];

interface ApplyStepperProps {
  positionId: string;
  positionTitle: string;
}

// validateFile mirrors the server's type/size gate (415/413) so the candidate
// gets instant inline feedback before uploading.
function validateFile(file: File): string | null {
  if (!ACCEPTED_TYPES.has(file.type)) return "รองรับเฉพาะไฟล์ PDF, DOCX, JPG หรือ PNG เท่านั้น";
  if (file.size > MAX_RESUME_BYTES) return "ไฟล์ต้องมีขนาดไม่เกิน 10MB";
  return null;
}

export function ApplyStepper({ positionId, positionTitle }: ApplyStepperProps) {
  const [step, setStep] = useState(0);
  const [consent, setConsent] = useState(false);
  const [fullName, setFullName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [idCard, setIdCard] = useState("");
  const [province, setProvince] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [lineToken, setLineToken] = useState("");
  const [statusToken, setStatusToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const mutation = useApplyMutation();

  const detailsValid = fullName.trim().length > 0;
  const canSubmit = consent && detailsValid && !!file && !fileError && !!lineToken;

  function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const picked = e.target.files?.[0] ?? null;
    if (!picked) {
      setFile(null);
      setFileError(null);
      return;
    }
    const err = validateFile(picked);
    setFileError(err);
    setFile(err ? null : picked);
  }

  function handleSubmit() {
    if (!canSubmit || !file) return;
    mutation.mutate(
      {
        positionId,
        fullName: fullName.trim(),
        phone: phone.trim() || undefined,
        email: email.trim() || undefined,
        idCard: idCard.trim() || undefined,
        province: province.trim() || undefined,
        consentVersion: CONSENT_VERSION,
        resume: file,
        lineIdToken: lineToken,
      },
      { onSuccess: (data) => setStatusToken(data.status_token) },
    );
  }

  async function copyLink() {
    if (!statusToken) return;
    const url = `${window.location.origin}/status?token=${encodeURIComponent(statusToken)}`;
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  }

  // Success — show the opaque status token + a copyable status link.
  if (statusToken) {
    return (
      <div className="space-y-6 text-center">
        <div className="mx-auto grid size-16 place-content-center rounded-full bg-brand-soft text-accent">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M20 6L9 17l-5-5" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>
        <div className="space-y-2">
          <h1 className="text-xl font-semibold">ส่งใบสมัครเรียบร้อยแล้ว</h1>
          <p className="text-sm text-muted-foreground">
            เก็บรหัสติดตามนี้ไว้เพื่อตรวจสอบสถานะใบสมัครของคุณภายหลัง
          </p>
        </div>
        <div className="space-y-2 text-left">
          <Label htmlFor="status-token">รหัสติดตาม</Label>
          <div className="flex items-center gap-2">
            <code
              id="status-token"
              className="min-w-0 flex-1 truncate rounded-lg bg-muted px-3 py-3 font-mono text-sm"
            >
              {statusToken}
            </code>
            <Button type="button" size="tap" variant="outline" onClick={copyLink} className="shrink-0">
              {copied ? "คัดลอกแล้ว" : "คัดลอกลิงก์"}
            </Button>
          </div>
        </div>
        <Link
          href={`/status?token=${encodeURIComponent(statusToken)}`}
          className={cn(buttonVariants({ size: "tap" }), "w-full")}
        >
          ดูสถานะใบสมัคร
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <p className="text-sm text-muted-foreground">สมัครตำแหน่ง</p>
        <h1 className="text-lg font-semibold">{positionTitle}</h1>
      </div>

      {/* Progress indicator */}
      <ol className="flex items-center gap-2" aria-label="ขั้นตอนการสมัคร">
        {STEP_LABELS.map((label, i) => (
          <li key={label} className="flex flex-1 flex-col items-center gap-1.5">
            <div
              className={`h-1.5 w-full rounded-full transition-colors ${i <= step ? "bg-accent" : "bg-muted"}`}
              aria-current={i === step ? "step" : undefined}
            />
            <span className={`text-xs ${i <= step ? "font-medium text-foreground" : "text-muted-foreground"}`}>
              {label}
            </span>
          </li>
        ))}
      </ol>

      {step === 0 ? <ConsentStep checked={consent} onChange={setConsent} /> : null}

      {step === 1 ? (
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="full_name">
              ชื่อ-นามสกุล <span className="text-destructive">*</span>
            </Label>
            <Input
              id="full_name"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              autoComplete="name"
              placeholder="เช่น สมชาย ใจดี"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="phone">เบอร์โทรศัพท์</Label>
            <Input id="phone" type="tel" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} autoComplete="tel" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="email">อีเมล</Label>
            <Input id="email" type="email" inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
          </div>
          <div className="space-y-2">
            <Label htmlFor="id_card">เลขบัตรประชาชน</Label>
            <Input id="id_card" inputMode="numeric" value={idCard} onChange={(e) => setIdCard(e.target.value)} />
          </div>
          <div className="space-y-2">
            <Label htmlFor="province">จังหวัด</Label>
            <Input id="province" value={province} onChange={(e) => setProvince(e.target.value)} />
          </div>
        </div>
      ) : null}

      {step === 2 ? (
        <div className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="resume">
              อัปโหลดเรซูเม่ <span className="text-destructive">*</span>
            </Label>
            <Input
              id="resume"
              type="file"
              accept=".pdf,.docx,image/jpeg,image/png"
              onChange={handleFile}
              aria-invalid={!!fileError}
              className="h-auto py-2.5 file:mr-3 file:rounded-md file:bg-secondary file:px-3 file:py-1.5"
            />
            <p className="text-xs text-muted-foreground">รองรับ PDF, DOCX, JPG, PNG ขนาดไม่เกิน 10MB</p>
            {fileError ? <p className="text-sm text-destructive">{fileError}</p> : null}
            {file && !fileError ? <p className="text-sm text-accent">เลือกไฟล์: {file.name}</p> : null}
          </div>

          <div className="space-y-2">
            <p className="text-sm font-medium">ยืนยันตัวตนด้วย LINE</p>
            <LineLoginButton onToken={setLineToken} connected={!!lineToken} />
          </div>

          {mutation.isError ? (
            <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
              ส่งใบสมัครไม่สำเร็จ: {mutation.error.message}
            </p>
          ) : null}
        </div>
      ) : null}

      {/* Navigation */}
      <div className="flex gap-3 pt-2">
        {step > 0 ? (
          <Button type="button" size="tap" variant="outline" onClick={() => setStep((s) => s - 1)} className="flex-1">
            ย้อนกลับ
          </Button>
        ) : null}
        {step < 2 ? (
          <Button
            type="button"
            size="tap"
            onClick={() => setStep((s) => s + 1)}
            disabled={(step === 0 && !consent) || (step === 1 && !detailsValid)}
            className="flex-1"
          >
            ถัดไป
          </Button>
        ) : (
          <Button type="button" size="tap" onClick={handleSubmit} disabled={!canSubmit || mutation.isPending} className="flex-1">
            {mutation.isPending ? "กำลังส่ง…" : "ส่งใบสมัคร"}
          </Button>
        )}
      </div>
    </div>
  );
}
