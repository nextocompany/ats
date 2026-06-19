"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { X } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useAddTag, useMemberTags, useRemoveTag } from "@/lib/queries";

export function TagEditor({ memberId }: { memberId: string }) {
  const t = useTranslations("members");
  const { data: tags, isLoading } = useMemberTags(memberId);
  const add = useAddTag(memberId);
  const remove = useRemoveTag(memberId);
  const [value, setValue] = useState("");

  const submit = () => {
    const tag = value.trim();
    if (!tag) return;
    add.mutate(tag, {
      onSuccess: () => setValue(""),
      onError: (e) => toast.error(e instanceof Error ? e.message : t("tagAddFailed")),
    });
  };

  return (
    <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
      <h2 className="eyebrow mb-3">{t("tagsHeading")}</h2>

      {isLoading ? (
        <Skeleton className="h-6 w-32 rounded-full" />
      ) : (
        <div className="flex flex-wrap gap-1.5">
          {(tags?.length ?? 0) === 0 && <span className="text-sm text-muted-foreground">{t("noTags")}</span>}
          {tags?.map((tagName) => (
            <span
              key={tagName}
              className="inline-flex items-center gap-1 rounded-full bg-brand-soft px-2 py-0.5 text-xs font-medium text-brand"
            >
              {tagName}
              <button
                type="button"
                aria-label={t("removeTag", { tag: tagName })}
                className="rounded-full p-0.5 hover:bg-brand/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() =>
                  remove.mutate(tagName, {
                    onError: (e) => toast.error(e instanceof Error ? e.message : t("tagRemoveFailed")),
                  })
                }
              >
                <X className="size-3" />
              </button>
            </span>
          ))}
        </div>
      )}

      <div className="mt-3 flex gap-2">
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              submit();
            }
          }}
          placeholder={t("addTagPlaceholder")}
          maxLength={50}
          className="h-8"
        />
        <Button size="sm" variant="outline" onClick={submit} disabled={add.isPending || !value.trim()}>
          {t("addTag")}
        </Button>
      </div>
    </div>
  );
}
