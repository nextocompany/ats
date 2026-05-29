"use client";

import Link from "next/link";
import { useParams } from "next/navigation";

import { ScoreBadge } from "@/components/inbox/ScoreBadge";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useCandidate, useTimeline } from "@/lib/queries";

export default function CandidateProfilePage() {
  const { id } = useParams<{ id: string }>();
  const { data, isLoading, isError } = useCandidate(id);
  const { data: timeline } = useTimeline(id);

  if (isLoading) return <Skeleton className="h-96 w-full" />;
  if (isError || !data) return <p className="text-sm text-destructive">Candidate not found.</p>;

  const { candidate, applications } = data;

  return (
    <div className="space-y-6">
      <Link href="/candidates" className="text-sm text-muted-foreground hover:text-foreground">
        ← Back to candidates
      </Link>

      <div className="grid gap-6 lg:grid-cols-[1fr_320px]">
        <div className="space-y-6">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">{candidate.full_name}</h1>
            <div className="mt-2 flex flex-wrap gap-2 text-xs">
              <Badge variant="secondary">{candidate.status}</Badge>
              {candidate.province && <Badge variant="outline">{candidate.province}</Badge>}
              {candidate.source_channel && <Badge variant="outline">{candidate.source_channel}</Badge>}
            </div>
            <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
              <dt className="text-muted-foreground">Phone</dt>
              <dd>{candidate.phone || "—"}</dd>
              <dt className="text-muted-foreground">Email</dt>
              <dd>{candidate.email || "—"}</dd>
            </dl>
          </div>

          <section>
            <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
              Applications ({applications.length})
            </h2>
            <div className="space-y-2">
              {applications.map((a) => (
                <Link key={a.id} href={`/applications/${a.id}`}>
                  <Card className="flex items-center justify-between p-3 transition-colors hover:bg-muted/40">
                    <div className="flex items-center gap-3">
                      <ScoreBadge score={a.ai_score} />
                      <span className="text-sm">{a.id.slice(0, 8)}</span>
                    </div>
                    <Badge variant="secondary">{a.status}</Badge>
                  </Card>
                </Link>
              ))}
            </div>
          </section>
        </div>

        <aside aria-label="History">
          <h2 className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">Timeline</h2>
          <ol className="space-y-3 border-l pl-4 text-sm">
            {(timeline ?? []).map((e, i) => (
              <li key={i} className="relative">
                <span className="absolute -left-[21px] top-1.5 h-2 w-2 rounded-full bg-[var(--color-accent)]" />
                <div className="font-medium">{e.action}</div>
                <div className="text-xs text-muted-foreground">
                  {e.entity_type} · {new Date(e.created_at).toLocaleString()}
                </div>
              </li>
            ))}
            {(timeline ?? []).length === 0 && <li className="text-muted-foreground">No history yet.</li>}
          </ol>
        </aside>
      </div>
    </div>
  );
}
