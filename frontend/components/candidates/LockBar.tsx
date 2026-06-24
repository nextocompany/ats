"use client";

// Candidate processing lock bar — shown on the person detail. When several
// operators share the central pool, this makes "who is working this candidate"
// visible and lets one claim it. Keyed by the canonical candidates.id.
import { useTranslations, useLocale } from "next-intl";
import { Loader2, Lock, LockOpen } from "lucide-react";
import { toast } from "sonner";

import { ApiError } from "@/lib/api";
import { useAcquireLock, useCandidateLock, useReleaseLock } from "@/lib/queries";
import { canManageUsers } from "@/lib/roles";
import type { Me } from "@/lib/types";

interface LockBarProps {
  candidateId: string;
  me?: Me;
}

export function LockBar({ candidateId, me }: LockBarProps) {
  const t = useTranslations("lock");
  const locale = useLocale();
  const { data: lock, isLoading } = useCandidateLock(candidateId);
  const acquire = useAcquireLock();
  const release = useReleaseLock();

  const mine = !!lock && !!me?.local_id && lock.locked_by === me.local_id;
  const canForce = canManageUsers(me);

  function fmtTime(iso: string): string {
    try {
      return new Intl.DateTimeFormat(locale === "th" ? "th-TH" : "en-GB", {
        hour: "2-digit",
        minute: "2-digit",
      }).format(new Date(iso));
    } catch {
      return iso;
    }
  }

  async function onAcquire() {
    try {
      await acquire.mutateAsync(candidateId);
      toast.success(t("claimed"));
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) toast.error(e.message);
      else toast.error(t("claimFailed"));
    }
  }

  async function onRelease() {
    try {
      await release.mutateAsync(candidateId);
      toast.success(t("released"));
    } catch {
      toast.error(t("releaseFailed"));
    }
  }

  if (isLoading) return null;

  // Unlocked → invite the operator to claim it before working.
  if (!lock) {
    return (
      <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-muted/40 px-4 py-3 text-sm">
        <span className="flex items-center gap-2 text-muted-foreground">
          <LockOpen className="h-4 w-4" /> {t("unlocked")}
        </span>
        <button
          onClick={onAcquire}
          disabled={acquire.isPending}
          className="inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground transition hover:opacity-90 disabled:opacity-50"
        >
          {acquire.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Lock className="h-3.5 w-3.5" />}
          {t("claim")}
        </button>
      </div>
    );
  }

  // Locked by me → reassure + allow release.
  if (mine) {
    return (
      <div className="flex items-center justify-between gap-3 rounded-lg border border-primary/40 bg-accent-soft px-4 py-3 text-sm">
        <span className="flex items-center gap-2 font-medium text-primary">
          <Lock className="h-4 w-4" /> {t("youAreProcessing")} · {t("until", { time: fmtTime(lock.expires_at) })}
        </span>
        <button
          onClick={onRelease}
          disabled={release.isPending}
          className="inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-3 py-1.5 text-xs font-semibold transition hover:bg-muted disabled:opacity-50"
        >
          {release.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <LockOpen className="h-3.5 w-3.5" />}
          {t("release")}
        </button>
      </div>
    );
  }

  // Locked by someone else → show holder; admins can force-release.
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg border px-4 py-3 text-sm" style={{ borderColor: "var(--amber, #c98a16)", background: "var(--amber-soft, #fbf2dd)" }}>
      <span className="flex items-center gap-2 font-medium" style={{ color: "var(--amber, #c98a16)" }}>
        <Lock className="h-4 w-4" />
        {t("processingBy", { name: lock.locked_by_name || t("anotherUser") })} · {t("until", { time: fmtTime(lock.expires_at) })}
      </span>
      {canForce && (
        <button
          onClick={onRelease}
          disabled={release.isPending}
          className="inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-3 py-1.5 text-xs font-semibold text-destructive transition hover:bg-muted disabled:opacity-50"
        >
          {release.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <LockOpen className="h-3.5 w-3.5" />}
          {t("forceRelease")}
        </button>
      )}
    </div>
  );
}
