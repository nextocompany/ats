"use client";

// Member offers (Module-3 3.6): a logged-in candidate views their offer(s) and
// accepts or declines. Session-gated client-side, mirroring app/account/page.tsx.
import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { OfferCard, formatThaiDate } from "@/components/offers/OfferCard";
import { PortalShell } from "@/components/PortalShell";
import { useMyLetters, useMyOffers } from "@/lib/queries";
import { useCandidate } from "@/lib/session";

export default function OffersPage() {
  const t = useTranslations("offers");
  const router = useRouter();
  const { isAuthenticated, isLoading } = useCandidate();
  const { data: offers, isError } = useMyOffers();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.replace("/login?return=/offers");
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !isAuthenticated) {
    return (
      <PortalShell backHref="/account" narrow>
        <p className="text-center text-sm text-muted-foreground">{t("loading")}</p>
      </PortalShell>
    );
  }

  return (
    <PortalShell backHref="/account" narrow>
      <div className="flex flex-col gap-6">
        <header className="border-b border-line pb-5">
          <h1 className="[font-size:var(--text-h2)] font-semibold leading-tight text-foreground">{t("title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        </header>

        {isError ? (
          <p role="alert" className="rounded-xl border border-line bg-card p-6 text-center text-sm text-destructive">
            {t("loadFailed")}
          </p>
        ) : !offers ? (
          <p className="text-center text-sm text-muted-foreground">{t("loading")}</p>
        ) : offers.length === 0 ? (
          <p className="rounded-xl border border-line bg-card p-6 text-center text-sm text-muted-foreground">
            {t("empty")}
          </p>
        ) : (
          <ul className="flex flex-col gap-4">
            {offers.map((o) => (
              <li key={o.id}>
                <OfferCard offer={o} t={t} />
              </li>
            ))}
          </ul>
        )}

        <DocumentsSection t={t} formatDate={formatThaiDate} />
      </div>
    </PortalShell>
  );
}

function DocumentsSection({
  t,
  formatDate,
}: {
  t: ReturnType<typeof useTranslations>;
  formatDate: (iso: string | null) => string;
}) {
  const { data: letters, isError } = useMyLetters();
  if (isError) {
    return (
      <section className="border-t border-line pt-6">
        <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">{t("documentsTitle")}</h2>
        <p role="alert" className="mt-3 text-sm text-destructive">{t("documentsFailed")}</p>
      </section>
    );
  }
  if (!letters || letters.length === 0) return null;
  return (
    <section className="border-t border-line pt-6">
      <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">{t("documentsTitle")}</h2>
      <ul className="mt-3 flex flex-col gap-2">
        {letters.map((l) => (
          <li key={l.id} className="flex items-center justify-between gap-3 rounded-lg border border-line bg-card px-4 py-3">
            <span className="text-sm text-foreground">
              {l.type === "interview" ? t("docInterview") : t("docOffer")}
              <span className="ml-2 text-xs text-muted-foreground">{formatDate(l.created_at)}</span>
            </span>
            {l.url && (
              <a href={l.url} target="_blank" rel="noopener noreferrer" className="text-sm font-medium text-primary underline-offset-2 hover:underline">
                {t("download")}
              </a>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

