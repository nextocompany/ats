"use client";

// Internal reference: the published privacy notice (the same document candidates
// see) + the DPO contact. Visible to any authenticated HR user so they can answer
// data-subject questions; DPO admins also get a link into the PDPA console.
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";

import { PageHeader } from "@/components/shell/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { useMe, usePrivacyPolicy, usePublicDpo } from "@/lib/queries";
import { canAdminPdpa } from "@/lib/roles";

function fmtDate(iso: string, locale: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return new Intl.DateTimeFormat(locale === "en" ? "en-GB" : "th-TH", {
    year: "numeric",
    month: "long",
    day: "numeric",
  }).format(d);
}

export default function PrivacyPage() {
  const t = useTranslations("privacy");
  const locale = useLocale();
  const { data: me } = useMe();
  const { data: doc, isLoading: docLoading } = usePrivacyPolicy(locale);
  const { data: dpo } = usePublicDpo();

  const paragraphs = (doc?.body ?? "").split(/\n\s*\n/).filter(Boolean);
  const placeholder = (v: string | null | undefined) => (!v || v.trim() === "" ? t("notSet") : v);
  const officers = dpo?.officers ?? [];

  return (
    <div className="settle space-y-8">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("meta")} />

      <section aria-labelledby="notice-heading" className="space-y-4 rounded-xl bg-card p-6 ring-1 ring-hairline">
        {docLoading ? (
          <Skeleton className="h-48 w-full rounded-lg" />
        ) : doc ? (
          <>
            <div>
              <h2 id="notice-heading" className="text-lg font-semibold text-foreground">
                {doc.title}
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">
                {t("version", { version: doc.version })}
                <span aria-hidden="true" className="mx-2">
                  &middot;
                </span>
                {t("lastUpdated", { date: fmtDate(doc.effective_at, locale) })}
              </p>
            </div>
            <div className="flex flex-col gap-3 text-sm leading-relaxed text-foreground/90">
              {paragraphs.map((p, i) => (
                <p key={i}>{p}</p>
              ))}
            </div>
          </>
        ) : (
          <p className="text-sm text-muted-foreground">{t("noticeUnavailable")}</p>
        )}
      </section>

      <section
        aria-labelledby="dpo-heading"
        className="space-y-3 rounded-xl bg-card p-6 ring-1 ring-hairline"
      >
        <h2 id="dpo-heading" className="text-lg font-semibold text-foreground">
          {t("dpoTitle")}
        </h2>
        {officers.length > 0 ? (
          <div className="space-y-3">
            <p className="text-sm">
              <span className="text-muted-foreground">{t("dpoCompany")}: </span>
              <span className="text-foreground">{placeholder(dpo?.company)}</span>
            </p>
            <ul className="space-y-3">
              {officers.map((o, i) => (
                <li key={i} className="rounded-lg border border-hairline p-3">
                  <dl className="flex flex-col gap-1 text-sm">
                    <div className="flex gap-2">
                      <dt className="w-24 shrink-0 text-muted-foreground">{t("dpoName")}</dt>
                      <dd className="text-foreground">{placeholder(o.name)}</dd>
                    </div>
                    <div className="flex gap-2">
                      <dt className="w-24 shrink-0 text-muted-foreground">{t("dpoEmail")}</dt>
                      <dd className="text-foreground">{placeholder(o.email)}</dd>
                    </div>
                    <div className="flex gap-2">
                      <dt className="w-24 shrink-0 text-muted-foreground">{t("dpoPhone")}</dt>
                      <dd className="text-foreground">{placeholder(o.phone)}</dd>
                    </div>
                  </dl>
                </li>
              ))}
            </ul>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">{t("dpoUnset")}</p>
        )}
      </section>

      {canAdminPdpa(me) && (
        <p className="text-sm text-muted-foreground">
          {t.rich("consoleLink", {
            a: (chunks) => (
              <Link
                href="/pdpa"
                className="font-medium text-foreground underline underline-offset-4 transition-colors hover:text-muted-foreground"
              >
                {chunks}
              </Link>
            ),
          })}
        </p>
      )}
    </div>
  );
}
