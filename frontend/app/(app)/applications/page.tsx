"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useMemo, useState } from "react";
import { X, ChevronLeft, ChevronRight, Flag, SlidersHorizontal, Inbox as InboxIcon } from "lucide-react";

import { BulkActionBar } from "@/components/bulk/BulkActionBar";
import { ScoreBadge, ScoreRail } from "@/components/inbox/ScoreBadge";
import { Pill, StatusPill } from "@/components/people/PeopleBits";
import { PageHeader } from "@/components/shell/PageHeader";
import { SummaryStrip } from "@/components/shell/SummaryStrip";
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

  // Page-level read of the visible queue — drives the summary strip so a
  // one-row table still presents as a designed screening surface.
  const queue = useMemo(() => {
    const passed = items.filter((a) => a.must_have_passed === true).length;
    const flagged = items.filter((a) => a.needs_manual_review).length;
    const scores = items.map((a) => a.ai_score).filter((s): s is number => typeof s === "number");
    const top = scores.length ? Math.round(Math.max(...scores)) : null;
    return { passed, flagged, top };
  }, [items]);

  const activeFilters: { key: string; label: string }[] = [];
  if (status) activeFilters.push({ key: "status", label: `Status · ${status[0].toUpperCase() + status.slice(1)}` });
  if (minScore) activeFilters.push({ key: "min_score", label: `Score ≥ ${minScore}` });

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

      {/* Active filters — reflect URL state as removable chips, with a count read */}
      {activeFilters.length > 0 && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="inline-flex items-center gap-1.5 text-[0.6875rem] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
            <SlidersHorizontal className="size-3.5" /> Filtering
          </span>
          {activeFilters.map((f) => (
            <button
              key={f.key}
              type="button"
              onClick={() => setParam(f.key, "")}
              className="group inline-flex items-center gap-1.5 rounded-full bg-brand-soft px-3 py-1 text-xs font-medium text-brand transition-colors hover:bg-brand hover:text-brand-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {f.label}
              <X className="size-3 opacity-60 transition-opacity group-hover:opacity-100" />
            </button>
          ))}
          <button
            type="button"
            onClick={() => router.replace("/applications")}
            className="text-xs font-medium text-muted-foreground underline-offset-2 transition-colors hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm px-1"
          >
            Clear all
          </button>
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {error instanceof Error ? error.message : "Failed to load applications. Try again in a moment."}
        </div>
      )}

      {/* Queue summary — the ranked inbox reads as a screening surface, not a
          bare table, even when only one application matches the filters. */}
      {!isError && (
        <SummaryStrip
          stats={[
            { label: "In queue", value: <span className="tabular-nums">{total}</span>, lead: true, accent: true },
            { label: "Passed AI gate", value: <span className="tabular-nums">{queue.passed}</span>, hint: "on this page" },
            { label: "Flagged for review", value: <span className="tabular-nums">{queue.flagged}</span>, hint: "needs an operator" },
            {
              label: "Top match",
              value: queue.top !== null ? <span className="tabular-nums">{queue.top}</span> : <span className="text-muted-foreground">—</span>,
              hint: "best AI score here",
            },
          ]}
        />
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
                  <td colSpan={6} className="px-5 py-20 text-center">
                    <span
                      aria-hidden
                      className="mx-auto mb-5 grid size-12 place-items-center rounded-2xl bg-brand-soft text-brand"
                    >
                      <InboxIcon className="size-6" strokeWidth={1.75} />
                    </span>
                    <p className="text-base font-semibold text-foreground">
                      {activeFilters.length > 0 ? "No applications match these filters" : "The queue is clear"}
                    </p>
                    <p className="mx-auto mt-1.5 max-w-sm text-sm text-muted-foreground">
                      {activeFilters.length > 0
                        ? "Try widening the status or lowering the minimum score to see more candidates."
                        : "Newly scored applications land here, ranked by AI fit. You're all caught up."}
                    </p>
                    {activeFilters.length > 0 && (
                      <Button
                        variant="outline"
                        size="sm"
                        className="mt-5"
                        onClick={() => router.replace("/applications")}
                      >
                        Clear filters
                      </Button>
                    )}
                    <span className="dot-rule mx-auto mt-6 opacity-70" aria-hidden />
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
                    <StatusPill status={a.status} />
                  </td>
                  <td className="px-3 py-3.5 tabular-nums text-muted-foreground">
                    {a.assigned_store_id ?? (a.talent_pool ? "pool" : "—")}
                  </td>
                  <td className="py-3.5 pl-3 pr-5">
                    {a.must_have_passed === null ? (
                      <span className="text-xs text-muted-foreground">—</span>
                    ) : a.must_have_passed ? (
                      <Pill tone="pass">Pass</Pill>
                    ) : (
                      <Pill tone="fail">Fail</Pill>
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
