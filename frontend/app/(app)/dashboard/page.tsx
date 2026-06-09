"use client";

import Link from "next/link";
import { ArrowRight, Inbox, AlertTriangle, ScanLine } from "lucide-react";

import { FunnelChart, KpiCards } from "@/components/analytics/Charts";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useFunnel, useKpi } from "@/lib/queries";

export default function DashboardPage() {
  const { data: kpi } = useKpi();
  const { data: funnel } = useFunnel();

  return (
    <div className="settle space-y-8">
      {/* Page masthead */}
      <header className="flex flex-wrap items-end justify-between gap-4 border-b border-hairline pb-6">
        <div>
          <p className="eyebrow">Command center</p>
          <h1 className="mt-1.5 font-heading text-3xl font-semibold tracking-tight">Overview</h1>
          <p className="mt-1.5 max-w-prose text-sm text-muted-foreground">
            Live read of the national recruitment pipeline — intake, AI screening, and onboarding.
          </p>
        </div>
        <Link
          href="/applications"
          className={buttonVariants({ size: "default", className: "h-9 gap-1.5 px-4" })}
        >
          Open ranked inbox
          <ArrowRight className="size-4" />
        </Link>
      </header>

      {/* KPI band */}
      {kpi ? <KpiCards kpi={kpi} /> : <Skeleton className="h-36 w-full rounded-xl" />}

      {/* Funnel + operator quick-actions, asymmetric Swiss grid */}
      <div className="grid gap-6 lg:grid-cols-[1.6fr_1fr]">
        {funnel ? <FunnelChart funnel={funnel} /> : <Skeleton className="h-80 w-full rounded-xl" />}

        <aside className="flex flex-col gap-6">
          <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
            <p className="eyebrow">Operator focus</p>
            <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">Where to act</h2>
            <ul className="mt-4 flex flex-col divide-y divide-hairline">
              <QuickAction
                href="/applications?status=scored"
                icon={<Inbox className="size-4" />}
                title="Review scored applications"
                hint={kpi ? `${kpi.waiting} awaiting an operator` : "—"}
              />
              <QuickAction
                href="/applications?min_score=75"
                icon={<ScanLine className="size-4" />}
                title="Top AI matches"
                hint="Score ≥ 75 — fast-track candidates"
              />
              <QuickAction
                href="/applications"
                icon={<AlertTriangle className="size-4 text-brass" />}
                title="Flagged for manual review"
                hint="OCR / dedup edge cases"
              />
            </ul>
          </section>

          {kpi && (
            <section className="rounded-xl bg-brand-soft p-6 ring-1 ring-brand/15">
              <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-brand">
                Pass-through
              </p>
              {(() => {
                const rate = kpi.applied > 0 ? Math.round((kpi.passed / kpi.applied) * 100) : 0;
                return (
                  <>
                    <div className="mt-3 flex items-end gap-3">
                      <span className="font-heading text-4xl font-semibold leading-none tracking-tight tabular-nums text-foreground">
                        {rate}%
                      </span>
                      {/* Tiny inline gauge — breaks the three-uniform-rows rhythm */}
                      <span
                        aria-hidden
                        className="mb-1 h-1.5 flex-1 overflow-hidden rounded-full bg-brand/15"
                      >
                        <span
                          className="block h-full rounded-full bg-brand transition-[width] duration-500"
                          style={{ width: `${rate}%`, transitionTimingFunction: "var(--ease-out)" }}
                        />
                      </span>
                    </div>
                    <p className="mt-3 text-sm text-foreground/80">
                      of applicants clear the AI gate. Onboarding holds{" "}
                      <span className="font-semibold tabular-nums text-foreground">{kpi.onboarded}</span> this cycle.
                    </p>
                  </>
                );
              })()}
            </section>
          )}
        </aside>
      </div>
    </div>
  );
}

function QuickAction({
  href,
  icon,
  title,
  hint,
}: {
  href: string;
  icon: React.ReactNode;
  title: string;
  hint: string;
}) {
  return (
    <li>
      <Link
        href={href}
        className="group -mx-2 flex items-center gap-3 rounded-md px-2 py-3 transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span className="grid size-9 shrink-0 place-items-center rounded-md bg-brand-soft text-brand">
          {icon}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-medium text-foreground">{title}</span>
          <span className="block text-xs text-muted-foreground">{hint}</span>
        </span>
        <ArrowRight className="size-4 shrink-0 text-muted-foreground/50 transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
      </Link>
    </li>
  );
}
