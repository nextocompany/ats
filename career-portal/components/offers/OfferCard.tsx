"use client";

// Shared offer card used by the offers list (app/offers) and the per-offer
// deep-link page (app/offers/[id]). Owns the accept/decline interaction.
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { useRespondOffer } from "@/lib/queries";
import type { Offer, OfferStatus } from "@/lib/types";

// Typed status → i18n-key map (no unsound `as` cast; exhaustive over OfferStatus).
export const STATUS_KEY: Record<OfferStatus, "status_draft" | "status_sent" | "status_accepted" | "status_declined" | "status_expired"> = {
  draft: "status_draft",
  sent: "status_sent",
  accepted: "status_accepted",
  declined: "status_declined",
  expired: "status_expired",
};

// Dates are stored UTC-midnight; render in UTC so a non-UTC viewer never sees an
// off-by-one calendar day.
export function formatThaiDate(iso: string | null): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return new Intl.DateTimeFormat("th-TH", { dateStyle: "long", timeZone: "UTC" }).format(d);
}

export function OfferCard({ offer, t }: { offer: Offer; t: ReturnType<typeof useTranslations> }) {
  const respond = useRespondOffer(offer.id);
  const [declining, setDeclining] = useState(false);
  const [reason, setReason] = useState("");

  const respondable = offer.status === "sent";

  function accept() {
    respond.mutate({ decision: "accept" });
  }
  function decline(e: React.FormEvent) {
    e.preventDefault();
    if (!reason.trim()) return;
    respond.mutate({ decision: "decline", reason: reason.trim() });
  }

  return (
    <div className="space-y-4 rounded-xl border border-line bg-card p-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-medium text-foreground">{offer.position_title || t("position")}</p>
          <p className="text-xs text-muted-foreground">{t(STATUS_KEY[offer.status] ?? "status_sent")}</p>
        </div>
        {offer.salary != null && (
          <p className="text-right text-sm font-semibold tabular-nums text-foreground">
            {offer.salary.toLocaleString("th-TH", { maximumFractionDigits: 0 })} {t("baht")}
          </p>
        )}
      </div>

      <dl className="space-y-2 border-t border-line pt-4 text-sm">
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">{t("startDate")}</dt>
          <dd className="text-right font-medium">{formatThaiDate(offer.start_date)}</dd>
        </div>
        {offer.expires_at && (
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">{t("expires")}</dt>
            <dd className="text-right font-medium">{formatThaiDate(offer.expires_at)}</dd>
          </div>
        )}
      </dl>

      {offer.terms && <p className="rounded-lg bg-secondary px-3 py-2 text-sm text-foreground/80">{offer.terms}</p>}

      {offer.status === "accepted" && <p className="text-sm font-medium text-primary">{t("acceptedNote")}</p>}
      {offer.status === "declined" && <p className="text-sm text-muted-foreground">{t("declinedNote")}</p>}
      {offer.status === "expired" && <p className="text-sm text-destructive">{t("expiredNote")}</p>}

      {respond.isError && (
        <p role="alert" className="text-sm font-medium text-destructive">
          {respond.error instanceof Error ? respond.error.message : t("respondFailed")}
        </p>
      )}

      {respondable && !declining && (
        <div className="flex gap-3">
          <Button onClick={accept} disabled={respond.isPending} className="gap-2">
            {respond.isPending && <Loader2 className="size-4 animate-spin" />}
            {t("accept")}
          </Button>
          <Button variant="outline" onClick={() => setDeclining(true)} disabled={respond.isPending}>
            {t("decline")}
          </Button>
        </div>
      )}

      {respondable && declining && (
        <form onSubmit={decline} className="space-y-3" noValidate>
          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-foreground">{t("declineReasonLabel")}</span>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={3}
              required
              className="w-full rounded-lg border border-line bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-primary"
              placeholder={t("declineReasonPlaceholder")}
            />
          </label>
          <div className="flex gap-3">
            <Button type="button" variant="ghost" onClick={() => setDeclining(false)}>
              {t("cancel")}
            </Button>
            <Button type="submit" variant="outline" className="gap-2" disabled={!reason.trim() || respond.isPending}>
              {respond.isPending && <Loader2 className="size-4 animate-spin" />}
              {t("confirmDecline")}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
