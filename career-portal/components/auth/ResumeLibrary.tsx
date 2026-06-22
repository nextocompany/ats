"use client";

// Candidate CV history: the account keeps up to RESUME_LIMIT resumes; one is the
// default used for quick-apply. Upload adds to the history (blocked when full),
// and each entry can be made default or deleted.
import { useEffect, useState } from "react";

import { ApiError } from "@/lib/api";
import { deleteResume, getResumes, RESUME_LIMIT, setDefaultResume, uploadResume } from "@/lib/auth";
import { useCandidate } from "@/lib/session";
import type { AccountResume } from "@/lib/types";
import { UPLOAD_ACCEPT_ATTR, validateUploadFile } from "@/lib/upload";

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

function formatDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleDateString("th-TH", { day: "numeric", month: "short", year: "numeric" });
}

export function ResumeLibrary() {
  const { refresh } = useCandidate();
  const [resumes, setResumes] = useState<AccountResume[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getResumes()
      .then(setResumes)
      .catch(() => setResumes([]));
  }, []);

  const count = resumes?.length ?? 0;
  const full = count >= RESUME_LIMIT;

  async function onUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0] ?? null;
    e.target.value = ""; // allow re-selecting the same file
    if (!file) return;
    const ferr = validateFile(file);
    if (ferr) {
      setError(ferr);
      return;
    }
    setError(null);
    setBusy(true);
    try {
      setResumes(await uploadResume(file));
      await refresh();
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 409
          ? `เก็บ CV ได้สูงสุด ${RESUME_LIMIT} ไฟล์ กรุณาลบไฟล์เก่าก่อน`
          : "อัปโหลดไม่สำเร็จ กรุณาลองใหม่",
      );
    } finally {
      setBusy(false);
    }
  }

  async function makeDefault(id: string) {
    setError(null);
    setBusy(true);
    try {
      setResumes(await setDefaultResume(id));
      await refresh();
    } catch {
      setError("ตั้งค่าเริ่มต้นไม่สำเร็จ");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: string) {
    setError(null);
    setBusy(true);
    try {
      setResumes(await deleteResume(id));
      await refresh();
    } catch {
      setError("ลบไม่สำเร็จ");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-muted-foreground">
          ประวัติเรซูเม่ {count}/{RESUME_LIMIT} ไฟล์ - ไฟล์ที่ตั้งเป็นค่าเริ่มต้นจะถูกใช้ตอนสมัครงาน
        </p>
      </div>

      {resumes === null ? (
        <p className="text-sm text-muted-foreground">กำลังโหลด…</p>
      ) : resumes.length === 0 ? (
        <p className="text-sm text-muted-foreground">ยังไม่มีเรซูเม่ที่บันทึกไว้</p>
      ) : (
        <ul className="space-y-2">
          {resumes.map((r) => (
            <li
              key={r.id}
              className="flex items-center gap-3 rounded-lg border border-line bg-secondary/40 px-4 py-3"
            >
              <div className="min-w-0 flex-1">
                <p className="flex items-center gap-2 truncate text-sm font-medium text-foreground">
                  <span className="truncate">{r.original_filename || `เรซูเม่ (${r.file_type.toUpperCase()})`}</span>
                  {r.is_default ? (
                    <span className="shrink-0 rounded-full bg-accent/15 px-2 py-0.5 text-xs font-semibold text-accent">
                      ค่าเริ่มต้น
                    </span>
                  ) : null}
                </p>
                <p className="text-xs text-muted-foreground">
                  {r.file_type.toUpperCase()}
                  {formatDate(r.created_at) ? ` · ${formatDate(r.created_at)}` : ""}
                </p>
              </div>
              {!r.is_default ? (
                <button
                  type="button"
                  onClick={() => makeDefault(r.id)}
                  disabled={busy}
                  className="shrink-0 rounded-md border border-line px-2.5 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-secondary disabled:opacity-50"
                >
                  ตั้งเป็นค่าเริ่มต้น
                </button>
              ) : null}
              <button
                type="button"
                onClick={() => remove(r.id)}
                disabled={busy}
                aria-label="ลบเรซูเม่"
                className="shrink-0 rounded-md border border-line px-2.5 py-1.5 text-xs font-medium text-destructive transition-colors hover:bg-destructive/10 disabled:opacity-50"
              >
                ลบ
              </button>
            </li>
          ))}
        </ul>
      )}

      {error ? (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      <div className="space-y-1.5">
        <label
          className={`inline-flex cursor-pointer items-center rounded-lg border border-line bg-secondary px-3 py-2 text-sm font-medium text-foreground transition-colors hover:bg-secondary/70 ${
            full || busy ? "pointer-events-none opacity-50" : ""
          }`}
        >
          {busy ? "กำลังอัปโหลด…" : "อัปโหลดเรซูเม่ใหม่"}
          <input
            type="file"
            accept={UPLOAD_ACCEPT_ATTR}
            onChange={onUpload}
            disabled={full || busy}
            className="sr-only"
          />
        </label>
        <p className="text-xs text-muted-foreground">
          {full
            ? `ครบ ${RESUME_LIMIT} ไฟล์แล้ว - ลบไฟล์เก่าก่อนจึงจะอัปโหลดเพิ่มได้`
            : "รองรับ PDF, DOCX, JPG, PNG ขนาดไม่เกิน 10MB"}
        </p>
      </div>
    </div>
  );
}
