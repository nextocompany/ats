"use client";

import { useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";

import { PortalShell } from "@/components/PortalShell";
import { StatusCard } from "@/components/StatusCard";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { useStatus } from "@/lib/queries";

function StatusContent() {
  const params = useSearchParams();
  const prefill = params.get("token") ?? "";
  const [input, setInput] = useState(prefill);
  // Query the token from the URL immediately; otherwise wait for a submit.
  const [token, setToken] = useState(prefill);

  const { data, isLoading, isError, isFetched } = useStatus(token);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setToken(input.trim());
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

      {token && isLoading ? <Skeleton className="h-48 w-full rounded-2xl" /> : null}

      {token && isError ? (
        <div className="rounded-xl border border-line bg-card p-6 text-center">
          <p className="text-sm text-muted-foreground">ไม่พบใบสมัครสำหรับรหัสนี้ กรุณาตรวจสอบรหัสอีกครั้ง</p>
        </div>
      ) : null}

      {token && data && isFetched ? <StatusCard status={data} /> : null}
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
