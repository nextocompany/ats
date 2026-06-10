import { Store, Globe, Megaphone, UserPlus, Building2, type LucideIcon } from "lucide-react";

/* ──────────────────────────────────────────────────────────────────────────
   Shared people-record presentation primitives — used by Candidates, Search,
   and detail surfaces so every list of humans reads in one design language:
   an initial chip, a labeled source chip, and a semantic status pill.
   ────────────────────────────────────────────────────────────────────────── */

// Deterministic tint per name so the roster reads as a set of distinct people,
// not seven identical gray rows. Tints are drawn from the CP Axtra dot palette —
// blue, yellow, coral, teal washes — a friendly multicolour spread, never harsh.
const CHIP_TINTS = [
  { bg: "var(--brand-soft)", fg: "var(--brand)" },
  { bg: "var(--brass-soft)", fg: "color-mix(in oklch, var(--brass) 70%, black)" },
  { bg: "oklch(95% 0.04 18)", fg: "oklch(48% 0.16 22)" },
  { bg: "oklch(95% 0.04 200)", fg: "oklch(44% 0.1 215)" },
] as const;

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "—";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

// Stable index from the name string — same person, same tint across renders.
function tintFor(seed: string): (typeof CHIP_TINTS)[number] {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  return CHIP_TINTS[h % CHIP_TINTS.length];
}

export function InitialChip({ name, size = "md" }: { name: string; size?: "sm" | "md" | "lg" }) {
  const tint = tintFor(name);
  const dim = size === "lg" ? "size-11 text-sm" : size === "sm" ? "size-8 text-xs" : "size-9 text-[0.8125rem]";
  return (
    <span
      aria-hidden
      className={`grid shrink-0 place-items-center rounded-lg font-semibold tracking-tight ${dim}`}
      style={{ backgroundColor: tint.bg, color: tint.fg }}
    >
      {initials(name)}
    </span>
  );
}

const SOURCE_ICONS: Record<string, LucideIcon> = {
  walk_in: Store,
  walkin: Store,
  store: Store,
  website: Globe,
  online: Globe,
  web: Globe,
  campaign: Megaphone,
  ad: Megaphone,
  referral: UserPlus,
  agency: Building2,
};

function prettyChannel(raw: string): string {
  return raw
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((w) => w[0].toUpperCase() + w.slice(1))
    .join(" ");
}

// Source rendered as a small labeled chip with a channel glyph — never plain text.
export function SourceChip({ channel }: { channel: string }) {
  if (!channel) {
    return <span className="text-sm text-muted-foreground">—</span>;
  }
  const key = channel.toLowerCase().replace(/\s+/g, "_");
  const Icon = SOURCE_ICONS[key] ?? Building2;
  return (
    <span className="inline-flex items-center gap-1.5 rounded-md border border-hairline bg-secondary/60 px-2 py-1 text-xs font-medium text-secondary-foreground">
      <Icon className="size-3.5 text-muted-foreground" strokeWidth={1.75} />
      {prettyChannel(channel)}
    </span>
  );
}

/* ──────────────────────────────────────────────────────────────────────────
   Semantic pill system — ONE token-driven vocabulary shared everywhere a
   status/outcome renders (Candidates, Inbox gate, application detail).
     pass / positive  → CP Axtra blue   (strong, on-brand)
     fail / negative  → warm clay-red   (genuine signal, AA on tint)
     pending / brass  → brass-tinted    (awaiting an operator)
     neutral          → quiet ink       (default, unremarkable states)
   A leading dot reinforces the tone without relying on color alone.
   ────────────────────────────────────────────────────────────────────────── */

export type PillTone = "pass" | "fail" | "pending" | "neutral";

const PILL_STYLE: Record<PillTone, { cls: string; dot: string }> = {
  pass: { cls: "bg-brand-soft text-brand", dot: "var(--brand)" },
  fail: {
    cls: "bg-[oklch(95%_0.045_27)] text-[oklch(48%_0.18_27)]",
    dot: "oklch(55% 0.2 27)",
  },
  pending: {
    cls: "bg-brass-soft text-[color-mix(in_oklch,var(--brass)_72%,black)]",
    dot: "var(--brass)",
  },
  neutral: { cls: "bg-secondary text-secondary-foreground", dot: "currentColor" },
};

export function Pill({
  tone,
  children,
  showDot = true,
}: {
  tone: PillTone;
  children: React.ReactNode;
  showDot?: boolean;
}) {
  const s = PILL_STYLE[tone];
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${s.cls}`}
    >
      {showDot && (
        <span
          aria-hidden
          className="size-1.5 shrink-0 rounded-full"
          style={{ background: s.dot, opacity: 0.85 }}
        />
      )}
      {children}
    </span>
  );
}

// Map a free-text candidate/application status onto a semantic tone.
const POSITIVE = new Set(["available", "active", "hired", "shortlisted", "interview", "onboarded", "pass", "passed"]);
const NEGATIVE = new Set(["rejected", "dropped", "withdrawn", "inactive", "failed", "fail"]);
const PENDING = new Set(["pending", "scored", "parsed", "review", "waiting", "in_review"]);

export function toneForStatus(status: string): PillTone {
  const key = (status ?? "").toLowerCase();
  if (POSITIVE.has(key)) return "pass";
  if (NEGATIVE.has(key)) return "fail";
  if (PENDING.has(key)) return "pending";
  return "neutral";
}

export function StatusPill({ status }: { status: string }) {
  const tone = toneForStatus(status);
  const label = status ? status[0].toUpperCase() + status.slice(1) : "—";
  return <Pill tone={tone}>{label}</Pill>;
}
