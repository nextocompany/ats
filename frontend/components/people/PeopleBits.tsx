import { Store, Globe, Megaphone, UserPlus, Building2, type LucideIcon } from "lucide-react";

/* ──────────────────────────────────────────────────────────────────────────
   Shared people-record presentation primitives — used by Candidates, Search,
   and detail surfaces so every list of humans reads in one design language:
   an initial chip, a labeled source chip, and a semantic status pill.
   ────────────────────────────────────────────────────────────────────────── */

// Deterministic warm-ink/emerald tint per name so the roster reads as a set of
// distinct people, not seven identical gray rows. Tints stay within the system
// palette (no rainbow) — emerald, brass, ink, clay washes only.
const CHIP_TINTS = [
  { bg: "var(--brand-soft)", fg: "var(--brand)" },
  { bg: "var(--brass-soft)", fg: "color-mix(in oklch, var(--brass) 78%, black)" },
  { bg: "oklch(94% 0.012 75)", fg: "oklch(34% 0.012 75)" },
  { bg: "oklch(94% 0.03 200)", fg: "oklch(40% 0.06 220)" },
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

export function InitialChip({ name }: { name: string }) {
  const tint = tintFor(name);
  return (
    <span
      aria-hidden
      className="grid size-9 shrink-0 place-items-center rounded-lg text-[0.8125rem] font-semibold tracking-tight"
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

// Semantic status pill — emerald for "available/active" states, clay for
// rejected/dropped, neutral ink otherwise. Status drives color, not decoration.
const POSITIVE = new Set(["available", "active", "hired", "shortlisted", "interview", "onboarded"]);
const NEGATIVE = new Set(["rejected", "dropped", "withdrawn", "inactive"]);

export function StatusPill({ status }: { status: string }) {
  const key = (status ?? "").toLowerCase();
  const tone = POSITIVE.has(key) ? "positive" : NEGATIVE.has(key) ? "negative" : "neutral";
  const cls =
    tone === "positive"
      ? "bg-brand-soft text-brand"
      : tone === "negative"
        ? "bg-[oklch(95%_0.04_27)] text-[var(--destructive)]"
        : "bg-secondary text-secondary-foreground";
  const label = status ? status[0].toUpperCase() + status.slice(1) : "—";
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ${cls}`}>
      {tone === "positive" && (
        <span aria-hidden className="size-1.5 rounded-full bg-current opacity-80" />
      )}
      {label}
    </span>
  );
}
