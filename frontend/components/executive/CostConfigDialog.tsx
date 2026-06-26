"use client";

// Cost-assumptions editor (settings.admin only). These figures drive the entire
// ROI band; an empty field means "unset" (null), which keeps the ROI cards in
// their honest empty-state rather than computing against a fabricated zero.
import { useState } from "react";
import { useTranslations } from "next-intl";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { useExecCostConfig, useSetExecCostConfig } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ExecCostConfig } from "@/lib/types";

interface Props {
  open: boolean;
  onClose: () => void;
}

// A nullable-number field rendered as a string for the controlled input.
type FormState = {
  currency: string;
  system_cost_monthly: string;
  traditional_cost_per_hire: string;
  vacancy_cost_per_day: string;
  traditional_time_to_hire_days: string;
};

const NUMERIC_FIELDS = [
  "system_cost_monthly",
  "traditional_cost_per_hire",
  "vacancy_cost_per_day",
  "traditional_time_to_hire_days",
] as const;

function toForm(cfg?: ExecCostConfig): FormState {
  const s = (n: number | null | undefined) => (n === null || n === undefined ? "" : String(n));
  return {
    currency: cfg?.currency || "THB",
    system_cost_monthly: s(cfg?.system_cost_monthly),
    traditional_cost_per_hire: s(cfg?.traditional_cost_per_hire),
    vacancy_cost_per_day: s(cfg?.vacancy_cost_per_day),
    traditional_time_to_hire_days: s(cfg?.traditional_time_to_hire_days),
  };
}

export function CostConfigDialog({ open, onClose }: Props) {
  const t = useTranslations("executive");
  const { data: cfg } = useExecCostConfig(open);
  const save = useSetExecCostConfig();
  const [form, setForm] = useState<FormState>(toForm());

  // Re-seed the form from the persisted figures whenever the dialog opens or the
  // loaded config changes. This is the React "adjust state during render" pattern
  // (not an effect) — keyed on open + config identity, it avoids the cascading
  // renders the set-state-in-effect lint rule guards against.
  const seedKey = open ? `${cfg?.updated_at ?? "loading"}|${cfg?.currency ?? ""}` : "closed";
  const [seededFor, setSeededFor] = useState("closed");
  if (seedKey !== seededFor) {
    setSeededFor(seedKey);
    setForm(toForm(cfg));
  }

  function field(name: keyof FormState, value: string) {
    setForm((prev) => ({ ...prev, [name]: value }));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const parse = (v: string): number | null => {
      const trimmed = v.trim();
      if (trimmed === "") return null;
      const n = Number(trimmed);
      return Number.isFinite(n) ? n : null;
    };
    // Reject negatives client-side (server enforces too).
    for (const f of NUMERIC_FIELDS) {
      const n = parse(form[f]);
      if (n !== null && n < 0) {
        toast.error(t("costNegative"));
        return;
      }
    }
    const payload: ExecCostConfig = {
      currency: form.currency.trim() || "THB",
      system_cost_monthly: parse(form.system_cost_monthly),
      traditional_cost_per_hire: parse(form.traditional_cost_per_hire),
      vacancy_cost_per_day: parse(form.vacancy_cost_per_day),
      traditional_time_to_hire_days: parse(form.traditional_time_to_hire_days),
    };
    await save.mutateAsync(payload, {
      onSuccess: () => {
        toast.success(t("costSaved"));
        onClose();
      },
      onError: (err) => toast.error(err instanceof Error ? err.message : t("costSaveFailed")),
    });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : onClose())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("costTitle")}</DialogTitle>
          <DialogDescription>{t("costDesc")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4" noValidate>
          <NumberField
            label={t("costCurrency")}
            value={form.currency}
            onChange={(v) => field("currency", v)}
            text
            placeholder="THB"
          />
          <NumberField
            label={t("costSystemMonthly")}
            hint={t("costSystemMonthlyHint")}
            value={form.system_cost_monthly}
            onChange={(v) => field("system_cost_monthly", v)}
          />
          <NumberField
            label={t("costTraditionalPerHire")}
            hint={t("costTraditionalPerHireHint")}
            value={form.traditional_cost_per_hire}
            onChange={(v) => field("traditional_cost_per_hire", v)}
          />
          <NumberField
            label={t("costVacancyPerDay")}
            hint={t("costVacancyPerDayHint")}
            value={form.vacancy_cost_per_day}
            onChange={(v) => field("vacancy_cost_per_day", v)}
          />
          <NumberField
            label={t("costTraditionalTth")}
            hint={t("costTraditionalTthHint")}
            value={form.traditional_time_to_hire_days}
            onChange={(v) => field("traditional_time_to_hire_days", v)}
          />

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={onClose}>
              {t("costCancel")}
            </Button>
            <Button type="submit" disabled={save.isPending} className="gap-2">
              {save.isPending && <Loader2 className="size-4 animate-spin" />}
              {t("costSave")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function NumberField({
  label,
  hint,
  value,
  onChange,
  text = false,
  placeholder,
}: {
  label: string;
  hint?: string;
  value: string;
  onChange: (v: string) => void;
  text?: boolean;
  placeholder?: string;
}) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-foreground">{label}</span>
      <Input
        type={text ? "text" : "number"}
        inputMode={text ? undefined : "decimal"}
        min={text ? undefined : 0}
        step="any"
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
      />
      {hint && <span className="block text-xs text-muted-foreground">{hint}</span>}
    </label>
  );
}
