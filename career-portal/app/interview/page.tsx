"use client";

import { useSyncExternalStore } from "react";

import { InterviewChat } from "@/components/InterviewChat";
import { PortalShell } from "@/components/PortalShell";
import { Skeleton } from "@/components/ui/skeleton";

// The token lives in the URL fragment (`#token=…`), not the query string, so it
// is never sent to the server — keeping it out of access logs and referrers.
function readHashToken(): string {
  const hash = window.location.hash.replace(/^#/, "");
  return new URLSearchParams(hash).get("token") ?? "";
}

function subscribeHash(onChange: () => void): () => void {
  window.addEventListener("hashchange", onChange);
  return () => window.removeEventListener("hashchange", onChange);
}

function InterviewContent() {
  // Read the hash as an external store: client snapshot returns the token; the
  // server snapshot is null so SSR/hydration shows a skeleton (no token flash).
  const token = useSyncExternalStore<string | null>(subscribeHash, readHashToken, () => null);

  if (token === null) {
    return <Skeleton className="h-80 w-full rounded-2xl" />;
  }

  if (!token) {
    return (
      <div className="rounded-2xl border border-border bg-card p-6 text-center">
        <p className="text-base font-medium text-foreground">ไม่พบรหัสสัมภาษณ์</p>
        <p className="mt-2 text-sm text-muted-foreground">กรุณาเปิดลิงก์สัมภาษณ์ที่ได้รับจากทีม HR</p>
      </div>
    );
  }

  return <InterviewChat token={token} />;
}

export default function InterviewPage() {
  return (
    <PortalShell backHref="/jobs" narrow>
      <InterviewContent />
    </PortalShell>
  );
}
