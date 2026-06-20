"use client";

// HR bulk CV upload: pick one position, drop many resumes, submit. Each file
// becomes one application + pipeline job; the parsed name replaces the filename
// placeholder once processing completes. Server re-validates everything.
import { useState } from "react";
import Link from "next/link";
import { Loader2, UploadCloud, CheckCircle2, XCircle } from "lucide-react";
import { toast } from "sonner";

import type { BulkIntakeResult } from "@/lib/types";
import { useBulkIntake, usePositions } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const MAX_FILES = 50;
const ACCEPT = ".pdf,.docx,.png,.jpg,.jpeg,application/pdf,image/png,image/jpeg";

export function BulkUpload() {
  const { data: positions, isLoading: posLoading } = usePositions();
  const bulk = useBulkIntake();
  const [positionId, setPositionId] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [result, setResult] = useState<BulkIntakeResult | null>(null);

  const tooMany = files.length > MAX_FILES;
  const canSubmit = positionId !== "" && files.length > 0 && !tooMany && !bulk.isPending;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    await bulk.mutateAsync(
      { positionId, files },
      {
        onSuccess: (r) => {
          setResult(r);
          setFiles([]);
          toast.success(`อัปโหลดสำเร็จ ${r.succeeded}/${r.total} ไฟล์`);
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : "อัปโหลดไม่สำเร็จ"),
      },
    );
  }

  return (
    <div className="settle space-y-6">
      <div>
        <p className="eyebrow">Bulk intake</p>
        <h1 className="mt-1 font-heading text-2xl font-semibold tracking-tight">อัปโหลด CV จำนวนมาก</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          เลือกตำแหน่ง แล้วเลือกไฟล์ CV ได้สูงสุด {MAX_FILES} ไฟล์ (PDF, DOCX, JPG, PNG ≤10MB/ไฟล์) ระบบจะคัดกรองอัตโนมัติ
        </p>
      </div>

      <form onSubmit={submit} className="max-w-xl space-y-4 rounded-xl bg-card p-6 ring-1 ring-hairline">
        <label className="block space-y-1.5">
          <span className="text-xs font-medium text-foreground">ตำแหน่ง</span>
          <Select value={positionId} onValueChange={(v) => setPositionId(v ?? "")}>
            <SelectTrigger className="w-full" aria-label="ตำแหน่ง">
              <SelectValue placeholder={posLoading ? "กำลังโหลด…" : "เลือกตำแหน่ง…"} />
            </SelectTrigger>
            <SelectContent>
              {(positions ?? []).map((p) => (
                <SelectItem key={p.id} value={p.id}>
                  {p.title_th || p.title_en}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>

        <label className="block space-y-1.5">
          <span className="text-xs font-medium text-foreground">ไฟล์ CV</span>
          <div className="flex items-center gap-3 rounded-lg border border-dashed border-input bg-transparent px-4 py-6">
            <UploadCloud className="size-5 shrink-0 text-muted-foreground" />
            <input
              type="file"
              multiple
              accept={ACCEPT}
              onChange={(e) => setFiles(Array.from(e.target.files ?? []))}
              className="block w-full text-sm text-foreground file:mr-3 file:rounded-md file:border-0 file:bg-secondary file:px-3 file:py-1.5 file:text-sm file:font-medium file:text-secondary-foreground"
            />
          </div>
          {files.length > 0 && (
            <span className={`text-xs ${tooMany ? "text-destructive" : "text-muted-foreground"}`}>
              เลือกแล้ว {files.length} ไฟล์{tooMany ? ` - เกิน ${MAX_FILES} ไฟล์` : ""}
            </span>
          )}
        </label>

        <Button type="submit" disabled={!canSubmit} className="gap-2">
          {bulk.isPending && <Loader2 className="size-4 animate-spin" />}
          อัปโหลด {files.length > 0 ? `(${files.length})` : ""}
        </Button>
      </form>

      {result && <ResultPanel result={result} />}
    </div>
  );
}

function ResultPanel({ result }: { result: BulkIntakeResult }) {
  return (
    <div className="max-w-xl space-y-4">
      <div className="flex gap-4 text-sm">
        <span className="inline-flex items-center gap-1.5 text-[var(--score-high)]">
          <CheckCircle2 className="size-4" /> สำเร็จ {result.succeeded}
        </span>
        {result.failed_count > 0 && (
          <span className="inline-flex items-center gap-1.5 text-destructive">
            <XCircle className="size-4" /> ล้มเหลว {result.failed_count}
          </span>
        )}
      </div>

      {result.created.length > 0 && (
        <ul className="divide-y divide-hairline rounded-lg bg-card ring-1 ring-hairline">
          {result.created.map((c) => (
            <li key={c.application_id} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
              <span className="truncate text-foreground">{c.filename}</span>
              <Link
                href={`/applications/${c.application_id}`}
                className="shrink-0 text-xs font-medium text-primary underline-offset-4 hover:underline"
              >
                เปิด
              </Link>
            </li>
          ))}
        </ul>
      )}

      {result.failed.length > 0 && (
        <ul className="divide-y divide-destructive/15 rounded-lg bg-destructive/5 ring-1 ring-destructive/20">
          {result.failed.map((f, i) => (
            <li key={`${f.filename}-${i}`} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
              <span className="truncate text-foreground">{f.filename}</span>
              <span className="shrink-0 text-xs text-destructive">{f.error}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
