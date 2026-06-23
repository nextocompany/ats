"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";

import { PortalShell } from "@/components/PortalShell";
import { ShareButtons } from "@/components/ShareButtons";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { levelLabel } from "@/lib/levels";
import { usePublicPosition } from "@/lib/queries";
import { cn } from "@/lib/utils";

export function JobDetailClient({ id }: { id: string }) {
  const t = useTranslations("jobs");
  const { data: position, isLoading, isError } = usePublicPosition(id);
  // The position catalog's role-generic Master JD is shown when present; when a
  // position has no JD text we fall back to the generic reassuring copy below
  // (no fabricated job-specific requirements).
  const hasJd = Boolean(
    position?.responsibilities?.trim() ||
      position?.qualifications?.trim() ||
      position?.benefits?.trim(),
  );
  const offer = t.raw("offer") as string[];
  const steps = t.raw("steps") as string[];

  return (
    <PortalShell backHref="/jobs">
      {isLoading ? (
        <div className="flex flex-col gap-5">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-12 w-2/3" />
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="mt-4 h-48 w-full" />
        </div>
      ) : null}

      {isError || (!isLoading && !position) ? (
        <div className="mx-auto flex max-w-md flex-col items-center gap-4 rounded-xl border border-line bg-card p-10 text-center">
          <p className="text-sm text-muted-foreground">{t("notFound")}</p>
          <Link href="/jobs" className={buttonVariants({ variant: "outline", size: "tap" })}>
            {t("backToAll")}
          </Link>
        </div>
      ) : null}

      {position ? (
        <article className="grid gap-12 lg:grid-cols-[1fr_320px] lg:gap-16">
          <div className="flex max-w-2xl flex-col gap-12">
            <header className="flex flex-col gap-4 border-b border-line pb-8">
              {position.level ? (
                <p className="text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground">
                  {levelLabel(position.level)}
                </p>
              ) : null}
              <h1 className="[font-size:var(--text-display)] font-bold leading-[1.1] text-foreground">
                {position.title_th}
              </h1>
              {position.title_en ? (
                <p className="[font-size:var(--text-lead)] text-muted-foreground">{position.title_en}</p>
              ) : null}
            </header>

            {hasJd ? (
              <>
                {position.responsibilities?.trim() ? (
                  <section className="flex flex-col gap-3">
                    <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">
                      {t("responsibilitiesHeading")}
                    </h2>
                    <p className="whitespace-pre-line leading-relaxed text-foreground/80">
                      {position.responsibilities}
                    </p>
                  </section>
                ) : null}
                {position.qualifications?.trim() ? (
                  <section className="flex flex-col gap-3">
                    <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">
                      {t("qualificationsHeading")}
                    </h2>
                    <p className="whitespace-pre-line leading-relaxed text-foreground/80">
                      {position.qualifications}
                    </p>
                  </section>
                ) : null}
                {position.benefits?.trim() ? (
                  <section className="flex flex-col gap-3">
                    <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">
                      {t("benefitsHeading")}
                    </h2>
                    <p className="whitespace-pre-line leading-relaxed text-foreground/80">
                      {position.benefits}
                    </p>
                  </section>
                ) : null}
              </>
            ) : (
              <section className="flex flex-col gap-3">
                <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">{t("aboutHeading")}</h2>
                <p className="leading-relaxed text-foreground/80">{t("about")}</p>
              </section>
            )}

            <section className="flex flex-col gap-4">
              <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">{t("offerHeading")}</h2>
              <ul className="divide-y divide-line border-y border-line">
                {offer.map((o) => (
                  <li key={o} className="flex items-start gap-3 py-4 text-foreground/85">
                    <span
                      aria-hidden="true"
                      className="mt-0.5 grid size-5 shrink-0 place-content-center rounded-full bg-primary/10 text-primary"
                    >
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none">
                        <path d="M5 13l4 4L19 7" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
                      </svg>
                    </span>
                    {o}
                  </li>
                ))}
              </ul>
            </section>

            <section className="flex flex-col gap-4">
              <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">{t("stepsHeading")}</h2>
              <ol className="flex flex-col gap-4">
                {steps.map((s, i) => (
                  <li key={s} className="flex items-start gap-3.5 text-foreground/85">
                    <span className="num grid size-7 shrink-0 place-content-center rounded-full border border-line bg-secondary text-sm font-semibold text-foreground">
                      {i + 1}
                    </span>
                    <span className="pt-0.5">{s}</span>
                  </li>
                ))}
              </ol>
            </section>
          </div>

          {/* Apply card — sticky on desktop, inline on mobile. */}
          <aside className="lg:col-start-2">
            <div className="flex flex-col gap-4 rounded-xl border border-line bg-card p-6 lg:sticky lg:top-24">
              <div className="flex flex-col gap-1">
                <p className="text-sm text-muted-foreground">{t("interested")}</p>
                <p className="text-lg font-semibold text-foreground">{t("applyToday")}</p>
              </div>
              <Link href={`/jobs/${position.id}/apply`} className={cn(buttonVariants({ size: "tap" }), "w-full")}>
                {t("applyCta")}
              </Link>
              <p className="text-center text-xs text-muted-foreground">{t("applyNote")}</p>
              <div className="border-t border-line pt-4">
                <ShareButtons title={position.title_th} />
              </div>
            </div>
          </aside>
        </article>
      ) : null}
    </PortalShell>
  );
}
