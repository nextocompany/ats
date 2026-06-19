"use client";

// Offer management (Module-3 3.6). HR (hr_manager/super_admin) composes an offer
// for an offer-stage application, edits it while draft, and sends it — after which
// it is read-only and the candidate responds from the career-portal. Reads are open
// to anyone who can see the application; the form is server-gated to offer roles.
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { Application, Offer, OfferStatus } from "@/lib/types";
import { useCreateOffer, useMe, useOffer, useSendOffer, useUpdateOffer } from "@/lib/queries";
import { canManageOffer } from "@/lib/roles";
import { Button } from "@/components/ui/button";

// Typed status → i18n-key map: adding an OfferStatus without a label is a compile
// error (no unsound `as` cast).
const STATUS_KEY: Record<OfferStatus, "status_draft" | "status_sent" | "status_accepted" | "status_declined" | "status_expired"> = {
  draft: "status_draft",
  sent: "status_sent",
  accepted: "status_accepted",
  declined: "status_declined",
  expired: "status_expired",
};

interface Props {
  applicationId: string;
  app: Application;
}

// Dates are stored UTC-midnight. Read the calendar-day portion directly (no UTC
// re-conversion) so a non-UTC viewer never sees an off-by-one date.
function toDateInput(iso: string | null): string {
  return iso ? iso.slice(0, 10) : "";
}
function fromDateInput(d: string): string | null {
  return d ? new Date(`${d}T00:00:00Z`).toISOString() : null;
}
function fmtDate(iso: string | null): string {
  return iso ? new Date(iso).toLocaleDateString(undefined, { timeZone: "UTC" }) : "—";
}

export function OfferPanel({ applicationId, app }: Props) {
  const t = useTranslations("offer");
  const { data: me } = useMe();
  const { data: offer, isLoading } = useOffer(applicationId);
  const canManage = canManageOffer(me);

  if (isLoading) return null;
  if (!offer && !(app.status === "offer" && canManage)) return null;

  const isDraft = !offer || offer.status === "draft";

  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <p className="eyebrow">{t("title")}</p>
      {isDraft && canManage ? (
        // key remounts the form (re-seeding its fields) when the offer identity
        // changes — e.g. after the draft is first created — without a sync effect.
        <OfferForm key={offer?.id ?? "new"} applicationId={applicationId} offer={offer ?? null} t={t} />
      ) : (
        <OfferSummary offer={offer!} t={t} />
      )}
    </section>
  );
}

function OfferForm({
  applicationId,
  offer,
  t,
}: {
  applicationId: string;
  offer: Offer | null;
  t: ReturnType<typeof useTranslations>;
}) {
  const create = useCreateOffer(applicationId);
  const update = useUpdateOffer(applicationId);
  const send = useSendOffer(applicationId);

  const [salary, setSalary] = useState(offer?.salary != null ? String(offer.salary) : "");
  const [startDate, setStartDate] = useState(toDateInput(offer?.start_date ?? null));
  const [terms, setTerms] = useState(offer?.terms ?? "");
  const [expiresAt, setExpiresAt] = useState(toDateInput(offer?.expires_at ?? null));

  const payload = () => ({
    salary: salary ? Number(salary) : null,
    start_date: fromDateInput(startDate),
    terms: terms.trim(),
    expires_at: fromDateInput(expiresAt),
  });

  const saving = create.isPending || update.isPending;
  const canSend = !!offer && Number(salary) > 0 && !!startDate && !saving && !send.isPending;

  function save(e: React.FormEvent) {
    e.preventDefault();
    const mut = offer ? update : create;
    mut.mutate(payload(), {
      onSuccess: () => toast.success(t("saved")),
      onError: (err) => toast.error(err instanceof Error ? err.message : t("saveFailed")),
    });
  }

  function doSend() {
    send.mutate(undefined, {
      onSuccess: () => toast.success(t("sent")),
      onError: (err) => toast.error(err instanceof Error ? err.message : t("sendFailed")),
    });
  }

  return (
    <form onSubmit={save} className="mt-3 space-y-3" noValidate>
      <label className="block space-y-1.5">
        <span className="text-xs font-medium text-foreground">{t("salary")}</span>
        <input
          type="number"
          min={0}
          step={1}
          value={salary}
          onChange={(e) => setSalary(e.target.value)}
          className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm tabular-nums outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          placeholder={t("salaryPlaceholder")}
        />
      </label>
      <label className="block space-y-1.5">
        <span className="text-xs font-medium text-foreground">{t("startDate")}</span>
        <input
          type="date"
          value={startDate}
          onChange={(e) => setStartDate(e.target.value)}
          className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        />
      </label>
      <label className="block space-y-1.5">
        <span className="text-xs font-medium text-foreground">{t("expiresAt")}</span>
        <input
          type="date"
          value={expiresAt}
          onChange={(e) => setExpiresAt(e.target.value)}
          className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        />
      </label>
      <label className="block space-y-1.5">
        <span className="text-xs font-medium text-foreground">{t("terms")}</span>
        <textarea
          value={terms}
          onChange={(e) => setTerms(e.target.value)}
          rows={3}
          className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
          placeholder={t("termsPlaceholder")}
        />
      </label>
      <div className="flex justify-end gap-2">
        <Button type="submit" size="sm" variant="secondary" className="gap-2" disabled={saving}>
          {saving && <Loader2 className="size-4 animate-spin" />}
          {offer ? t("save") : t("create")}
        </Button>
        <Button type="button" size="sm" variant="default" className="gap-2" disabled={!canSend} onClick={doSend}>
          {send.isPending && <Loader2 className="size-4 animate-spin" />}
          {t("send")}
        </Button>
      </div>
      {!offer && <p className="text-xs text-muted-foreground">{t("createHint")}</p>}
    </form>
  );
}

function OfferSummary({ offer, t }: { offer: Offer; t: ReturnType<typeof useTranslations> }) {
  const statusLabel = t(STATUS_KEY[offer.status] ?? "status_sent");
  return (
    <div className="mt-3 space-y-3 text-sm">
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground">{t("statusLabel")}</span>
        <span className="font-semibold text-foreground">{statusLabel}</span>
      </div>
      <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-xs">
        <dt className="text-muted-foreground">{t("salary")}</dt>
        <dd className="text-right font-medium tabular-nums">
          {offer.salary != null ? offer.salary.toLocaleString("th-TH", { maximumFractionDigits: 0 }) : "—"}
        </dd>
        <dt className="text-muted-foreground">{t("startDate")}</dt>
        <dd className="text-right tabular-nums">{fmtDate(offer.start_date)}</dd>
        {offer.expires_at && (
          <>
            <dt className="text-muted-foreground">{t("expiresAt")}</dt>
            <dd className="text-right tabular-nums">{fmtDate(offer.expires_at)}</dd>
          </>
        )}
      </dl>
      {offer.terms && <p className="rounded-lg bg-muted/40 px-3 py-2 text-xs text-foreground">{offer.terms}</p>}
      {offer.status === "declined" && offer.decline_reason && (
        <div className="rounded-lg bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <p className="font-medium">{t("declinedTitle")}</p>
          <p className="mt-0.5 text-destructive/90">{offer.decline_reason}</p>
        </div>
      )}
    </div>
  );
}
