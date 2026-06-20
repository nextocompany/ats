"use client";

// Requisition management — open/approve/close manual position openings, RBAC-scoped.
import { useState } from "react";
import { useTranslations } from "next-intl";
import { Plus, ShieldAlert } from "lucide-react";

import { RequisitionDialog } from "@/components/requisitions/RequisitionDialog";
import { RequisitionTable } from "@/components/requisitions/RequisitionTable";
import { PageHeader } from "@/components/shell/PageHeader";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useMe, useRequisitions } from "@/lib/queries";
import { canManageRequisitions } from "@/lib/roles";
import type { Requisition } from "@/lib/types";

const PAGE_SIZE = 20;
const STATUSES = ["pending_approval", "open", "closed", "cancelled"] as const;
const ALL = "all";

export default function RequisitionsPage() {
  const t = useTranslations("requisitions");
  const { data: me, isLoading: meLoading } = useMe();
  const allowed = canManageRequisitions(me);

  const [status, setStatus] = useState<string>(ALL);
  const [page, setPage] = useState(1);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Requisition | null>(null);

  const { data, isLoading } = useRequisitions(
    { status: status === ALL ? undefined : status, page, limit: PAGE_SIZE },
    allowed && !meLoading,
  );
  const items = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
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
    <div className="settle space-y-6">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        meta={t("meta")}
        actions={
          <div className="flex items-center gap-3">
            <Select
              value={status}
              onValueChange={(v) => {
                setStatus(v ?? ALL);
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
                    {t(`status_${s}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button onClick={() => setCreateOpen(true)} className="gap-2">
              <Plus className="size-4" />
              {t("newReq")}
            </Button>
          </div>
        }
      />

      {isLoading ? (
        <Skeleton className="h-64 w-full rounded-xl" />
      ) : items.length === 0 ? (
        <p className="rounded-xl bg-card px-5 py-12 text-center text-sm text-muted-foreground ring-1 ring-hairline">
          {t("empty")}
        </p>
      ) : (
        <>
          <RequisitionTable requisitions={items} me={me} onEdit={setEditing} />
          {pages > 1 && <Pagination page={page} pages={pages} onPage={setPage} />}
        </>
      )}

      <RequisitionDialog
        key={createOpen ? "create-open" : "create-closed"}
        mode="create"
        open={createOpen}
        onClose={() => setCreateOpen(false)}
      />
      <RequisitionDialog
        key={editing?.id ?? "none"}
        mode="edit"
        open={!!editing}
        requisition={editing}
        onClose={() => setEditing(null)}
      />
    </div>
  );
}
