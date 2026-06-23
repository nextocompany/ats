"use client";

import Link from "next/link";
import { useState } from "react";

import { Button, buttonVariants } from "@/components/ui/button";
import { ConsentStep } from "@/components/ConsentStep";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useApplyMutation, useQuickApply } from "@/lib/queries";
import type { Account } from "@/lib/types";
import { cn } from "@/lib/utils";

const CONSENT_VERSION = "1.0";
const MAX_RESUME_BYTES = 10 * 1024 * 1024;
const ACCEPTED_TYPES = new Set([
  "application/pdf",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "image/jpeg",
  "image/png",
]);

// ApplyPrefill carries values from an external source (e.g. an "Apply with SEEK"
// deep link). They seed the editable fields, taking precedence over the saved
// account values so the candidate sees the data they came in with.
export interface ApplyPrefill {
  fullName?: string;
  email?: string;
  phone?: string;
  province?: string;
}

interface ApplyStepperProps {
  positionId: string;
  positionTitle: string;
  // The logged-in member — used to prefill the form and enable quick-apply.
  account: Account;
  // Optional external prefill (SEEK/job-board deep link); overrides account values.
  prefill?: ApplyPrefill;
}

function validateFile(file: File): string | null {
  if (!ACCEPTED_TYPES.has(file.type)) return "รองรับเฉพาะไฟล์ PDF, DOCX, JPG หรือ PNG เท่านั้น";
  if (file.size > MAX_RESUME_BYTES) return "ไฟล์ต้องมีขนาดไม่เกิน 10MB";
  return null;
}

// ApplyStepper is account-first. It opens on a prefilled review with a one-tap
// "apply with saved resume", or lets the member edit details / upload a different
// resume before submitting.
export function ApplyStepper({ positionId, positionTitle, account, prefill }: ApplyStepperProps) {
  // External prefill (e.g. SEEK deep link) takes precedence over saved account
  // values; a blank prefill field falls back to the account.
  const [mode, setMode] = useState<"review" | "edit">("review");
  const [fullName, setFullName] = useState(prefill?.fullName || account.full_name);
  const [phone, setPhone] = useState(prefill?.phone || account.phone);
  const [email, setEmail] = useState(prefill?.email || account.email);
  const [province, setProvince] = useState(prefill?.province || account.province);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [statusToken, setStatusToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  // A member who signed up via OAuth (or before consent was captured) may not yet
  // have given PDPA consent. In that case we must collect it here before applying
  // — otherwise the backend rejects the application with no way to consent.
  const needsConsent = !account.pdpa_consent;
  const [consent, setConsent] = useState(account.pdpa_consent);
  const consentOk = !needsConsent || consent;

  const quick = useQuickApply();
  const apply = useApplyMutation();
  const pending = quick.isPending || apply.isPending;
  const errorMessage = quick.error?.message || apply.error?.message || null;

  // Phone is always required; email is too — a LINE-only account often has no
  // email, so the form forces it (prefilled from the account/LINE when present).
  const fullNameOk = fullName.trim().length > 0;
  const phoneOk = phone.trim().length > 0;
  const emailOk = /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
  // The saved-profile (quick) path can only run when the account already carries a
  // complete contact set; otherwise the candidate is sent to the form to fill it.
  const profileComplete = fullNameOk && phoneOk && emailOk;

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

  function submitQuick() {
    if (!consentOk || !profileComplete) return;
    quick.mutate(
      { positionId, consentGiven: consent },
      { onSuccess: (d) => setStatusToken(d.status_token) },
    );
  }

  function submitForm() {
    if (!fullNameOk || !phoneOk || !emailOk || !file || fileError || !consentOk) return;
    apply.mutate(
      {
        positionId,
        fullName: fullName.trim(),
        phone: phone.trim(),
        email: email.trim(),
        province: province.trim() || undefined,
        consentVersion: CONSENT_VERSION,
        resume: file,
      },
      { onSuccess: (d) => setStatusToken(d.status_token) },
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

  // Success — opaque status token + a copyable status link.
  if (statusToken) {
    return (
      <div className="space-y-6 text-center">
        <div className="mx-auto grid size-16 place-content-center rounded-full bg-accent-soft text-primary">
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M20 6L9 17l-5-5" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          </svg>
        </div>
        <div className="space-y-2">
          <h1 className="text-xl font-semibold">ส่งใบสมัครเรียบร้อยแล้ว</h1>
          <p className="text-sm text-muted-foreground">เก็บรหัสติดตามนี้ไว้เพื่อตรวจสอบสถานะใบสมัครของคุณภายหลัง</p>
        </div>
        <div className="space-y-2 text-left">
          <Label htmlFor="status-token">รหัสติดตาม</Label>
          <div className="flex items-center gap-2">
            <code id="status-token" className="min-w-0 flex-1 truncate rounded-lg border border-line bg-surface-muted px-3 py-3 font-mono text-sm">
              {statusToken}
            </code>
            <Button type="button" size="tap" variant="outline" onClick={copyLink} className="shrink-0">
              {copied ? "คัดลอกแล้ว" : "คัดลอกลิงก์"}
            </Button>
          </div>
        </div>
        <Link href={`/status?token=${encodeURIComponent(statusToken)}`} className={cn(buttonVariants({ size: "tap" }), "w-full")}>
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

      {mode === "review" ? (
        <div className="space-y-5">
          <dl className="space-y-3 rounded-xl border border-line bg-surface-muted p-4 text-sm">
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">ชื่อ-นามสกุล</dt>
              <dd className="font-medium">{fullName || "-"}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">เบอร์โทรศัพท์</dt>
              <dd className={phoneOk ? "font-medium" : "font-medium text-destructive"}>{phone || "ยังไม่ได้กรอก"}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">อีเมล</dt>
              <dd className={emailOk ? "font-medium" : "font-medium text-destructive"}>{email || "ยังไม่ได้กรอก"}</dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-muted-foreground">เรซูเม่</dt>
              <dd className="font-medium">
                {account.has_resume ? `บันทึกไว้แล้ว (${account.resume_file_type.toUpperCase()})` : "ยังไม่มี"}
              </dd>
            </div>
          </dl>

          {needsConsent ? <ConsentStep checked={consent} onChange={setConsent} /> : null}

          {errorMessage ? (
            <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
              ส่งใบสมัครไม่สำเร็จ: {errorMessage}
            </p>
          ) : null}

          {!profileComplete ? (
            <p className="rounded-lg bg-secondary px-3 py-2 text-sm text-muted-foreground">
              กรุณากรอกเบอร์โทรศัพท์และอีเมลให้ครบก่อนสมัคร
            </p>
          ) : null}

          {account.has_resume && profileComplete ? (
            <Button type="button" size="tap" onClick={submitQuick} disabled={pending || !consentOk} className="w-full">
              {quick.isPending ? "กำลังส่ง…" : "สมัครด้วยเรซูเม่ที่บันทึกไว้"}
            </Button>
          ) : null}
          <Button type="button" size="tap" variant="outline" onClick={() => setMode("edit")} className="w-full">
            {account.has_resume && profileComplete ? "แก้ไขข้อมูล / อัปโหลดเรซูเม่ใหม่" : "กรอกข้อมูล / อัปโหลดเรซูเม่"}
          </Button>
        </div>
      ) : (
        <div className="space-y-5">
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="full_name">
                ชื่อ-นามสกุล <span className="text-destructive">*</span>
              </Label>
              <Input id="full_name" value={fullName} onChange={(e) => setFullName(e.target.value)} autoComplete="name" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="phone">
                เบอร์โทรศัพท์ <span className="text-destructive">*</span>
              </Label>
              <Input id="phone" type="tel" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} autoComplete="tel" aria-invalid={phone.length > 0 && !phoneOk} />
              {phone.length > 0 && !phoneOk ? <p className="text-sm text-destructive">กรุณากรอกเบอร์โทรศัพท์</p> : null}
            </div>
            <div className="space-y-2">
              <Label htmlFor="email">
                อีเมล <span className="text-destructive">*</span>
              </Label>
              <Input id="email" type="email" inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" aria-invalid={email.length > 0 && !emailOk} />
              {email.length > 0 && !emailOk ? <p className="text-sm text-destructive">กรุณากรอกอีเมลให้ถูกต้อง</p> : null}
            </div>
            <div className="space-y-2">
              <Label htmlFor="province">จังหวัด</Label>
              <Input id="province" value={province} onChange={(e) => setProvince(e.target.value)} />
            </div>
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
          </div>

          {needsConsent ? <ConsentStep checked={consent} onChange={setConsent} /> : null}

          {errorMessage ? (
            <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
              ส่งใบสมัครไม่สำเร็จ: {errorMessage}
            </p>
          ) : null}

          <div className="flex gap-3">
            <Button type="button" size="tap" variant="outline" onClick={() => setMode("review")} className="flex-1">
              ย้อนกลับ
            </Button>
            <Button
              type="button"
              size="tap"
              onClick={submitForm}
              disabled={!fullNameOk || !phoneOk || !emailOk || !file || !!fileError || pending || !consentOk}
              className="flex-1"
            >
              {apply.isPending ? "กำลังส่ง…" : "ส่งใบสมัคร"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
