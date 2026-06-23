import type { ApplicationStatus } from "@/lib/types";
import { TONE_CLASS, formatThaiDate, metaFor } from "@/lib/applicationStatus";

export function StatusCard({ status }: { status: ApplicationStatus }) {
  const meta = metaFor(status.status);
  return (
    <div className="space-y-5 rounded-xl border border-line bg-card p-6">
      <span className={`inline-flex rounded-full px-3 py-1 text-sm font-medium ${TONE_CLASS[meta.tone]}`}>
        {meta.label}
      </span>
      <p className="text-sm leading-relaxed text-muted-foreground">{meta.detail}</p>
      <dl className="space-y-3 border-t border-line pt-4 text-sm">
        {status.position ? (
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">ตำแหน่ง</dt>
            <dd className="text-right font-medium">{status.position}</dd>
          </div>
        ) : null}
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">วันที่สมัคร</dt>
          <dd className="text-right font-medium">{formatThaiDate(status.applied_at)}</dd>
        </div>
      </dl>
    </div>
  );
}
