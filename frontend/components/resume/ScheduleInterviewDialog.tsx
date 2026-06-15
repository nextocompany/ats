"use client";

// Schedule a human interview: date/time, duration, and onsite/online mode. For an
// online interview the backend creates a Teams meeting and emails the candidate
// the calendar invite. Reachable only from ai_interviewed / shortlisted.
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { useScheduleInterview } from "@/lib/queries";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

interface Props {
  applicationId: string;
  open: boolean;
  onClose: () => void;
}

// Local field label (the dashboard has no shared Label primitive). Rendered as a
// span because each field's outer wrapper is already a <label>.
function Label({ children }: { htmlFor?: string; children: React.ReactNode }) {
  return <span className="text-xs font-medium text-foreground">{children}</span>;
}

export function ScheduleInterviewDialog({ applicationId, open, onClose }: Props) {
  const schedule = useScheduleInterview(applicationId);
  const [when, setWhen] = useState("");
  const [duration, setDuration] = useState("60");
  const [mode, setMode] = useState<"onsite" | "online">("onsite");
  const [location, setLocation] = useState("");

  function close() {
    setWhen("");
    setDuration("60");
    setMode("onsite");
    setLocation("");
    schedule.reset();
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!when) return;
    await schedule.mutateAsync(
      {
        // datetime-local is wall-clock in the browser zone; toISOString normalizes
        // to UTC RFC3339, which the backend parses + checks is in the future.
        scheduled_at: new Date(when).toISOString(),
        duration_min: Number(duration),
        mode,
        location_text: location.trim() || undefined,
      },
      {
        onSuccess: () => {
          toast.success(mode === "online" ? "Interview scheduled — Teams invite sent" : "Interview scheduled");
          close();
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : "Could not schedule"),
      },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Schedule interview</DialogTitle>
          <DialogDescription>
            Pick a date, time, and mode. Online interviews create a Microsoft Teams
            meeting and email the candidate a calendar invite.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3" noValidate>
          <label className="block space-y-1.5">
            <Label htmlFor="when">Date &amp; time</Label>
            <Input id="when" type="datetime-local" value={when} onChange={(e) => setWhen(e.target.value)} required />
          </label>
          <div className="grid grid-cols-2 gap-3">
            <label className="block space-y-1.5">
              <Label>Duration</Label>
              <Select value={duration} onValueChange={(v) => setDuration(v ?? "60")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {["30", "45", "60", "90"].map((d) => (
                    <SelectItem key={d} value={d}>
                      {d} min
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <label className="block space-y-1.5">
              <Label>Mode</Label>
              <Select value={mode} onValueChange={(v) => setMode((v as "onsite" | "online") ?? "onsite")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="onsite">Onsite</SelectItem>
                  <SelectItem value="online">Online (Teams)</SelectItem>
                </SelectContent>
              </Select>
            </label>
          </div>
          <label className="block space-y-1.5">
            <Label htmlFor="loc">{mode === "onsite" ? "Location" : "Note (optional)"}</Label>
            <Input
              id="loc"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
              placeholder={mode === "onsite" ? "เช่น สำนักงานใหญ่ ชั้น 10" : "รายละเอียดเพิ่มเติม"}
            />
          </label>

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" disabled={!when || schedule.isPending} className="gap-2">
              {schedule.isPending && <Loader2 className="size-4 animate-spin" />}
              Schedule
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
