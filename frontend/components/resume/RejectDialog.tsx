"use client";

// Reject an application with a mandatory reason. The reason is stored internally
// for HR — it is never sent to the candidate (per spec).
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { useSetStatus } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
interface Props {
  applicationId: string;
  open: boolean;
  onClose: () => void;
}

// Local field label (the dashboard has no shared Label primitive). Rendered as a
// span because the field's outer wrapper is already a <label>.
function Label({ children }: { htmlFor?: string; children: React.ReactNode }) {
  return <span className="text-xs font-medium text-foreground">{children}</span>;
}

export function RejectDialog({ applicationId, open, onClose }: Props) {
  const setStatus = useSetStatus(applicationId);
  const [reason, setReason] = useState("");

  function close() {
    setReason("");
    setStatus.reset();
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = reason.trim();
    if (!trimmed) return;
    await setStatus.mutateAsync(
      { status: "rejected", reason: trimmed },
      {
        onSuccess: () => {
          toast.success("Candidate rejected");
          close();
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : "Could not reject"),
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reject candidate</DialogTitle>
          <DialogDescription>
            A reason is required. It is recorded for HR only and is never sent to the candidate.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3" noValidate>
          <label className="block space-y-1.5">
            <Label htmlFor="reason">Reason</Label>
            <textarea
              id="reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={4}
              required
              className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              placeholder="เช่น คุณสมบัติไม่ตรงกับตำแหน่ง / ไม่ผ่านการสัมภาษณ์"
            />
          </label>

          {setStatus.isError && (
            <p role="alert" className="text-xs font-medium text-destructive">
              {setStatus.error instanceof Error ? setStatus.error.message : "Failed"}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" variant="destructive" disabled={!reason.trim() || setStatus.isPending} className="gap-2">
              {setStatus.isPending && <Loader2 className="size-4 animate-spin" />}
              Reject
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
