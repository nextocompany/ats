"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight } from "lucide-react";

import { ScoreBadge } from "@/components/inbox/ScoreBadge";
import { InitialChip } from "@/components/people/PeopleBits";
import { Checkbox } from "@/components/ui/checkbox";
import type { CompareItem } from "@/lib/types";

import { RecommendationBadge } from "./RecommendationBadge";

interface CompareLeaderboardProps {
  items: CompareItem[];
  selected: string[];
  onToggle: (id: string) => void;
}

// CompareLeaderboard ranks the eligible pool (highest composite first). The top 10
// are visually emphasized; the rest sit below a divider. Each row carries a
// checkbox that adds/removes the candidate from the side-by-side grid.
const TOP_N = 10;

export function CompareLeaderboard({ items, selected, onToggle }: CompareLeaderboardProps) {
  const t = useTranslations("compare");

  function renderRow(it: CompareItem, i: number) {
    const checked = selected.includes(it.application_id);
    return (
      <li key={it.application_id}>
        <div
          className={`flex items-center gap-3 rounded-xl bg-card p-3 ring-1 transition-colors ${
            checked ? "ring-brand/40" : "ring-hairline"
          }`}
        >
          <Checkbox
            checked={checked}
            onCheckedChange={() => onToggle(it.application_id)}
            aria-label={it.candidate_name || "candidate"}
          />
          <span
            className={`num w-6 shrink-0 text-center text-sm font-semibold tabular-nums ${
              i < 3 ? "text-brand" : "text-muted-foreground"
            }`}
          >
            {i + 1}
          </span>
          <InitialChip name={it.candidate_name || "?"} />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium text-foreground">
              {it.candidate_name || "ผู้สมัคร"}
            </span>
            {it.store_name ? (
              <span className="block truncate text-xs text-muted-foreground">{it.store_name}</span>
            ) : null}
          </span>
          <RecommendationBadge recommendation={it.recommendation} />
          <span className="hidden items-center gap-3 sm:flex">
            <ScoreItem label={t("screening")} value={it.screening_score} />
            <ScoreItem label={t("interview")} value={it.interview_score} />
            <span className="text-right">
              <span className="block text-[0.625rem] uppercase tracking-[0.12em] text-muted-foreground">
                {t("composite")}
              </span>
              <span className="num text-base font-semibold tabular-nums text-brand">{it.composite}</span>
            </span>
          </span>
          <Link
            href={`/applications/${it.application_id}`}
            aria-label={t("viewProfile")}
            className="shrink-0 rounded-md p-1 text-muted-foreground/50 transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <ArrowRight className="size-4" />
          </Link>
        </div>
      </li>
    );
  }

  const top = items.slice(0, TOP_N);
  const rest = items.slice(TOP_N);

  return (
    <section className="space-y-3">
      <GroupLabel>{rest.length > 0 ? t("topTen") : t("rankingTitle")}</GroupLabel>
      <ol className="flex flex-col gap-2">{top.map((it, i) => renderRow(it, i))}</ol>

      {rest.length > 0 ? (
        <>
          <GroupLabel>{t("rest")}</GroupLabel>
          <ol className="flex flex-col gap-2 opacity-90">{rest.map((it, i) => renderRow(it, i + TOP_N))}</ol>
        </>
      ) : null}
    </section>
  );
}

function GroupLabel({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{children}</h2>
  );
}

function ScoreItem({ label, value }: { label: string; value: number | null }) {
  return (
    <span className="text-right">
      <span className="block text-[0.625rem] uppercase tracking-[0.12em] text-muted-foreground">{label}</span>
      <span className="flex justify-end">
        <ScoreBadge score={value} />
      </span>
    </span>
  );
}
