"use client";

// Structured interview feedback the hiring panel records during the interview
// stage. Many entries per application (one per interviewer/round); recorded
// independently of "mark interview done". Read is open to anyone who can see the
// application; the add form is gated to sgm/hr_manager/super_admin (server-enforced).
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import type {
  InterviewCompetencies,
  InterviewFeedback,
  InterviewRecommendation,
} from "@/lib/types";
import { useAddInterviewFeedback, useInterviewFeedback, useMe } from "@/lib/queries";
import { canRecordInterviewFeedback } from "@/lib/roles";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const REC_LABEL: Record<InterviewRecommendation, string> = {
  pass: "ผ่าน",
  hold: "รอพิจารณา",
  fail: "ไม่ผ่าน",
};

function recTone(rec: string): string {
  if (rec === "pass") return "var(--score-high)";
  if (rec === "hold") return "var(--score-mid)";
  if (rec === "fail") return "var(--score-low)";
  return "var(--muted-foreground)";
}

const COMPETENCIES: { key: keyof InterviewCompetencies; label: string }[] = [
  { key: "communication", label: "การสื่อสาร" },
  { key: "technical", label: "ความรู้/ทักษะงาน" },
  { key: "experience", label: "ประสบการณ์" },
  { key: "culture_fit", label: "ทัศนคติ/วัฒนธรรม" },
];

const emptyComp: InterviewCompetencies = { communication: 0, technical: 0, experience: 0, culture_fit: 0 };

function Label({ children }: { children: React.ReactNode }) {
  return <span className="text-xs font-medium text-foreground">{children}</span>;
}

function Textarea(props: React.ComponentProps<"textarea">) {
  const { className, ...rest } = props;
  return (
    <textarea
      className={cn(
        "min-h-16 w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30",
        className,
      )}
      {...rest}
    />
  );
}

export function InterviewFeedbackPanel({
  applicationId,
  status,
}: {
  applicationId: string;
  status: string;
}) {
  const { data: me } = useMe();
  const { data: list, isLoading } = useInterviewFeedback(applicationId);
  const add = useAddInterviewFeedback(applicationId);

  const canRecord = canRecordInterviewFeedback(me?.role);
  const stageOpen = status === "interview" || status === "interviewed";
  const showForm = canRecord && stageOpen;

  const [open, setOpen] = useState(false);
  const [rating, setRating] = useState("3");
  const [rec, setRec] = useState<InterviewRecommendation | "">("");
  const [comp, setComp] = useState<InterviewCompetencies>(emptyComp);
  const [strengths, setStrengths] = useState("");
  const [concerns, setConcerns] = useState("");
  const [notes, setNotes] = useState("");

  function reset() {
    setRating("3");
    setRec("");
    setComp(emptyComp);
    setStrengths("");
    setConcerns("");
    setNotes("");
    add.reset();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!rec) return;
    await add.mutateAsync(
      {
        overall_rating: Number(rating),
        recommendation: rec,
        competencies: comp,
        strengths: strengths.trim() || undefined,
        concerns: concerns.trim() || undefined,
        notes: notes.trim() || undefined,
      },
      {
        onSuccess: () => {
          toast.success("บันทึกผลสัมภาษณ์แล้ว");
          reset();
          setOpen(false);
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : "บันทึกไม่สำเร็จ"),
      },
    );
  }

  // Nothing to show and nothing the user can do → render nothing (keeps the panel
  // out of the way for non-interview stages / read-only roles with no entries).
  if (!showForm && (isLoading || !list || list.length === 0)) return null;

  return (
    <div className="mt-6 space-y-5 border-t border-hairline pt-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="eyebrow">Interview feedback</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">ผลการสัมภาษณ์</h2>
        </div>
        {showForm && !open && (
          <Button size="sm" variant="secondary" onClick={() => setOpen(true)}>
            + บันทึกผล
          </Button>
        )}
      </div>

      {showForm && open && (
        <form onSubmit={submit} className="space-y-3 rounded-lg bg-muted/40 p-4 ring-1 ring-hairline">
          <div className="grid grid-cols-2 gap-3">
            <label className="block space-y-1.5">
              <Label>ผลสรุป</Label>
              <Select value={rec} onValueChange={(v) => setRec((v as InterviewRecommendation) ?? "")}>
                <SelectTrigger className="w-full" aria-label="ผลสรุป">
                  <SelectValue placeholder="เลือกผล…" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="pass">ผ่าน</SelectItem>
                  <SelectItem value="hold">รอพิจารณา</SelectItem>
                  <SelectItem value="fail">ไม่ผ่าน</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label className="block space-y-1.5">
              <Label>คะแนนรวม</Label>
              <Select value={rating} onValueChange={(v) => setRating(v ?? "3")}>
                <SelectTrigger className="w-full" aria-label="คะแนนรวม">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {["1", "2", "3", "4", "5"].map((n) => (
                    <SelectItem key={n} value={n}>
                      {n} / 5
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
          </div>

          <div>
            <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
              คะแนนรายด้าน
            </p>
            <div className="grid grid-cols-2 gap-3">
              {COMPETENCIES.map((c) => (
                <label key={c.key} className="flex items-center justify-between gap-2">
                  <span className="text-xs text-foreground">{c.label}</span>
                  <Select
                    value={String(comp[c.key])}
                    onValueChange={(v) => setComp((prev) => ({ ...prev, [c.key]: Number(v ?? "0") }))}
                  >
                    <SelectTrigger className="w-20" aria-label={c.label}>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="0">–</SelectItem>
                      {["1", "2", "3", "4", "5"].map((n) => (
                        <SelectItem key={n} value={n}>
                          {n}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </label>
              ))}
            </div>
          </div>

          <label className="block space-y-1.5">
            <Label>จุดแข็ง</Label>
            <Textarea value={strengths} onChange={(e) => setStrengths(e.target.value)} placeholder="เช่น สื่อสารชัดเจน มีประสบการณ์ตรง" />
          </label>
          <label className="block space-y-1.5">
            <Label>จุดอ่อน / ข้อสังเกต</Label>
            <Textarea value={concerns} onChange={(e) => setConcerns(e.target.value)} placeholder="เช่น ยังขาดประสบการณ์ด้าน…" />
          </label>
          <label className="block space-y-1.5">
            <Label>บันทึกเพิ่มเติม</Label>
            <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="รายละเอียดอื่น ๆ" />
          </label>

          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                reset();
                setOpen(false);
              }}
            >
              ยกเลิก
            </Button>
            <Button type="submit" size="sm" disabled={!rec || add.isPending} className="gap-2">
              {add.isPending && <Loader2 className="size-4 animate-spin" />}
              บันทึก
            </Button>
          </div>
        </form>
      )}

      {list && list.length > 0 && (
        <ul className="space-y-3">
          {list.map((f) => (
            <FeedbackCard key={f.id} f={f} />
          ))}
        </ul>
      )}

      {list && list.length === 0 && !open && (
        <p className="text-sm text-muted-foreground">ยังไม่มีการบันทึกผลสัมภาษณ์</p>
      )}
    </div>
  );
}

function FeedbackCard({ f }: { f: InterviewFeedback }) {
  const rated = COMPETENCIES.filter((c) => f.competencies[c.key] > 0);
  return (
    <li className="rounded-lg bg-card p-4 ring-1 ring-hairline">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-foreground">{f.interviewer_name || "ผู้สัมภาษณ์"}</p>
          <p className="text-xs text-muted-foreground">{new Date(f.created_at).toLocaleString()}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Badge variant="secondary" className="tabular-nums">
            {f.overall_rating}/5
          </Badge>
          <span
            className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold text-white"
            style={{ backgroundColor: recTone(f.recommendation) }}
          >
            {REC_LABEL[f.recommendation] ?? f.recommendation}
          </span>
        </div>
      </div>

      {rated.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
          {rated.map((c) => (
            <span key={c.key}>
              {c.label} <span className="font-medium text-foreground tabular-nums">{f.competencies[c.key]}/5</span>
            </span>
          ))}
        </div>
      )}

      {f.strengths && (
        <p className="mt-3 text-sm text-foreground">
          <span className="font-medium">จุดแข็ง: </span>
          {f.strengths}
        </p>
      )}
      {f.concerns && (
        <p className="mt-1.5 text-sm text-foreground">
          <span className="font-medium">ข้อสังเกต: </span>
          {f.concerns}
        </p>
      )}
      {f.notes && <p className="mt-1.5 text-sm text-muted-foreground">{f.notes}</p>}
    </li>
  );
}
