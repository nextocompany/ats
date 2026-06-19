"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { Store, Briefcase, ArrowRight } from "lucide-react";

import type { OpenRole, StoreLoad } from "@/lib/types";

const fmt = new Intl.NumberFormat("en-US");

/* ──────────────────────────────────────────────────────────────────────────
   Operational breakdown panels for the Overview — answer "where do I act?"
   instead of showing aggregate vanity stats. Ranked horizontal bars in the
   shared ledger language: blue→brass tonal ramp, brass marks the top row.
   ────────────────────────────────────────────────────────────────────────── */

// Blue → brass ramp so the ranked rows read as a deliberate scale, not a stack
// of identical bars. Top row is the deepest CP Axtra blue; the tail warms.
function rampShade(index: number, count: number): string {
  if (count <= 1) return "oklch(46% 0.18 264)";
  const t = index / (count - 1);
  return `oklch(${46 + t * 26}% ${0.18 - t * 0.03} ${264 - t * 170})`;
}

interface PanelProps {
  eyebrow: string;
  title: string;
  icon: React.ReactNode;
  meta?: string;
  rows: RankRow[];
  emptyTitle: string;
  emptyHint: string;
}

interface RankRow {
  key: string;
  label: string;
  sub?: string;
  value: number;
  href?: string;
}

function RankPanel({ eyebrow, title, icon, meta, rows, emptyTitle, emptyHint }: PanelProps) {
  const max = Math.max(...rows.map((r) => r.value), 1);

  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">{eyebrow}</p>
          <h2 className="mt-3 flex items-center gap-2 font-heading text-lg font-semibold tracking-tight">
            <span aria-hidden className="text-muted-foreground">{icon}</span>
            {title}
          </h2>
        </div>
        {meta && <span className="text-xs tabular-nums text-muted-foreground">{meta}</span>}
      </header>

      {rows.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <span aria-hidden className="mb-4 grid size-11 place-items-center rounded-2xl bg-brand-soft text-brand">
            {icon}
          </span>
          <p className="text-sm font-semibold text-foreground">{emptyTitle}</p>
          <p className="mx-auto mt-1 max-w-xs text-xs text-muted-foreground">{emptyHint}</p>
          <span className="mx-auto mt-5 block h-px w-10 bg-hairline" aria-hidden />
        </div>
      ) : (
        <ol className="flex flex-col gap-3">
          {rows.map((r, i) => {
            const widthPct = Math.max((r.value / max) * 100, 6);
            const shade = rampShade(i, rows.length);
            const isTop = i === 0;
            const Row = (
              <>
                <div className="mb-1.5 flex items-baseline justify-between gap-2 text-sm">
                  <span className="flex min-w-0 items-baseline gap-2">
                    <span className="truncate font-medium text-foreground">{r.label}</span>
                    {r.sub && <span className="shrink-0 text-xs text-muted-foreground">{r.sub}</span>}
                  </span>
                  <span className="flex shrink-0 items-baseline gap-1.5 tabular-nums">
                    <span className={`font-semibold ${isTop ? "text-brass" : "text-foreground"}`}>
                      {fmt.format(r.value)}
                    </span>
                    {r.href && (
                      <ArrowRight className="size-3.5 text-muted-foreground/40 transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
                    )}
                  </span>
                </div>
                <div className="h-2.5 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    className="h-full rounded-full transition-[width] duration-500"
                    style={{
                      width: `${widthPct}%`,
                      backgroundColor: isTop ? "var(--brass)" : shade,
                      transitionTimingFunction: "var(--ease-out)",
                    }}
                  />
                </div>
              </>
            );
            return (
              <li key={r.key}>
                {r.href ? (
                  <Link
                    href={r.href}
                    className="group -mx-2 block rounded-md px-2 py-1 outline-none transition-colors hover:bg-muted/60 focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    {Row}
                  </Link>
                ) : (
                  <div className="-mx-2 px-2 py-1">{Row}</div>
                )}
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}

export function WaitingByStore({ data }: { data: StoreLoad[] }) {
  const t = useTranslations("analytics");
  const total = data.reduce((sum, d) => sum + d.waiting, 0);
  return (
    <RankPanel
      eyebrow={t("backlogEyebrow")}
      title={t("backlogTitle")}
      icon={<Store className="size-4" strokeWidth={1.75} />}
      meta={total > 0 ? t("backlogMeta", { count: fmt.format(total) }) : undefined}
      rows={data.map((d) => ({
        key: d.store_id !== null ? String(d.store_id) : d.store_name,
        label: d.store_name,
        value: d.waiting,
        // Drill into the review queue (the inbox's scored backlog).
        href: "/applications?status=scored",
      }))}
      emptyTitle={t("backlogEmptyTitle")}
      emptyHint={t("backlogEmptyHint")}
    />
  );
}

export function OpenRoles({ data }: { data: OpenRole[] }) {
  const t = useTranslations("analytics");
  const total = data.reduce((sum, d) => sum + d.openings, 0);
  return (
    <RankPanel
      eyebrow={t("rolesEyebrow")}
      title={t("rolesTitle")}
      icon={<Briefcase className="size-4" strokeWidth={1.75} />}
      meta={total > 0 ? t("rolesMeta", { count: fmt.format(total) }) : undefined}
      rows={data.map((d) => ({
        key: d.position_id,
        label: d.title,
        sub: t("rolesStores", { count: d.stores }),
        value: d.openings,
      }))}
      emptyTitle={t("rolesEmptyTitle")}
      emptyHint={t("rolesEmptyHint")}
    />
  );
}
