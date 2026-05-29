"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";

import { BulkActionBar } from "@/components/bulk/BulkActionBar";
import { ScoreBadge } from "@/components/inbox/ScoreBadge";
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
    <div className="space-y-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Ranked Inbox</h1>
          <p className="text-sm text-muted-foreground">{total} application(s) · sorted by AI score</p>
        </div>
        <div className="flex items-end gap-2">
          <label className="flex flex-col gap-1 text-xs text-muted-foreground">
            Status
            <Select value={status || "all"} onValueChange={(v) => setParam("status", v && v !== "all" ? v : "")}>
              <SelectTrigger className="w-40" size="sm">
                <SelectValue placeholder="All" />
              </SelectTrigger>
              <SelectContent>
                {STATUSES.map((s) => (
                  <SelectItem key={s || "all"} value={s || "all"}>
                    {s || "All"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>
          <label className="flex flex-col gap-1 text-xs text-muted-foreground">
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
        </div>
      </div>

      {isError && (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive">
          {error instanceof Error ? error.message : "Failed to load"}
        </div>
      )}

      <div className="overflow-hidden rounded-lg border">
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/40 text-left text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="w-10 px-3 py-2">
                <Checkbox
                  checked={allChecked}
                  aria-label="Select all"
                  onCheckedChange={(c) => setSelected(c ? items.map((i) => i.id) : [])}
                />
              </th>
              <th className="w-16 px-3 py-2">Score</th>
              <th className="px-3 py-2">Application</th>
              <th className="w-28 px-3 py-2">Status</th>
              <th className="w-24 px-3 py-2">Store</th>
              <th className="w-28 px-3 py-2">Gate</th>
            </tr>
          </thead>
          <tbody>
            {isLoading &&
              Array.from({ length: 8 }).map((_, i) => (
                <tr key={i} className="border-b">
                  <td className="px-3 py-2" colSpan={6}>
                    <Skeleton className="h-5 w-full" />
                  </td>
                </tr>
              ))}
            {!isLoading && items.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-10 text-center text-muted-foreground">
                  No applications match these filters.
                </td>
              </tr>
            )}
            {items.map((a) => (
              <tr key={a.id} className="border-b transition-colors hover:bg-muted/40">
                <td className="px-3 py-2">
                  <Checkbox
                    checked={selected.includes(a.id)}
                    aria-label={`Select ${a.id}`}
                    onCheckedChange={(c) =>
                      setSelected((s) => (c ? [...s, a.id] : s.filter((x) => x !== a.id)))
                    }
                  />
                </td>
                <td className="px-3 py-2">
                  <ScoreBadge score={a.ai_score} />
                </td>
                <td className="px-3 py-2">
                  <Link href={`/applications/${a.id}`} className="font-medium text-foreground hover:underline">
                    {a.id.slice(0, 8)}
                  </Link>
                  {a.needs_manual_review && (
                    <Badge variant="outline" className="ml-2 text-[10px]">
                      review
                    </Badge>
                  )}
                </td>
                <td className="px-3 py-2">
                  <Badge variant="secondary">{a.status}</Badge>
                </td>
                <td className="px-3 py-2 tabular-nums">{a.assigned_store_id ?? (a.talent_pool ? "pool" : "—")}</td>
                <td className="px-3 py-2">
                  {a.must_have_passed === null ? "—" : a.must_have_passed ? "✓ pass" : "✕ fail"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-sm">
        <span className="text-muted-foreground">
          Page {page} of {pages}
        </span>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setParam("page", String(page - 1))}>
            Previous
          </Button>
          <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => setParam("page", String(page + 1))}>
            Next
          </Button>
        </div>
      </div>

      <BulkActionBar selected={selected} onDone={() => setSelected([])} />
    </div>
  );
}

export default function InboxPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <InboxInner />
    </Suspense>
  );
}
