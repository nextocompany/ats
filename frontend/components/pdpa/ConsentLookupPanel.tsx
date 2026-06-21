"use client";

// Consent-history lookup: a DPO enters a subject's account id (or candidate id)
// and sees their unified consent ledger - each given/withdrawn event with the
// notice version and source. Backs a data-subject access request on consent.
import { useState } from "react";
import { useTranslations } from "next-intl";

import { useConsentLookup } from "@/lib/queries";
import { formatDateTime } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";

export function ConsentLookupPanel() {
  const t = useTranslations("pdpa");
  const [accountId, setAccountId] = useState("");
  const [submitted, setSubmitted] = useState("");

  const { data, isLoading, isError, error } = useConsentLookup(
    { account_id: submitted },
    submitted !== "",
  );

  return (
    <section className="space-y-4 rounded-xl bg-card p-5 ring-1 ring-hairline" aria-labelledby="pdpa-consent-heading">
      <div>
        <h2 id="pdpa-consent-heading" className="text-sm font-semibold text-foreground">{t("consentTitle")}</h2>
        <p className="mt-1 text-xs text-muted-foreground">{t("consentHint")}</p>
      </div>

      <form
        className="flex flex-wrap items-center gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          setSubmitted(accountId.trim());
        }}
      >
        <Input
          value={accountId}
          onChange={(e) => setAccountId(e.target.value)}
          placeholder={t("consentAccountPlaceholder")}
          className="w-80 max-w-full"
          aria-label={t("consentAccountPlaceholder")}
        />
        <Button type="submit" disabled={accountId.trim() === ""}>
          {t("lookup")}
        </Button>
      </form>

      {submitted !== "" && (
        <div>
          {isLoading ? (
            <Skeleton className="h-24 w-full rounded-lg" />
          ) : isError ? (
            <p className="text-sm text-destructive">
              {error instanceof Error ? error.message : t("lookupFailed")}
            </p>
          ) : !data || data.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("consentEmpty")}</p>
          ) : (
            <ul className="divide-y divide-hairline rounded-lg ring-1 ring-hairline">
              {data.map((c) => (
                <li key={c.id} className="flex items-center justify-between gap-4 px-4 py-3 text-sm">
                  <div>
                    <span className="text-foreground">{t("consentVersionLabel", { version: c.version || "-" })}</span>
                    <span className="ml-2 text-xs text-muted-foreground">{c.source_channel || "-"}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    <Badge
                      variant="outline"
                      className={`rounded-full border-0 ${
                        c.consent_given ? "bg-brand-soft text-brand" : "bg-destructive/10 text-destructive"
                      }`}
                    >
                      {c.consent_given ? t("consentGiven") : t("consentWithdrawn")}
                    </Badge>
                    <span className="text-xs text-muted-foreground tabular-nums">{formatDateTime(c.created_at)}</span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
