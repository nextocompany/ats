"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { uploadResume } from "@/lib/auth";
import { useCandidate } from "@/lib/session";
import type { Account } from "@/lib/types";
import { UPLOAD_ACCEPT_ATTR, validateUploadFile } from "@/lib/upload";

interface ResumeUploadStepProps {
  account: Account;
  submitLabel: string;
  onUploaded: () => void;
}

// validateFile mirrors the server gate (415/413) for instant inline feedback,
// mapping the shared validator's error code to Thai copy.
function validateFile(file: File): string | null {
  switch (validateUploadFile(file)) {
    case "fileTypeInvalid":
      return "รองรับเฉพาะไฟล์ PDF, DOCX, JPG หรือ PNG เท่านั้น";
    case "fileTooLarge":
      return "ไฟล์ต้องมีขนาดไม่เกิน 10MB";
    default:
      return null;
  }
}

// ResumeUploadStep saves the account's reusable resume — uploaded once, reused on
// every apply.
export function ResumeUploadStep({ account, submitLabel, onUploaded }: ResumeUploadStepProps) {
  const { refresh } = useCandidate();
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

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

  async function save() {
    setError(null);
    if (!file || fileError) return;
    setBusy(true);
    try {
      await uploadResume(file);
      await refresh();
      onUploaded();
    } catch {
      setError("อัปโหลดเรซูเม่ไม่สำเร็จ กรุณาลองใหม่");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <Label htmlFor="resume">
          อัปโหลดเรซูเม่ {account.has_resume ? "" : <span className="text-destructive">*</span>}
        </Label>
        <Input
          id="resume"
          type="file"
          accept={UPLOAD_ACCEPT_ATTR}
          onChange={handleFile}
          aria-invalid={!!fileError}
          className="h-auto py-2.5 file:mr-3 file:rounded-md file:bg-secondary file:px-3 file:py-1.5"
        />
        <p className="text-xs text-muted-foreground">รองรับ PDF, DOCX, JPG, PNG ขนาดไม่เกิน 10MB</p>
        {fileError ? <p className="text-sm text-destructive">{fileError}</p> : null}
        {file && !fileError ? <p className="text-sm text-accent">เลือกไฟล์: {file.name}</p> : null}
        {account.has_resume && !file ? (
          <p className="text-sm text-[oklch(50%_0.16_150)]">มีเรซูเม่ที่บันทึกไว้แล้ว ({account.resume_file_type.toUpperCase()})</p>
        ) : null}
      </div>

      {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}

      <Button type="button" size="tap" onClick={save} disabled={!file || !!fileError || busy} className="w-full">
        {busy ? "กำลังอัปโหลด…" : submitLabel}
      </Button>
    </div>
  );
}
