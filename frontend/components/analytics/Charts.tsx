"use client";

import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import type { Funnel, KPI, Source } from "@/lib/types";
import { Card } from "@/components/ui/card";

const ACCENT = "var(--color-accent, #4f7cff)";

export function KpiCards({ kpi }: { kpi: KPI }) {
  const cards: { label: string; value: number }[] = [
    { label: "Applied", value: kpi.applied },
    { label: "Passed AI", value: kpi.passed },
    { label: "Onboarded", value: kpi.onboarded },
    { label: "Waiting", value: kpi.waiting },
  ];
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {cards.map((c) => (
        <Card key={c.label} className="p-4">
          <div className="text-xs uppercase tracking-wide text-muted-foreground">{c.label}</div>
          <div className="mt-1 text-3xl font-bold tabular-nums">{c.value}</div>
        </Card>
      ))}
    </div>
  );
}

export function FunnelChart({ funnel }: { funnel: Funnel }) {
  const data = [
    { stage: "Applied", value: funnel.applied },
    { stage: "Passed AI", value: funnel.passed_ai },
    { stage: "Reviewed", value: funnel.reviewed },
    { stage: "Hired", value: funnel.hired },
  ];
  return (
    <Card className="p-4">
      <h2 className="mb-3 text-sm font-semibold">Recruitment Funnel</h2>
      <ResponsiveContainer width="100%" height={260}>
        <BarChart data={data} layout="vertical" margin={{ left: 16 }}>
          <CartesianGrid horizontal={false} strokeOpacity={0.2} />
          <XAxis type="number" allowDecimals={false} fontSize={12} />
          <YAxis type="category" dataKey="stage" width={80} fontSize={12} />
          <Tooltip />
          <Bar dataKey="value" radius={[0, 4, 4, 0]} fill={ACCENT} isAnimationActive={false} />
        </BarChart>
      </ResponsiveContainer>
    </Card>
  );
}

export function SourcesChart({ sources }: { sources: Source[] }) {
  const data = sources.map((s) => ({ ...s, pct: Math.round(s.conversion * 100) }));
  return (
    <Card className="p-4">
      <h2 className="mb-3 text-sm font-semibold">Sourcing Efficiency</h2>
      <ResponsiveContainer width="100%" height={260}>
        <BarChart data={data} margin={{ left: 8 }}>
          <CartesianGrid vertical={false} strokeOpacity={0.2} />
          <XAxis dataKey="channel" fontSize={12} />
          <YAxis allowDecimals={false} fontSize={12} />
          <Tooltip />
          <Bar dataKey="applied" radius={[4, 4, 0, 0]} isAnimationActive={false}>
            {data.map((_, i) => (
              <Cell key={i} fill={ACCENT} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </Card>
  );
}
