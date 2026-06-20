"use client";

import { useTranslations } from "next-intl";

import { BulkUpload } from "@/components/applications/BulkUpload";
import { useMe } from "@/lib/queries";
import { canBulkUpload } from "@/lib/roles";

export default function BulkUploadPage() {
  const t = useTranslations("bulk");
  const { data: me, isLoading } = useMe();

  if (isLoading) return null;
  if (!canBulkUpload(me)) {
    return (
      <div className="settle">
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          {t("noPermission")}
        </div>
      </div>
    );
  }
  return <BulkUpload />;
}
