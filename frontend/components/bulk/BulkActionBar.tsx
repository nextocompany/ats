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
      className="sticky bottom-4 z-10 mx-auto flex w-fit items-center gap-2 rounded-lg border bg-background px-4 py-2 shadow-lg"
    >
      <span className="text-sm font-medium">{selected.length} selected</span>
      <span className="mx-1 h-4 w-px bg-border" />
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
