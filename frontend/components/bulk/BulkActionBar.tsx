"use client";

import { toast } from "sonner";

import { useBulk } from "@/lib/queries";
import { Button } from "@/components/ui/button";

interface BulkActionBarProps {
  selected: string[];
  onDone: () => void;
}

const ACTIONS: { label: string; action: string; value: string }[] = [
  { label: "Shortlist", action: "status", value: "shortlisted" },
  { label: "Interview", action: "status", value: "interview" },
  { label: "Reject", action: "reject", value: "rejected" },
];

export function BulkActionBar({ selected, onDone }: BulkActionBarProps) {
  const bulk = useBulk();
  if (selected.length === 0) return null;

  const run = (action: string, value: string, label: string) => {
    bulk.mutate(
      { ids: selected, action, value },
      {
        onSuccess: () => {
          toast.success(`${label}: ${selected.length} updated`);
          onDone();
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : "Bulk action failed"),
      },
    );
  };

  return (
    <div
      role="region"
      aria-label="Bulk actions"
      className="settle sticky bottom-5 z-10 mx-auto flex w-fit items-center gap-2.5 rounded-xl bg-sidebar px-4 py-2.5 text-sidebar-foreground shadow-2xl ring-1 ring-black/20"
    >
      <span className="flex items-center gap-2 text-sm font-medium">
        <span className="grid size-5 place-items-center rounded-full bg-sidebar-primary text-[0.6875rem] font-semibold text-sidebar-primary-foreground tabular-nums">
          {selected.length}
        </span>
        selected
      </span>
      <span className="mx-1 h-5 w-px bg-sidebar-border" />
      {ACTIONS.map((a) => (
        <Button
          key={a.value}
          size="sm"
          variant={a.action === "reject" ? "destructive" : "secondary"}
          disabled={bulk.isPending}
          onClick={() => run(a.action, a.value, a.label)}
        >
          {a.label}
        </Button>
      ))}
    </div>
  );
}
