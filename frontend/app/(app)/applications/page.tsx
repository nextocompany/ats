"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { Check, X, ChevronLeft, ChevronRight, Flag } from "lucide-react";

import { BulkActionBar } from "@/components/bulk/BulkActionBar";
import { ScoreBadge, ScoreRail } from "@/components/inbox/ScoreBadge";
import { PageHeader } from "@/components/shell/PageHeader";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { useApplications } from "@/lib/queries";

const STATUSES = ["", "pending", "parsed", "scored", "shortlisted", "interview", "hired", "rejected"];
const LIMIT = 20;

function InboxInner() {
  const params = useSearchParams();
  const router = useRouter();
  const [selected, setSelected] = useState<string[]>([]);

  const status = params.get("status") ?? "";
  const minScore = params.get("min_score") ?? "";
  const page = Math.max(1, Number(params.get("page") ?? "1"));

  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params.toString());
    if (value) next.set(key, value);
    else next.delete(key);
    if (key !== "page") next.delete("page");
    router.replace(`/applications?${next.toString()}`);
    setSelected([]);
  };

  const { data, isLoading, isError, error } = useApplications({
    status: status || undefined,
    min_score: minScore ? Number(minScore) : undefined,
    page,
    limit: LIMIT,
  });

  const items = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / LIMIT));
  const allChecked = items.length > 0 && selected.length === items.length;

  return (
    <div className="settle space-y-6">
      <PageHeader
        eyebrow="Screening queue"
        title="Ranked Inbox"
        meta={
          <span className="tabular-nums">
            {total} application{total === 1 ? "" : "s"} · sorted by AI score
          </span>
        }
        actions={
          <>
            <label className="flex flex-col gap-1 text-[0.6875rem] font-medium uppercase tracking-wide text-muted-foreground">
              Status
              <Select value={status || "all"} onValueChange={(v) => setParam("status", v && v !== "all" ? v : "")}>
                <SelectTrigger className="w-40" size="sm">
                  <SelectValue placeholder="All" />
                </SelectTrigger>
                <SelectContent>
                  {STATUSES.map((s) => (
                    <SelectItem key={s || "all"} value={s || "all"}>
                      {s ? s[0].toUpperCase() + s.slice(1) : "All statuses"}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <label className="flex flex-col gap-1 text-[0.6875rem] font-medium uppercase tracking-wide text-muted-foreground">
              Min score
              <Input
                type="number"
                min={0}
                max={100}
                defaultValue={minScore}
                className="w-28"
                onBlur={(e) => setParam("min_score", e.target.value)}
              />
            </label>
          </>
        }
      />

      {isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {error instanceof Error ? error.message : "Failed to load applications. Try again in a moment."}
        </div>
      )}

      <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead className="ledger-head sticky top-0 z-10 text-left">
              <tr>
                <th className="w-10 py-3 pl-5 pr-0">
                  <span className="flex items-center">
                    <Checkbox
                      checked={allChecked}
                      aria-label="Select all"
                      onCheckedChange={(c) => setSelected(c ? items.map((i) => i.id) : [])}
                    />
                  </span>
                </th>
                <th className="w-16 px-3 py-3">Score</th>
                <th className="px-3 py-3">Application</th>
                <th className="w-28 px-3 py-3">Status</th>
                <th className="w-24 px-3 py-3">Store</th>
                <th className="w-24 py-3 pl-3 pr-5">Gate</th>
              </tr>
            </thead>
            <tbody>
              {isLoading &&
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-hairline last:border-0">
                    <td className="px-5 py-3.5" colSpan={6}>
                      <Skeleton className="h-5 w-full" />
                    </td>
                  </tr>
                ))}
              {!isLoading && items.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-5 py-16 text-center">
                    <p className="text-sm font-medium text-foreground">Nothing in the queue</p>
                    <p className="mt-1 text-sm text-muted-foreground">
                      No applications match these filters. Try widening the status or lowering the score.
                    </p>
                  </td>
                </tr>
              )}
              {items.map((a) => (
                <tr
                  key={a.id}
                  className="ledger-row group border-b border-hairline last:border-0 data-[sel=true]:bg-brand-soft/55"
                  data-sel={selected.includes(a.id)}
                >
                  <td className="py-3.5 pl-5 pr-0">
                    <span className="flex items-center">
                      <Checkbox
                        checked={selected.includes(a.id)}
                        aria-label={`Select ${a.id}`}
                        onCheckedChange={(c) =>
                          setSelected((s) => (c ? [...s, a.id] : s.filter((x) => x !== a.id)))
                        }
                      />
                    </span>
                  </td>
                  <td className="px-3 py-3.5">
                    <span className="inline-flex flex-col items-start">
                      <ScoreBadge score={a.ai_score} />
                      <ScoreRail score={a.ai_score} />
                    </span>
                  </td>
                  <td className="px-3 py-3.5">
                    <Link
                      href={`/applications/${a.id}`}
                      className="font-mono text-[0.8125rem] font-medium text-foreground underline-offset-2 hover:text-brand hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
                    >
                      {a.id.slice(0, 8)}
                    </Link>
                    {a.needs_manual_review && (
                      <span className="ml-2 inline-flex items-center gap-1 rounded-full bg-brass-soft px-1.5 py-0.5 align-middle text-[10px] font-medium text-brass">
                        <Flag className="size-2.5" /> review
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-3.5">
                    <Badge variant="secondary" className="capitalize">{a.status}</Badge>
                  </td>
                  <td className="px-3 py-3.5 tabular-nums text-muted-foreground">
                    {a.assigned_store_id ?? (a.talent_pool ? "pool" : "—")}
                  </td>
                  <td className="py-3.5 pl-3 pr-5">
                    {a.must_have_passed === null ? (
                      <span className="text-muted-foreground">—</span>
                    ) : a.must_have_passed ? (
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-brand">
                        <Check className="size-3.5" /> Pass
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-xs font-medium text-destructive">
                        <X className="size-3.5" /> Fail
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination page={page} pages={pages} onPage={(p) => setParam("page", String(p))} />

      <BulkActionBar selected={selected} onDone={() => setSelected([])} />
    </div>
  );
}

export function Pagination({
  page,
  pages,
  onPage,
}: {
  page: number;
  pages: number;
  onPage: (p: number) => void;
}) {
  return (
    <div className="flex items-center justify-between text-sm">
      <span className="text-muted-foreground tabular-nums">
        Page {page} <span className="text-muted-foreground/50">of</span> {pages}
      </span>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => onPage(page - 1)}>
          <ChevronLeft className="size-4" /> Previous
        </Button>
        <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => onPage(page + 1)}>
          Next <ChevronRight className="size-4" />
        </Button>
      </div>
    </div>
  );
}

export default function InboxPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <InboxInner />
    </Suspense>
  );
}
