"use client";

// PDPA / DPO console: compliance overview, the held DSAR request queue, and a
// subject consent-history lookup. Gated to pdpa.admin.
import { useState } from "react";
import { useTranslations } from "next-intl";
import { ShieldAlert } from "lucide-react";

import { PdpaOverviewCards } from "@/components/pdpa/PdpaOverviewCards";
import { DsarQueueTable } from "@/components/pdpa/DsarQueueTable";
import { ConsentLookupPanel } from "@/components/pdpa/ConsentLookupPanel";
import { PageHeader } from "@/components/shell/PageHeader";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useMe, usePdpaOverview, useDsarRequests } from "@/lib/queries";
import { canAdminPdpa } from "@/lib/roles";

const PAGE_SIZE = 20;
const STATUSES = ["pending", "completed", "rejected"] as const;
const ALL = "all";

export default function PdpaConsolePage() {
  const t = useTranslations("pdpa");
  const { data: me, isLoading: meLoading } = useMe();
  const allowed = canAdminPdpa(me);

  const [status, setStatus] = useState<string>("pending");
  const [page, setPage] = useState(1);

  const enabled = allowed && !meLoading;
  const { data: overview, isLoading: ovLoading } = usePdpaOverview(enabled);
  const { data: dsar, isLoading: dsarLoading } = useDsarRequests(
    { status: status === ALL ? undefined : status, page, limit: PAGE_SIZE },
    enabled,
  );

  const requests = dsar?.data ?? [];
  const total = dsar?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  if (meLoading) return <Skeleton className="h-40 w-full rounded-xl" />;

  if (!allowed) {
    return (
      <div className="settle space-y-8">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <section className="flex items-start gap-3 rounded-xl bg-card p-6 ring-1 ring-hairline">
          <ShieldAlert className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            {t.rich("restricted", { b: (chunks) => <span className="font-medium text-foreground">{chunks}</span> })}
          </p>
        </section>
      </div>
    );
  }

  return (
    <div className="settle space-y-8">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("meta")} />

      {ovLoading ? (
        <Skeleton className="h-48 w-full rounded-xl" />
      ) : overview ? (
        <PdpaOverviewCards overview={overview} />
      ) : null}

      <section className="space-y-4" aria-labelledby="pdpa-dsar-heading">
        <div className="flex items-center justify-between gap-3">
          <h2 id="pdpa-dsar-heading" className="text-sm font-semibold text-foreground">{t("dsarTitle")}</h2>
          <Select
            value={status}
            onValueChange={(v) => {
              setStatus(v ?? "pending");
              setPage(1);
            }}
          >
            <SelectTrigger className="w-44" aria-label={t("filterStatus")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>{t("filterAll")}</SelectItem>
              {STATUSES.map((s) => (
                <SelectItem key={s} value={s}>
                  {t(`dsarStatus_${s}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {dsarLoading ? (
          <Skeleton className="h-48 w-full rounded-xl" />
        ) : requests.length === 0 ? (
          <p className="rounded-xl bg-card px-5 py-12 text-center text-sm text-muted-foreground ring-1 ring-hairline">
            {t("dsarEmpty")}
          </p>
        ) : (
          <>
            <DsarQueueTable requests={requests} />
            {pages > 1 && <Pagination page={page} pages={pages} onPage={setPage} />}
          </>
        )}
      </section>

      <ConsentLookupPanel />
    </div>
  );
}
