"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft } from "lucide-react";

import { ScoreBadge } from "@/components/inbox/ScoreBadge";
import { SourceChip, StatusPill } from "@/components/people/PeopleBits";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { useCandidate, useTimeline } from "@/lib/queries";

export default function CandidateProfilePage() {
  const { id } = useParams<{ id: string }>();
  const { data, isLoading, isError } = useCandidate(id);
  const { data: timeline } = useTimeline(id);

  if (isLoading) return <Skeleton className="h-96 w-full rounded-xl" />;
  if (isError || !data)
    return (
      <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
        Candidate not found.
      </div>
    );

  const { candidate, applications } = data;
  const initials = candidate.full_name.slice(0, 2).toUpperCase();

  return (
    <div className="settle space-y-6">
      <Link
        href="/candidates"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
      >
        <ArrowLeft className="size-4" /> Back to candidates
      </Link>

      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="space-y-6">
          {/* Identity card */}
          <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
            <div className="flex items-start gap-4">
              <span className="grid size-14 shrink-0 place-items-center rounded-xl bg-brand text-lg font-semibold text-brand-foreground">
                {initials}
              </span>
              <div className="min-w-0 flex-1">
                <p className="eyebrow brass-underline inline-block">Candidate</p>
                <h1 className="mt-3 font-heading text-2xl font-semibold tracking-tight">
                  {candidate.full_name}
                </h1>
                <div className="mt-3 flex flex-wrap items-center gap-2 text-xs">
                  <StatusPill status={candidate.status} />
                  {candidate.province && (
                    <Badge variant="outline" className="rounded-full">{candidate.province}</Badge>
                  )}
                  {candidate.source_channel && <SourceChip channel={candidate.source_channel} />}
                </div>
              </div>
            </div>
            <dl className="mt-5 grid grid-cols-1 gap-x-8 gap-y-3 border-t border-hairline pt-5 text-sm sm:grid-cols-2">
              <div className="flex justify-between gap-4 sm:block">
                <dt className="text-muted-foreground sm:text-xs sm:uppercase sm:tracking-wide">Phone</dt>
                <dd className="font-medium tabular-nums sm:mt-1">{candidate.phone || "-"}</dd>
              </div>
              <div className="flex justify-between gap-4 sm:block">
                <dt className="text-muted-foreground sm:text-xs sm:uppercase sm:tracking-wide">Email</dt>
                <dd className="truncate font-medium sm:mt-1">{candidate.email || "-"}</dd>
              </div>
            </dl>
          </section>

          <section>
            <h2 className="eyebrow mb-3">Applications ({applications.length})</h2>
            <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
              {applications.length === 0 ? (
                <p className="px-5 py-8 text-center text-sm text-muted-foreground">No applications yet.</p>
              ) : (
                applications.map((a, i) => (
                  <Link
                    key={a.id}
                    href={`/applications/${a.id}`}
                    className={`flex items-center justify-between px-5 py-3.5 transition-colors hover:bg-brand-soft/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${
                      i > 0 ? "border-t border-hairline" : ""
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <ScoreBadge score={a.ai_score} />
                      <span className="font-mono text-sm text-foreground">{a.id.slice(0, 8)}</span>
                    </div>
                    <Badge variant="secondary" className="capitalize">{a.status}</Badge>
                  </Link>
                ))
              )}
            </div>
          </section>
        </div>

        <aside aria-label="History">
          <h2 className="eyebrow mb-3">Timeline</h2>
          <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
            <ol className="relative space-y-4 border-l border-hairline pl-5 text-sm">
              {(timeline ?? []).map((e, i) => (
                <li key={i} className="relative">
                  <span className="absolute -left-[1.5625rem] top-1 size-2.5 rounded-full bg-brand ring-4 ring-card" />
                  <div className="font-medium text-foreground">{e.action}</div>
                  <div className="text-xs text-muted-foreground tabular-nums">
                    {e.entity_type} · {new Date(e.created_at).toLocaleString()}
                  </div>
                </li>
              ))}
              {(timeline ?? []).length === 0 && (
                <li className="text-muted-foreground">No history yet.</li>
              )}
            </ol>
          </div>
        </aside>
      </div>
    </div>
  );
}
