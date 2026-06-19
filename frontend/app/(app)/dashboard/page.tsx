"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight, Inbox, AlertTriangle, ScanLine } from "lucide-react";

import { KpiCards } from "@/components/analytics/Charts";
import { WaitingByStore, OpenRoles } from "@/components/analytics/Operations";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useKpi, useOpenRoles, useWaitingByStore } from "@/lib/queries";

export default function DashboardPage() {
  const t = useTranslations("dashboard");
  const { data: kpi } = useKpi();
  const { data: byStore } = useWaitingByStore();
  const { data: openRoles } = useOpenRoles();

  return (
    <div className="settle space-y-8">
      {/* Page masthead — a calm title and one supporting line. One keyline
          under the eyebrow is the single accent. */}
      <header className="border-b border-hairline pb-7">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="max-w-prose">
            <p className="eyebrow brass-underline inline-block">{t("eyebrow")}</p>
            <h1 className="mt-4 font-heading text-4xl font-semibold leading-[1.02] tracking-tight sm:text-[2.75rem]">
              {t("title")}
            </h1>
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">{t("desc")}</p>
          </div>
          <Link
            href="/applications"
            className={buttonVariants({ size: "default", className: "h-10 gap-1.5 px-5 shadow-sm" })}
          >
            {t("openInbox")}
            <ArrowRight className="size-4" />
          </Link>
        </div>
      </header>

      {/* KPI band */}
      {kpi ? <KpiCards kpi={kpi} /> : <Skeleton className="h-36 w-full rounded-xl" />}

      {/* Operational breakdown — the dashboard's centerpiece: where to act,
          by store and by role, instead of an aggregate vanity funnel. */}
      <div className="grid gap-6 lg:grid-cols-2">
        {byStore ? <WaitingByStore data={byStore} /> : <Skeleton className="h-72 w-full rounded-xl" />}
        {openRoles ? <OpenRoles data={openRoles} /> : <Skeleton className="h-72 w-full rounded-xl" />}
      </div>

      {/* Quick actions + screening pass rate */}
      <div className="grid gap-6 lg:grid-cols-[1.6fr_1fr]">
          <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
            <header className="flex items-baseline justify-between">
              <div>
                <p className="eyebrow">{t("actionEyebrow")}</p>
                <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">{t("actionTitle")}</h2>
              </div>
              {kpi && (
                <span className="rounded-full bg-brass-soft px-2.5 py-1 text-[0.6875rem] font-semibold tabular-nums text-[color-mix(in_oklch,var(--brass)_70%,black)]">
                  {t("openCount", { count: kpi.waiting })}
                </span>
              )}
            </header>
            <ul className="mt-4 flex flex-col divide-y divide-hairline">
              <QuickAction
                href="/applications?status=scored"
                icon={<Inbox className="size-4" />}
                title={t("qaReviewTitle")}
                hint={kpi ? t("qaReviewHint", { count: kpi.waiting }) : "—"}
              />
              <QuickAction
                href="/applications?min_score=75"
                icon={<ScanLine className="size-4" />}
                title={t("qaBestFitTitle")}
                hint={t("qaBestFitHint")}
              />
              <QuickAction
                href="/applications"
                icon={<AlertTriangle className="size-4 text-brass" />}
                title={t("qaHumanTitle")}
                hint={t("qaHumanHint")}
              />
            </ul>
          </section>

          {kpi && (
            <section className="relative overflow-hidden rounded-xl bg-brand p-6 text-brand-foreground ring-1 ring-brand/15">
              {/* Brass left keyline — the single accent, matching the hero KPI */}
              <span
                aria-hidden
                className="absolute inset-y-6 left-0 w-[3px] rounded-full"
                style={{ background: "var(--brass)" }}
              />
              <p className="pl-3 text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-brand-foreground/70">
                {t("passRateLabel")}
              </p>
              {(() => {
                const rate = kpi.applied > 0 ? Math.round((kpi.passed / kpi.applied) * 100) : 0;
                return (
                  <div className="pl-3">
                    <div className="mt-3 flex items-end gap-3">
                      <span className="font-heading text-[2.75rem] font-semibold leading-none tracking-tight tabular-nums">
                        {rate}%
                      </span>
                      <span
                        aria-hidden
                        className="mb-1.5 h-1.5 flex-1 overflow-hidden rounded-full bg-brand-foreground/15"
                      >
                        <span
                          className="block h-full origin-left rounded-full bg-brass transition-transform duration-700"
                          style={{ transform: `scaleX(${rate / 100})`, transitionTimingFunction: "var(--ease-out)" }}
                        />
                      </span>
                    </div>
                    <p className="mt-3 text-sm text-brand-foreground/80">
                      {t.rich("passRateBody", {
                        onboarded: kpi.onboarded,
                        b: (chunks) => (
                          <span className="font-semibold tabular-nums text-brand-foreground">{chunks}</span>
                        ),
                      })}
                    </p>
                  </div>
                );
              })()}
            </section>
          )}
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
        <span className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-soft text-brand transition-colors group-hover:bg-brand group-hover:text-brand-foreground">
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
