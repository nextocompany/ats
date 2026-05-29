"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Candidates</h1>
        <p className="text-sm text-muted-foreground">{total} candidate(s)</p>
      </div>
      <div className="overflow-hidden rounded-lg border">
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/40 text-left text-xs uppercase tracking-wide text-muted-foreground">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Province</th>
              <th className="px-3 py-2">Source</th>
              <th className="w-28 px-3 py-2">Status</th>
            </tr>
          </thead>
          <tbody>
            {isLoading &&
              Array.from({ length: 8 }).map((_, i) => (
                <tr key={i} className="border-b">
                  <td colSpan={4} className="px-3 py-2">
                    <Skeleton className="h-5 w-full" />
                  </td>
                </tr>
              ))}
            {!isLoading && items.length === 0 && (
              <tr>
                <td colSpan={4} className="px-3 py-10 text-center text-muted-foreground">
                  No candidates yet.
                </td>
              </tr>
            )}
            {items.map((c) => (
              <tr key={c.id} className="border-b transition-colors hover:bg-muted/40">
                <td className="px-3 py-2">
                  <Link href={`/candidates/${c.id}`} className="font-medium hover:underline">
                    {c.full_name}
                  </Link>
                </td>
                <td className="px-3 py-2">{c.province || "—"}</td>
                <td className="px-3 py-2">{c.source_channel || "—"}</td>
                <td className="px-3 py-2">
                  <Badge variant="secondary">{c.status}</Badge>
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
          <Button
            variant="outline"
            size="sm"
            disabled={page <= 1}
            onClick={() => router.replace(`/candidates?page=${page - 1}`)}
          >
            Previous
          </Button>
          <Button
            variant="outline"
            size="sm"
            disabled={page >= pages}
            onClick={() => router.replace(`/candidates?page=${page + 1}`)}
          >
            Next
          </Button>
        </div>
      </div>
    </div>
  );
}

export default function CandidatesPage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <CandidatesInner />
    </Suspense>
  );
}
