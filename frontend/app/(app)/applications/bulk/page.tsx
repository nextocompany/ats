"use client";

import { BulkUpload } from "@/components/applications/BulkUpload";
import { useMe } from "@/lib/queries";
import { canBulkUpload } from "@/lib/roles";

export default function BulkUploadPage() {
  const { data: me, isLoading } = useMe();

  if (isLoading) return null;
  if (!canBulkUpload(me)) {
    return (
      <div className="settle">
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          คุณไม่มีสิทธิ์อัปโหลด CV จำนวนมาก
        </div>
      </div>
    );
  }
  return <BulkUpload />;
}
