"use client";

// Interview scorecards split by perspective: the TA (recruiter) rates technical/
// communication/experience/attitude; the Line Manager (sgm) rates culture-fit/
// growth/leadership. Both feed a combined aggregate. Reads are open to anyone who
// can see the application; each form is server-gated to the relevant role.
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import type {
  InterviewCompetencies,
  InterviewFeedback,
  InterviewRecommendation,
  ScorecardPerspective,
} from "@/lib/types";
import {
  useAddInterviewFeedback,
  useInterviewFeedback,
  useMe,
  useScorecardSummary,
} from "@/lib/queries";
import { canRecordLmScorecard, canRecordTaScorecard } from "@/lib/roles";
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

type Comp = keyof InterviewCompetencies;
const TA_COMPS: { key: Comp; label: string }[] = [
  { key: "technical", label: "ความรู้/ทักษะงาน" },
  { key: "communication", label: "การสื่อสาร" },
  { key: "experience", label: "ประสบการณ์" },
  { key: "attitude", label: "ทัศนคติ" },
];
const LM_COMPS: { key: Comp; label: string }[] = [
  { key: "culture_fit", label: "วัฒนธรรมองค์กร" },
  { key: "growth_potential", label: "ศักยภาพการเติบโต" },
  { key: "leadership", label: "ภาวะผู้นำ" },
];
const COMP_LABEL: Record<string, string> = Object.fromEntries(
  [...TA_COMPS, ...LM_COMPS].map((c) => [c.key, c.label]),
);

const emptyComp: InterviewCompetencies = {
  communication: 0,
  technical: 0,
  experience: 0,
  attitude: 0,
  culture_fit: 0,
  growth_potential: 0,
  leadership: 0,
};

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

const STAGE_OPEN = (status: string) => status === "interview" || status === "interviewed";

// ─── Aggregate summary ──────────────────────────────────────────────────────
export function ScorecardSummary({ applicationId }: { applicationId: string }) {
  const { data } = useScorecardSummary(applicationId);
  if (!data || (!data.ta && !data.line_manager)) return null;
  return (
    <div className="mt-6 space-y-3 border-t border-hairline pt-6">
      <div className="flex items-baseline justify-between">
        <div>
          <p className="eyebrow">Scorecard</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">สรุปคะแนนสัมภาษณ์</h2>
        </div>
        {data.composite_score != null && (
          <span className="num text-2xl font-semibold tabular-nums text-brand">
            {data.composite_score}
            <span className="ml-1 text-xs font-normal text-muted-foreground">composite</span>
          </span>
        )}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <AggCard title="TA" agg={data.ta} />
        <AggCard title="Line Manager" agg={data.line_manager} />
      </div>
    </div>
  );
}

function AggCard({ title, agg }: { title: string; agg: import("@/lib/types").PerspectiveAgg | null }) {
  return (
    <div className="rounded-lg bg-muted/40 p-3 ring-1 ring-hairline">
      <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{title}</p>
      {agg ? (
        <>
          <p className="mt-1">
            <span className="num text-xl font-semibold tabular-nums text-foreground">{agg.avg_overall}</span>
            <span className="text-xs text-muted-foreground"> /5 · {agg.count} ราย</span>
          </p>
          <div className="mt-2 flex flex-wrap gap-x-3 gap-y-0.5 text-[0.6875rem] text-muted-foreground">
            {Object.entries(agg.avg_competencies).map(([k, v]) => (
              <span key={k}>
                {COMP_LABEL[k] ?? k} <span className="font-medium tabular-nums text-foreground">{v}</span>
              </span>
            ))}
          </div>
        </>
      ) : (
        <p className="mt-1 text-xs text-muted-foreground">ยังไม่มี</p>
      )}
    </div>
  );
}

// ─── Per-perspective scorecard (form + list) ────────────────────────────────
export function TaScorecard({ applicationId, status }: { applicationId: string; status: string }) {
  const { data: me } = useMe();
  return (
    <Scorecard
      applicationId={applicationId}
      status={status}
      perspective="ta"
      eyebrow="TA scorecard"
      title="สกอร์การ์ด — TA"
      comps={TA_COMPS}
      canRecord={canRecordTaScorecard(me?.role)}
    />
  );
}

export function LineManagerScorecard({ applicationId, status }: { applicationId: string; status: string }) {
  const { data: me } = useMe();
  return (
    <Scorecard
      applicationId={applicationId}
      status={status}
      perspective="line_manager"
      eyebrow="Line manager scorecard"
      title="สกอร์การ์ด — ผู้จัดการสาขา"
      comps={LM_COMPS}
      canRecord={canRecordLmScorecard(me?.role)}
    />
  );
}

function Scorecard({
  applicationId,
  status,
  perspective,
  eyebrow,
  title,
  comps,
  canRecord,
}: {
  applicationId: string;
  status: string;
  perspective: ScorecardPerspective;
  eyebrow: string;
  title: string;
  comps: { key: Comp; label: string }[];
  canRecord: boolean;
}) {
  const { data: list, isLoading } = useInterviewFeedback(applicationId);
  const add = useAddInterviewFeedback(applicationId);
  const mine = (list ?? []).filter((f) => f.perspective === perspective);
  const showForm = canRecord && STAGE_OPEN(status);

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
        perspective,
        overall_rating: Number(rating),
        recommendation: rec,
        competencies: comp,
        strengths: strengths.trim() || undefined,
        concerns: concerns.trim() || undefined,
        notes: notes.trim() || undefined,
      },
      {
        onSuccess: () => {
          toast.success("บันทึกสกอร์การ์ดแล้ว");
          reset();
          setOpen(false);
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : "บันทึกไม่สำเร็จ"),
      },
    );
  }

  // Nothing the user can do and nothing recorded for this perspective → hide.
  if (!showForm && (isLoading || mine.length === 0)) return null;

  return (
    <div className="mt-6 space-y-4 border-t border-hairline pt-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="eyebrow">{eyebrow}</p>
          <h2 className="mt-1 font-heading text-base font-semibold tracking-tight">{title}</h2>
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
              {comps.map((c) => (
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
            <Textarea value={strengths} onChange={(e) => setStrengths(e.target.value)} placeholder="เช่น สื่อสารชัดเจน" />
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

      {mine.length > 0 && (
        <ul className="space-y-3">
          {mine.map((f) => (
            <ScorecardCard key={f.id} f={f} comps={comps} />
          ))}
        </ul>
      )}
    </div>
  );
}

function ScorecardCard({ f, comps }: { f: InterviewFeedback; comps: { key: Comp; label: string }[] }) {
  const rated = comps.filter((c) => f.competencies[c.key] > 0);
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
