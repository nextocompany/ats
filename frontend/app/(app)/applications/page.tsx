"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useMemo, useState } from "react";
import { X, Flag, SlidersHorizontal, Inbox as InboxIcon } from "lucide-react";

import { BulkActionBar } from "@/components/bulk/BulkActionBar";
import { Pagination } from "@/components/ui/pagination";
import { ScoreBadge, ScoreRail, FitLabel } from "@/components/inbox/ScoreBadge";
import { InitialChip, Pill, StatusPill } from "@/components/people/PeopleBits";
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
import type { Application } from "@/lib/types";

const STATUSES = ["", "pending", "parsed", "scored", "ai_interview", "ai_interviewed", "shortlisted", "interview", "interviewed", "offer", "hired", "rejected"];
const LIMIT = 20;

// Friendly relative time so "Applied" reads as recency, not an ISO timestamp.
function appliedAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "—";
  const mins = Math.floor((Date.now() - then) / 60000);
  if (mins < 1) return "Just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  if (days < 7) return `${days}d ago`;
  if (days < 30) return `${Math.floor(days / 7)}w ago`;
  return new Date(iso).toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

// Where this application would be placed — store name first, else the central
// pool (no nearby branch yet — awaiting manual assignment), else the candidate's
// province. Never a bare numeric store id. centralPoolLabel is passed in so the
// copy is localized + conveys "holding pool, to be assigned".
function placement(a: Application, centralPoolLabel: string): string {
  if (a.store_name) return a.store_name;
  if (a.talent_pool) return centralPoolLabel;
  if (a.candidate_province) return a.candidate_province;
  return "—";
}

// The must-have screening gate, spoken plainly. "Gate / Pass / Fail" was
// engineering jargon; HR reads whether a candidate meets the role's musts.
function Requirements({ passed }: { passed: boolean | null }) {
  if (passed === null) return <span className="text-xs text-muted-foreground">Pending</span>;
  return passed ? (
    <Pill tone="pass">Meets requirements</Pill>
  ) : (
    <Pill tone="fail">Missing requirements</Pill>
  );
}

function InboxInner() {
  const t = useTranslations("inbox");
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
  if (minScore) activeFilters.push({ key: "min_score", label: `Fit ≥ ${minScore}` });

  return (
    <div className="settle space-y-6">
      <PageHeader
        eyebrow="Screening queue"
        title={t("title")}
        meta={
          <span className="tabular-nums">
            {total} candidate{total === 1 ? "" : "s"} · best fit first
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
              Min fit
              <Input
                type="number"
                min={0}
                max={100}
                defaultValue={minScore}
                placeholder="0–100"
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
          {error instanceof Error ? error.message : "Failed to load candidates. Try again in a moment."}
        </div>
      )}

      {/* Queue summary — plain-language reads of the visible page so the inbox
          presents as a screening surface even at one row. */}
      {!isError && (
        <SummaryStrip
          stats={[
            { label: "In queue", value: <span className="tabular-nums">{total}</span>, lead: true, accent: true },
            { label: "Meet requirements", value: <span className="tabular-nums">{queue.passed}</span>, hint: "on this page" },
            { label: "Need a closer look", value: <span className="tabular-nums">{queue.flagged}</span>, hint: "flagged for you" },
            {
              label: "Best fit",
              value: queue.top !== null ? <span className="tabular-nums">{queue.top}</span> : <span className="text-muted-foreground">—</span>,
              hint: "top score here",
            },
          ]}
        />
      )}

      {/* Mobile (<768px) — stacked candidate cards. Each leads with the person
          (avatar + name + role applied for), the fit on the right, then a
          status/requirements/placement line. No horizontal overflow. */}
      <ul className="space-y-2.5 md:hidden">
        {isLoading &&
          Array.from({ length: 6 }).map((_, i) => (
            <li key={i} className="rounded-xl bg-card p-4 ring-1 ring-hairline">
              <Skeleton className="h-5 w-full" />
            </li>
          ))}
        {!isLoading && items.length === 0 && <EmptyState filtered={activeFilters.length > 0} onClear={() => router.replace("/applications")} />}
        {items.map((a) => {
          const name = a.candidate_name?.trim() || "Unnamed candidate";
          return (
            <li
              key={a.id}
              className="rounded-xl bg-card ring-1 ring-hairline data-[sel=true]:bg-brand-soft/55 data-[sel=true]:ring-brand/30"
              data-sel={selected.includes(a.id)}
            >
              <div className="flex items-start gap-3 p-4">
                <span className="flex items-center pt-1">
                  <Checkbox
                    checked={selected.includes(a.id)}
                    aria-label={`Select ${name}`}
                    onCheckedChange={(c) =>
                      setSelected((s) => (c ? [...s, a.id] : s.filter((x) => x !== a.id)))
                    }
                  />
                </span>
                <InitialChip name={name} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <Link
                        href={`/applications/${a.id}`}
                        className="block truncate text-sm font-semibold text-foreground underline-offset-2 hover:text-brand hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
                      >
                        {name}
                      </Link>
                      <p className="truncate text-xs text-muted-foreground">{a.position_title || "Role not set"}</p>
                    </div>
                    <span className="flex shrink-0 flex-col items-end gap-1">
                      <ScoreBadge score={a.ai_score} />
                      <FitLabel score={a.ai_score} />
                    </span>
                  </div>
                  <div className="mt-2.5 flex flex-wrap items-center gap-2 border-t border-hairline pt-2.5">
                    <StatusPill status={a.status} />
                    <Requirements passed={a.must_have_passed} />
                    {a.needs_manual_review && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-brass-soft px-1.5 py-0.5 text-[10px] font-medium text-brass">
                        <Flag className="size-2.5" /> review
                      </span>
                    )}
                    <span className="ml-auto truncate text-xs text-muted-foreground">
                      {placement(a, t("centralPool"))} · {appliedAgo(a.created_at)}
                    </span>
                  </div>
                </div>
              </div>
            </li>
          );
        })}
      </ul>

      <div className="hidden overflow-hidden rounded-xl bg-card ring-1 ring-hairline md:block">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[820px] text-sm">
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
                <th className="w-36 px-3 py-3">Fit</th>
                <th className="px-3 py-3">Candidate</th>
                <th className="w-40 px-3 py-3">Placement</th>
                <th className="w-24 px-3 py-3">Applied</th>
                <th className="w-28 px-3 py-3">Status</th>
                <th className="w-36 py-3 pl-3 pr-5">Requirements</th>
              </tr>
            </thead>
            <tbody>
              {isLoading &&
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-hairline last:border-0">
                    <td className="px-5 py-3.5" colSpan={7}>
                      <Skeleton className="h-5 w-full" />
                    </td>
                  </tr>
                ))}
              {!isLoading && items.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-5 py-20 text-center">
                    <EmptyStateBody filtered={activeFilters.length > 0} onClear={() => router.replace("/applications")} />
                  </td>
                </tr>
              )}
              {items.map((a) => {
                const name = a.candidate_name?.trim() || "Unnamed candidate";
                return (
                  <tr
                    key={a.id}
                    className="ledger-row group border-b border-hairline last:border-0 data-[sel=true]:bg-brand-soft/55"
                    data-sel={selected.includes(a.id)}
                  >
                    <td className="py-3.5 pl-5 pr-0">
                      <span className="flex items-center">
                        <Checkbox
                          checked={selected.includes(a.id)}
                          aria-label={`Select ${name}`}
                          onCheckedChange={(c) =>
                            setSelected((s) => (c ? [...s, a.id] : s.filter((x) => x !== a.id)))
                          }
                        />
                      </span>
                    </td>
                    <td className="px-3 py-3.5">
                      <div className="flex items-center gap-2">
                        <ScoreBadge score={a.ai_score} />
                        <FitLabel score={a.ai_score} />
                      </div>
                      <ScoreRail score={a.ai_score} />
                    </td>
                    <td className="px-3 py-3.5">
                      <div className="flex items-center gap-3">
                        <InitialChip name={name} size="sm" />
                        <div className="min-w-0">
                          <span className="flex items-center gap-2">
                            <Link
                              href={`/applications/${a.id}`}
                              className="truncate font-medium text-foreground underline-offset-2 hover:text-brand hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
                            >
                              {name}
                            </Link>
                            {a.needs_manual_review && (
                              <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-brass-soft px-1.5 py-0.5 text-[10px] font-medium text-brass">
                                <Flag className="size-2.5" /> review
                              </span>
                            )}
                          </span>
                          <p className="truncate text-xs text-muted-foreground">{a.position_title || "Role not set"}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-3.5 text-muted-foreground">
                      <span className="block max-w-[9rem] truncate" title={placement(a, t("centralPool"))}>
                        {placement(a, t("centralPool"))}
                      </span>
                    </td>
                    <td className="px-3 py-3.5 text-muted-foreground" title={new Date(a.created_at).toLocaleString()}>
                      {appliedAgo(a.created_at)}
                    </td>
                    <td className="px-3 py-3.5">
                      <StatusPill status={a.status} />
                    </td>
                    <td className="py-3.5 pl-3 pr-5">
                      <Requirements passed={a.must_have_passed} />
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination page={page} pages={pages} onPage={(p) => setParam("page", String(p))} />

      <BulkActionBar selected={selected} onDone={() => setSelected([])} />
    </div>
  );
}

// Empty-state body shared by the mobile list item and the desktop table cell.
function EmptyStateBody({ filtered, onClear }: { filtered: boolean; onClear: () => void }) {
  return (
    <>
      <span
        aria-hidden
        className="mx-auto mb-5 grid size-12 place-items-center rounded-2xl bg-brand-soft text-brand"
      >
        <InboxIcon className="size-6" strokeWidth={1.75} />
      </span>
      <p className="text-base font-semibold text-foreground">
        {filtered ? "No candidates match these filters" : "The queue is clear"}
      </p>
      <p className="mx-auto mt-1.5 max-w-sm text-sm text-muted-foreground">
        {filtered
          ? "Try widening the status or lowering the minimum fit to see more candidates."
          : "Newly screened candidates land here, ranked by AI fit. You're all caught up."}
      </p>
      {filtered && (
        <Button variant="outline" size="sm" className="mt-5" onClick={onClear}>
          Clear filters
        </Button>
      )}
      <span className="mx-auto mt-6 block h-px w-10 bg-hairline" aria-hidden />
    </>
  );
}

function EmptyState({ filtered, onClear }: { filtered: boolean; onClear: () => void }) {
  return (
    <li className="rounded-xl bg-card px-5 py-16 text-center ring-1 ring-hairline">
      <EmptyStateBody filtered={filtered} onClear={onClear} />
    </li>
  );
}

export default function InboxPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <InboxInner />
    </Suspense>
  );
}
