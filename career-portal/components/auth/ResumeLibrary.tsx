"use client";

// Candidate CV history: the account keeps up to RESUME_LIMIT resumes; one is the
// default used for quick-apply. Upload adds to the history (blocked when full),
// and each entry can be made default or deleted.
import { useEffect, useState } from "react";
import { FileText, Image as ImageIcon, MoreVertical, Upload } from "lucide-react";

import { ApiError } from "@/lib/api";
import { deleteResume, getResumes, getResumeViewUrl, RESUME_LIMIT, setDefaultResume, uploadResume } from "@/lib/auth";
import { useCandidate } from "@/lib/session";
import type { AccountResume } from "@/lib/types";
import { UPLOAD_ACCEPT_ATTR, validateUploadFile } from "@/lib/upload";
import { cn } from "@/lib/utils";

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

interface ResumeLibraryProps {
  // hideHeading omits the internal count line (when a parent section already shows
  // the title + count, e.g. the account page's AccountSection).
  hideHeading?: boolean;
  // onCountChange reports the current resume count up to a parent (e.g. the account
  // summary figure) whenever the library loads or changes.
  onCountChange?: (count: number) => void;
}

export function ResumeLibrary({ hideHeading, onCountChange }: ResumeLibraryProps = {}) {
  const { refresh } = useCandidate();
  const [resumes, setResumes] = useState<AccountResume[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getResumes()
      .then(setResumes)
      .catch(() => setResumes([]));
  }, []);

  // Report the count up to any parent (account summary) on load + every change.
  useEffect(() => {
    if (resumes) onCountChange?.(resumes.length);
  }, [resumes, onCountChange]);

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

  // view opens a CV in a new tab. The blank tab is opened synchronously (inside
  // the click gesture) so Safari does not block it, then redirected once the
  // short-lived signed URL resolves. opener is severed (cross-origin blob host).
  async function view(id: string) {
    setError(null);
    const w = window.open("about:blank", "_blank");
    try {
      const url = await getResumeViewUrl(id);
      if (w) {
        w.opener = null;
        w.location.replace(url);
      } else {
        window.location.assign(url); // popup blocked → navigate current tab
      }
    } catch {
      w?.close();
      setError("เปิดไฟล์ไม่สำเร็จ กรุณาลองใหม่");
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
      {!hideHeading ? (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            ประวัติเรซูเม่ {count}/{RESUME_LIMIT} ไฟล์ - ไฟล์ที่ตั้งเป็นค่าเริ่มต้นจะถูกใช้ตอนสมัครงาน
          </p>
        </div>
      ) : null}

      {resumes === null ? (
        <p className="text-sm text-muted-foreground">กำลังโหลด…</p>
      ) : resumes.length === 0 ? (
        <p className="text-sm text-muted-foreground">ยังไม่มีเรซูเม่ที่บันทึกไว้</p>
      ) : (
        <ul className="flex flex-col divide-y divide-line">
          {resumes.map((r) => (
            <ResumeRow key={r.id} resume={r} busy={busy} onView={view} onMakeDefault={makeDefault} onDelete={remove} />
          ))}
        </ul>
      )}

      {error ? (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      <div className="flex flex-col gap-2 border-t border-line pt-4">
        <label
          className={cn(
            "inline-flex h-11 cursor-pointer items-center justify-center gap-2 self-stretch rounded-lg border border-line bg-secondary px-4 text-sm font-medium text-foreground transition-colors sm:self-start",
            "hover:border-foreground/30 hover:bg-[color-mix(in_oklch,var(--secondary),var(--foreground)_6%)]",
            "focus-within:outline-none focus-within:ring-2 focus-within:ring-ring/60 focus-within:ring-offset-2 focus-within:ring-offset-background",
            (full || busy) && "pointer-events-none opacity-50",
          )}
        >
          <Upload className="size-4" aria-hidden="true" />
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

// ResumeRow renders one CV: an icon, filename + default badge + meta, and the
// view/set-default/delete actions. At >=768px the actions are an inline trio; at
// <768px they collapse into a 44px kebab toggle that reveals an in-flow action
// list below the row (in-flow so the Card's overflow can never clip it). Every
// control is a >=44px tap target for the LINE in-app browser.
function ResumeRow({
  resume,
  busy,
  onView,
  onMakeDefault,
  onDelete,
}: {
  resume: AccountResume;
  busy: boolean;
  onView: (id: string) => void;
  onMakeDefault: (id: string) => void;
  onDelete: (id: string) => void;
}) {
  const isImage = resume.file_type === "image";
  const Icon = isImage ? ImageIcon : FileText;
  const [open, setOpen] = useState(false);
  const panelId = `resume-actions-${resume.id}`;
  const name = resume.original_filename || `เรซูเม่ (${resume.file_type.toUpperCase()})`;
  const meta = formatDate(resume.created_at)
    ? `${resume.file_type.toUpperCase()} · ${formatDate(resume.created_at)}`
    : resume.file_type.toUpperCase();

  return (
    <li className="flex flex-col py-3.5 first:pt-0">
      <div className="flex items-center gap-3">
        <span
          aria-hidden="true"
          className="grid size-10 shrink-0 place-content-center rounded-lg border border-line bg-secondary/50 text-muted-foreground"
        >
          <Icon className="size-4.5" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="flex items-center gap-2 text-sm font-medium text-foreground">
            <span className="truncate">{name}</span>
            {resume.is_default ? (
              <span className="shrink-0 rounded-full bg-accent-soft px-2 py-0.5 text-[0.7rem] font-semibold tracking-wide text-primary">
                ค่าเริ่มต้น
              </span>
            ) : null}
          </p>
          <p className="text-xs text-muted-foreground">{meta}</p>
        </div>

        {/* >=768px: inline trio, each >=44px tall. */}
        <div className="hidden shrink-0 items-center gap-1.5 md:flex">
          <RowAction onClick={() => onView(resume.id)} disabled={busy}>
            ดูไฟล์
          </RowAction>
          {!resume.is_default ? (
            <RowAction onClick={() => onMakeDefault(resume.id)} disabled={busy}>
              ตั้งเป็นค่าเริ่มต้น
            </RowAction>
          ) : null}
          <RowAction onClick={() => onDelete(resume.id)} disabled={busy} tone="danger" ariaLabel="ลบเรซูเม่">
            ลบ
          </RowAction>
        </div>

        {/* <768px: a 44px toggle revealing an in-flow action list below the row. */}
        <button
          type="button"
          aria-label={`ตัวเลือกสำหรับ ${name}`}
          aria-expanded={open}
          aria-controls={panelId}
          onClick={() => setOpen((v) => !v)}
          className={cn(
            "grid size-11 shrink-0 place-content-center rounded-md border border-line text-foreground transition-colors md:hidden",
            "hover:border-foreground/30 hover:bg-secondary aria-expanded:bg-secondary",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
          )}
        >
          <MoreVertical className="size-4" aria-hidden="true" />
        </button>
      </div>

      {open ? (
        <div id={panelId} className="mt-2.5 flex flex-col gap-1.5 md:hidden">
          <RowAction wide onClick={() => onView(resume.id)} disabled={busy}>
            ดูไฟล์
          </RowAction>
          {!resume.is_default ? (
            <RowAction wide onClick={() => onMakeDefault(resume.id)} disabled={busy}>
              ตั้งเป็นค่าเริ่มต้น
            </RowAction>
          ) : null}
          <RowAction wide tone="danger" onClick={() => onDelete(resume.id)} disabled={busy}>
            ลบ
          </RowAction>
        </div>
      ) : null}
    </li>
  );
}

function RowAction({
  children,
  tone = "default",
  ariaLabel,
  wide,
  onClick,
  disabled,
}: {
  children: React.ReactNode;
  tone?: "default" | "danger";
  ariaLabel?: string;
  // wide renders the full-width, left-aligned variant used in the mobile in-flow
  // action list; the default is the compact inline (>=768px) chip.
  wide?: boolean;
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "inline-flex h-11 items-center rounded-md border border-line text-xs font-medium transition-colors disabled:opacity-50",
        wide ? "justify-start px-4 text-sm" : "px-3",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:ring-offset-2 focus-visible:ring-offset-background",
        tone === "danger"
          ? "text-destructive hover:bg-destructive/10"
          : "text-foreground hover:border-foreground/30 hover:bg-secondary",
      )}
    >
      {children}
    </button>
  );
}
