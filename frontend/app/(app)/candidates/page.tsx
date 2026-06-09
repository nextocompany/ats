"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { Pagination } from "@/app/(app)/applications/page";
import { InitialChip, SourceChip, StatusPill } from "@/components/people/PeopleBits";
import { PageHeader } from "@/components/shell/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { useCandidates } from "@/lib/queries";

const LIMIT = 20;

function CandidatesInner() {
  const params = useSearchParams();
  const router = useRouter();
  const page = Math.max(1, Number(params.get("page") ?? "1"));
  const { data, isLoading } = useCandidates(page);

  const items = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / LIMIT));

  return (
    <div className="settle space-y-6">
      <PageHeader
        eyebrow="Talent records"
        title="Candidates"
        meta={<span className="tabular-nums">{total} candidate{total === 1 ? "" : "s"} on file</span>}
      />

      <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[640px] text-sm">
            <thead className="ledger-head text-left">
              <tr>
                <th className="py-3 pl-5 pr-3">Candidate</th>
                <th className="px-3 py-3">Province</th>
                <th className="px-3 py-3">Source</th>
                <th className="w-32 py-3 pl-3 pr-5 text-right">Status</th>
              </tr>
            </thead>
            <tbody>
              {isLoading &&
                Array.from({ length: 8 }).map((_, i) => (
                  <tr key={i} className="border-b border-hairline last:border-0">
                    <td colSpan={4} className="px-5 py-3.5">
                      <div className="flex items-center gap-3">
                        <Skeleton className="size-9 shrink-0 rounded-lg" />
                        <Skeleton className="h-5 w-full" />
                      </div>
                    </td>
                  </tr>
                ))}
              {!isLoading && items.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-5 py-16 text-center">
                    <p className="text-sm font-medium text-foreground">No candidates yet</p>
                    <p className="mt-1 text-sm text-muted-foreground">
                      Records appear here as applications are parsed.
                    </p>
                  </td>
                </tr>
              )}
              {items.map((c) => (
                <tr key={c.id} className="ledger-row border-b border-hairline last:border-0">
                  <td className="py-3 pl-5 pr-3">
                    <Link
                      href={`/candidates/${c.id}`}
                      className="-my-1 flex items-center gap-3 rounded-md py-1 outline-none focus-visible:ring-2 focus-visible:ring-ring"
                    >
                      <InitialChip name={c.full_name} />
                      <span className="min-w-0">
                        <span className="block truncate font-semibold text-foreground">
                          {c.full_name}
                        </span>
                        <span className="block truncate font-mono text-[0.6875rem] uppercase tracking-wide text-muted-foreground">
                          {c.id.slice(0, 8)}
                          {/* Region is hidden at narrow widths so the mono id never clips */}
                          {c.subregion ? (
                            <span className="hidden sm:inline">{` · ${c.subregion}`}</span>
                          ) : null}
                        </span>
                      </span>
                    </Link>
                  </td>
                  <td className="px-3 py-3 text-foreground/80">{c.province || "—"}</td>
                  <td className="px-3 py-3">
                    <SourceChip channel={c.source_channel} />
                  </td>
                  <td className="py-3 pl-3 pr-5 text-right">
                    <StatusPill status={c.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination page={page} pages={pages} onPage={(p) => router.replace(`/candidates?page=${p}`)} />
    </div>
  );
}

export default function CandidatesPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <CandidatesInner />
    </Suspense>
  );
}
