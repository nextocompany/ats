"use client";

// Letter generation (Module-3 3.3). HR (letter roles) generates an interview or
// offer PDF letter; both HR and the candidate can download it. Generation is
// gated server-side by preconditions (a scheduled interview / a sent offer) — a
// missing precondition surfaces as a toast.
import { Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { Application, LetterType } from "@/lib/types";
import { useGenerateLetter, useLetters, useMe } from "@/lib/queries";
import { canManageLetters } from "@/lib/roles";
import { Button } from "@/components/ui/button";

interface Props {
  applicationId: string;
  app: Application;
}

// Letters only make sense once the candidate has reached the interview band (an
// interview letter needs a scheduled appointment, an offer letter needs a sent
// offer). Earlier statuses hide the panel; the server is the real precondition gate
// (a premature click returns 400 → toast), so this set is only a UX affordance.
const SHOW_STATUSES = new Set([
  "interview",
  "interviewed",
  "pending_approval",
  "offer",
  "hired",
  "rejected",
]);

export function LettersPanel({ applicationId, app }: Props) {
  const t = useTranslations("letters");
  const { data: me } = useMe();
  const { data: letters } = useLetters(applicationId);
  const generate = useGenerateLetter(applicationId);
  const canManage = canManageLetters(me);

  if (!canManage && (!letters || letters.length === 0)) return null;
  if (!SHOW_STATUSES.has(app.status) && (!letters || letters.length === 0)) return null;

  // Which letter type's generation is in flight (so only that button spins).
  const pendingType = generate.isPending ? generate.variables : undefined;

  function gen(type: LetterType) {
    generate.mutate(type, {
      onSuccess: () => toast.success(t("generated")),
      onError: (e) => toast.error(e instanceof Error ? e.message : t("generateFailed")),
    });
  }

  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <p className="eyebrow">{t("title")}</p>

      {canManage && (
        <div className="mt-3 grid grid-cols-2 gap-2">
          <Button size="sm" variant="secondary" className="gap-2" disabled={generate.isPending} onClick={() => gen("interview")}>
            {pendingType === "interview" && <Loader2 className="size-4 animate-spin" />}
            {t("generateInterview")}
          </Button>
          <Button size="sm" variant="secondary" className="gap-2" disabled={generate.isPending} onClick={() => gen("offer")}>
            {pendingType === "offer" && <Loader2 className="size-4 animate-spin" />}
            {t("generateOffer")}
          </Button>
        </div>
      )}

      {letters && letters.length > 0 ? (
        <ul className="mt-3 flex flex-col gap-2">
          {letters.map((l) => (
            <li key={l.id} className="flex items-center justify-between gap-3 rounded-lg bg-muted/40 px-3 py-2">
              <span className="text-sm text-foreground">
                {l.type === "interview" ? t("type_interview") : t("type_offer")}
                <span className="ml-2 text-xs text-muted-foreground">{new Date(l.created_at).toLocaleDateString()}</span>
              </span>
              {l.url && (
                <a
                  href={l.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-sm font-medium text-brand underline-offset-2 hover:underline"
                >
                  {t("open")}
                </a>
              )}
            </li>
          ))}
        </ul>
      ) : (
        canManage && <p className="mt-3 text-xs text-muted-foreground">{t("empty")}</p>
      )}
    </section>
  );
}
