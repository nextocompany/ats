"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { LogOut, Pencil, ShieldCheck, ShieldOff, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  useAnonymizeMember,
  useForceLogout,
  useSetMemberStatus,
  useUpdateMember,
} from "@/lib/queries";
import { canEraseMember } from "@/lib/roles";
import type { Me, Member } from "@/lib/types";

interface MemberActionsProps {
  member: Member;
  me?: Me;
}

function errMsg(e: unknown, fallback: string): string {
  return e instanceof Error ? e.message : fallback;
}

export function MemberActions({ member, me }: MemberActionsProps) {
  const t = useTranslations("members");
  const setStatus = useSetMemberStatus(member.id);
  const forceLogout = useForceLogout(member.id);
  const anonymize = useAnonymizeMember(member.id);

  // Anonymized accounts are terminal — no lifecycle actions apply.
  if (member.status === "anonymized") {
    return (
      <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
        <h2 className="eyebrow mb-2">{t("manage")}</h2>
        <p className="text-xs text-muted-foreground">{t("anonymizedNotice")}</p>
      </div>
    );
  }

  const suspended = member.status === "suspended";

  const toggleSuspend = () => {
    const next = suspended ? "active" : "suspended";
    setStatus.mutate(next, {
      onSuccess: () => toast.success(suspended ? t("reactivated") : t("suspendedToast")),
      onError: (e) => toast.error(errMsg(e, t("actionFailed"))),
    });
  };

  const doForceLogout = () => {
    forceLogout.mutate(undefined, {
      onSuccess: () => toast.success(t("forceLoggedOut")),
      onError: (e) => toast.error(errMsg(e, t("actionFailed"))),
    });
  };

  return (
    <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
      <h2 className="eyebrow mb-3">{t("manage")}</h2>
      <div className="flex flex-col gap-2">
        {/* Suspend (confirmed) / reactivate (direct) */}
        {suspended ? (
          <Button
            variant="outline"
            size="sm"
            className="justify-start"
            disabled={setStatus.isPending}
            onClick={toggleSuspend}
          >
            <ShieldCheck className="size-4" /> {t("reactivate")}
          </Button>
        ) : (
          <ConfirmButton
            triggerVariant="destructive"
            triggerChildren={
              <>
                <ShieldOff className="size-4" /> {t("suspend")}
              </>
            }
            title={t("confirmSuspendTitle")}
            description={t("confirmSuspendDesc")}
            confirmLabel={t("suspend")}
            cancelLabel={t("cancel")}
            pending={setStatus.isPending}
            onConfirm={toggleSuspend}
          />
        )}

        {/* Force logout */}
        <Button
          variant="outline"
          size="sm"
          className="justify-start"
          disabled={forceLogout.isPending || member.active_sessions === 0}
          onClick={doForceLogout}
        >
          <LogOut className="size-4" /> {t("forceLogout")}
          {member.active_sessions > 0 && (
            <span className="ml-1 tabular-nums text-muted-foreground">({member.active_sessions})</span>
          )}
        </Button>

        {/* Edit profile */}
        <EditProfileDialog member={member} />

        {/* PDPA erasure — super_admin only */}
        {canEraseMember(me) && (
          <>
            <div className="my-1 border-t border-hairline" />
            <ConfirmButton
              triggerVariant="destructive"
              triggerChildren={
                <>
                  <Trash2 className="size-4" /> {t("eraseTrigger")}
                </>
              }
              title={t("confirmEraseTitle")}
              description={t("confirmEraseDesc")}
              confirmLabel={t("eraseConfirm")}
              cancelLabel={t("cancel")}
              pending={anonymize.isPending}
              onConfirm={() =>
                anonymize.mutate(undefined, {
                  onSuccess: () => toast.success(t("erased")),
                  onError: (e) => toast.error(errMsg(e, t("eraseFailed"))),
                })
              }
            />
          </>
        )}
      </div>
    </div>
  );
}

interface ConfirmButtonProps {
  triggerChildren: React.ReactNode;
  triggerVariant: "outline" | "destructive";
  title: string;
  description: string;
  confirmLabel: string;
  cancelLabel: string;
  onConfirm: () => void;
  pending?: boolean;
}

// ConfirmButton wraps a destructive action in a confirm Dialog (Base UI has no
// AlertDialog; the trigger composes a Button via the `render` prop).
function ConfirmButton({
  triggerChildren,
  triggerVariant,
  title,
  description,
  confirmLabel,
  cancelLabel,
  onConfirm,
  pending,
}: ConfirmButtonProps) {
  const [open, setOpen] = useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant={triggerVariant} size="sm" className="justify-start" />}>
        {triggerChildren}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>{cancelLabel}</DialogClose>
          <Button
            variant="destructive"
            disabled={pending}
            onClick={() => {
              onConfirm();
              setOpen(false);
            }}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function EditProfileDialog({ member }: { member: Member }) {
  const t = useTranslations("members");
  const [open, setOpen] = useState(false);
  const update = useUpdateMember(member.id);
  const [form, setForm] = useState({
    full_name: member.full_name ?? "",
    phone: member.phone ?? "",
    province: member.province ?? "",
    email: member.email ?? "",
  });

  const save = () => {
    // Send only changed, non-empty fields (sparse update — empties never blank data).
    const changed: Record<string, string> = {};
    if (form.full_name && form.full_name !== member.full_name) changed.full_name = form.full_name;
    if (form.phone && form.phone !== member.phone) changed.phone = form.phone;
    if (form.province && form.province !== member.province) changed.province = form.province;
    if (form.email && form.email !== member.email) changed.email = form.email;
    if (Object.keys(changed).length === 0) {
      toast.info(t("noChanges"));
      setOpen(false);
      return;
    }
    update.mutate(changed, {
      onSuccess: () => {
        toast.success(t("saved"));
        setOpen(false);
      },
      onError: (e) => toast.error(errMsg(e, t("saveFailed"))),
    });
  };

  const field = (label: string, key: keyof typeof form, type = "text") => (
    <label className="block space-y-1">
      <span className="text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">{label}</span>
      <Input
        type={type}
        value={form[key]}
        onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
      />
    </label>
  );

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="sm" className="justify-start" />}>
        <Pencil className="size-4" /> {t("editProfile")}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("editTitle")}</DialogTitle>
          <DialogDescription>{t("editDesc")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          {field(t("fieldFullName"), "full_name")}
          {field(t("fieldEmail"), "email", "email")}
          {field(t("fieldPhone"), "phone")}
          {field(t("fieldProvince"), "province")}
        </div>
        <DialogFooter>
          <DialogClose render={<Button variant="outline" />}>{t("cancel")}</DialogClose>
          <Button onClick={save} disabled={update.isPending}>
            {t("save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
