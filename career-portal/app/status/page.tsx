"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";

import { PortalShell } from "@/components/PortalShell";
import { StatusCard } from "@/components/StatusCard";
import { StatusTimeline } from "@/components/StatusTimeline";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { useApplicationTimeline, useStatus } from "@/lib/queries";
import { useCandidate } from "@/lib/session";

function StatusContent() {
  const params = useSearchParams();
  const router = useRouter();
  const prefill = params.get("token") ?? "";
  const [input, setInput] = useState(prefill);
  // Query the token from the URL immediately; otherwise wait for a submit.
  const [token, setToken] = useState(prefill);

  const { isAuthenticated, isLoading: authLoading } = useCandidate();
  const { data, isLoading, isError, isFetched } = useStatus(token);
  // The richer timeline is login-gated; only fetch once we know the candidate is
  // authenticated. A 404 (unknown / not-owned token) falls back to the card.
  const timeline = useApplicationTimeline(token, isAuthenticated);
  const hasTimeline = isAuthenticated && timeline.data && !timeline.isError;

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setToken(input.trim());
  }

  function handleLogin() {
    router.push(`/login?return=${encodeURIComponent(`/status?token=${token}`)}`);
  }

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-3">
        <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">ติดตามใบสมัคร</p>
        <h1 className="[font-size:var(--text-h2)] font-semibold leading-tight text-foreground">ตรวจสอบสถานะใบสมัคร</h1>
        <p className="[font-size:var(--text-lead)] text-muted-foreground">กรอกรหัสติดตามที่คุณได้รับหลังสมัครงาน</p>
      </header>

      <form onSubmit={handleSubmit} className="space-y-3">
        <Label htmlFor="token">รหัสติดตาม</Label>
        <div className="flex gap-2">
          <Input
            id="token"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="วางรหัสติดตามที่นี่"
            autoComplete="off"
            className="flex-1"
          />
          <Button type="submit" size="tap" disabled={!input.trim()} className="shrink-0">
            ตรวจสอบ
          </Button>
        </div>
      </form>

      {token && (isLoading || (isAuthenticated && timeline.isLoading)) ? (
        <Skeleton className="h-48 w-full rounded-2xl" />
      ) : null}

      {token && isError ? (
        <div className="rounded-xl border border-line bg-card p-6 text-center">
          <p className="text-sm text-muted-foreground">ไม่พบใบสมัครสำหรับรหัสนี้ กรุณาตรวจสอบรหัสอีกครั้ง</p>
        </div>
      ) : null}

      {/* Logged in + owns this application → the full curated timeline. */}
      {token && hasTimeline ? <StatusTimeline timeline={timeline.data} /> : null}

      {/* Otherwise show the current status card. When not logged in, invite the
          candidate to sign in for the full progress timeline. */}
      {token && data && isFetched && !hasTimeline && !(isAuthenticated && timeline.isLoading) ? (
        <div className="space-y-4">
          <StatusCard status={data} />
          {!isAuthenticated && !authLoading ? (
            <div className="rounded-xl border border-dashed border-line bg-card/60 p-5 text-center">
              <p className="text-sm text-muted-foreground">เข้าสู่ระบบเพื่อดูความคืบหน้าทั้งหมดของใบสมัคร</p>
              <Button variant="outline" size="tap" className="mt-3" onClick={handleLogin}>
                เข้าสู่ระบบ
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export default function StatusPage() {
  return (
    <PortalShell backHref="/jobs" narrow>
      <Suspense fallback={<Skeleton className="h-64 w-full rounded-2xl" />}>
        <StatusContent />
      </Suspense>
    </PortalShell>
  );
}
