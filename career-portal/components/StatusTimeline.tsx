import type { ApplicationTimeline, TimelineMilestone } from "@/lib/types";
import { formatThaiDate } from "@/lib/applicationStatus";

// Branch milestone keys that carry a non-neutral tone (mirrors apptimeline).
const KEY_NOT_SELECTED = "not_selected";
const KEY_ACTION_NEEDED = "action_needed";

type Tone = "done" | "active" | "upcoming" | "ended" | "attention";

function toneFor(m: TimelineMilestone): Tone {
  if (m.state === "done") return "done";
  if (m.state === "upcoming") return "upcoming";
  // current:
  if (m.key === KEY_NOT_SELECTED) return "ended";
  if (m.key === KEY_ACTION_NEEDED) return "attention";
  return "active";
}

const NODE_CLASS: Record<Tone, string> = {
  done: "border-primary bg-primary text-primary-foreground",
  active: "border-primary bg-card text-primary ring-4 ring-primary/15",
  upcoming: "border-line bg-card text-muted-foreground/50",
  ended: "border-destructive bg-destructive/10 text-destructive",
  attention: "border-amber-500 bg-amber-50 text-amber-600",
};

const LABEL_CLASS: Record<Tone, string> = {
  done: "text-foreground",
  active: "font-semibold text-foreground",
  upcoming: "text-muted-foreground/60",
  ended: "font-semibold text-destructive",
  attention: "font-semibold text-amber-700",
};

export function StatusTimeline({ timeline }: { timeline: ApplicationTimeline }) {
  return (
    <section
      aria-label="ความคืบหน้าใบสมัคร"
      className="space-y-6 rounded-2xl border border-line bg-card p-6 shadow-sm"
    >
      <header className="space-y-1">
        <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
          ความคืบหน้าใบสมัคร
        </p>
        {timeline.position ? (
          <h2 className="[font-size:var(--text-h3)] font-semibold leading-tight text-foreground">
            {timeline.position}
          </h2>
        ) : null}
      </header>

      <ol className="relative">
        {timeline.milestones.map((m, i) => {
          const tone = toneFor(m);
          const isLast = i === timeline.milestones.length - 1;
          return (
            <li
              key={m.key}
              aria-current={m.state === "current" ? "step" : undefined}
              className="relative flex gap-4 pb-7 last:pb-0"
            >
              {!isLast ? (
                <span
                  aria-hidden
                  className={`absolute left-[11px] top-7 h-full w-px ${
                    tone === "done" ? "bg-primary/40" : "bg-line"
                  }`}
                />
              ) : null}

              <span
                aria-hidden
                className={`relative z-10 mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 text-[11px] font-bold ${NODE_CLASS[tone]}`}
              >
                {tone === "done" ? "✓" : tone === "ended" ? "×" : tone === "attention" ? "!" : ""}
              </span>

              <div className="min-w-0 flex-1">
                <p className={`text-sm leading-snug ${LABEL_CLASS[tone]}`}>{m.label}</p>
                {/* The current/branch step shows its actionable detail (e.g. the
                    re-apply instruction); completed steps show their date. */}
                {m.state === "current" ? (
                  <p className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{m.detail}</p>
                ) : m.state === "done" && m.reached_at ? (
                  <p className="mt-0.5 text-xs text-muted-foreground">{formatThaiDate(m.reached_at)}</p>
                ) : null}
              </div>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
