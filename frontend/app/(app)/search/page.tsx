"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { ScoreBadge } from "@/components/inbox/ScoreBadge";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useCandidateSearch } from "@/lib/queries";

const LIMIT = 20;
const DEBOUNCE_MS = 300;

function SearchInner() {
  const params = useSearchParams();
  const router = useRouter();

  const q = params.get("q") ?? "";
  const page = Math.max(1, Number(params.get("page") ?? "1"));

  // Local input state, debounced into the URL so we don't fire a request per keystroke.
  const [text, setText] = useState(q);
  useEffect(() => {
    const id = setTimeout(() => {
      if (text === q) return;
      const next = new URLSearchParams(params.toString());
      if (text) next.set("q", text);
      else next.delete("q");
      next.delete("page");
      router.replace(`/search?${next.toString()}`);
    }, DEBOUNCE_MS);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text]);

  const setPage = (p: number) => {
    const next = new URLSearchParams(params.toString());
    next.set("page", String(p));
    router.replace(`/search?${next.toString()}`);
  };

  const { data, isLoading, isError, error } = useCandidateSearch({ q, page, limit: LIMIT });
  const hits = data?.data ?? [];
  const total = data?.meta?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / LIMIT));

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Search candidates</h1>
        <p className="text-sm text-muted-foreground">By name or province — across the pipeline</p>
      </div>

      <Input
        type="search"
        autoFocus
        placeholder="Search by name or province…"
        value={text}
        onChange={(e) => setText(e.target.value)}
        className="max-w-md"
        aria-label="Search candidates"
      />

      {!q ? (
        <p className="py-10 text-center text-sm text-muted-foreground">Type a name or province to search.</p>
      ) : isError ? (
        <div className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive">
          {error instanceof Error ? error.message : "Search failed"}
        </div>
      ) : (
        <>
          <p className="text-sm text-muted-foreground">{total} result(s)</p>
          <div className="overflow-hidden rounded-lg border">
            <table className="w-full text-sm">
              <thead className="border-b bg-muted/40 text-left text-xs uppercase tracking-wide text-muted-foreground">
                <tr>
                  <th className="w-16 px-3 py-2">Score</th>
                  <th className="px-3 py-2">Candidate</th>
                  <th className="w-40 px-3 py-2">Province</th>
                  <th className="w-28 px-3 py-2">Status</th>
                </tr>
              </thead>
              <tbody>
                {isLoading &&
                  Array.from({ length: 6 }).map((_, i) => (
                    <tr key={i} className="border-b">
                      <td className="px-3 py-2" colSpan={4}>
                        <Skeleton className="h-5 w-full" />
                      </td>
                    </tr>
                  ))}
                {!isLoading && hits.length === 0 && (
                  <tr>
                    <td colSpan={4} className="px-3 py-10 text-center text-muted-foreground">
                      No candidates match “{q}”.
                    </td>
                  </tr>
                )}
                {hits.map((h) => (
                  <tr key={h.candidate_id} className="border-b transition-colors hover:bg-muted/40">
                    <td className="px-3 py-2">
                      <ScoreBadge score={h.ai_score} />
                    </td>
                    <td className="px-3 py-2">
                      <Link href={`/candidates/${h.candidate_id}`} className="font-medium text-foreground hover:underline">
                        {h.full_name}
                      </Link>
                    </td>
                    <td className="px-3 py-2">{h.province || "—"}</td>
                    <td className="px-3 py-2">
                      <Badge variant="secondary">{h.status}</Badge>
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
              <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(page - 1)}>
                Previous
              </Button>
              <Button variant="outline" size="sm" disabled={page >= pages} onClick={() => setPage(page + 1)}>
                Next
              </Button>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

export default function SearchPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <SearchInner />
    </Suspense>
  );
}
