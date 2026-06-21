"use client";

// Super_admin console for local password accounts (the username/password sign-in
// path that runs alongside Entra SSO). List, provision, edit role/status, and
// reset passwords. The backend is the real gate; this UI mirrors it.
import { useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { KeyRound, Loader2, UserPlus } from "lucide-react";

import { useCreateHRUser, useHRUsers, useRbacRoles, useUpdateHRUser } from "@/lib/queries";
import type { CreateHRUserInput, HRUser, UpdateHRUserInput } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
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
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

function errMessage(e: unknown): string | null {
  return e instanceof Error ? e.message : null;
}

export function UserManagement() {
  const t = useTranslations("admin");
  const { data: users, isLoading } = useHRUsers(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<HRUser | null>(null);
  const shortRole = (role: string) => (t.has(`role_${role}`) ? t(`role_${role}`) : role);

  return (
    <section className="rounded-xl bg-card ring-1 ring-hairline">
      <header className="flex items-start justify-between gap-4 border-b border-hairline px-6 py-4">
        <div>
          <p className="eyebrow">{t("usersEyebrow")}</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">{t("usersTitle")}</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">{t("usersDesc")}</p>
        </div>
        <Button className="gap-2" onClick={() => setCreateOpen(true)}>
          <UserPlus className="size-4" />
          {t("addUser")}
        </Button>
      </header>

      <div className="px-2 py-2 sm:px-4">
        {isLoading ? (
          <div className="space-y-2 p-4">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
          </div>
        ) : !users || users.length === 0 ? (
          <p className="px-4 py-10 text-center text-sm text-muted-foreground">
            {t("usersEmpty")}
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("colUser")}</TableHead>
                <TableHead>{t("colRole")}</TableHead>
                <TableHead>{t("colStatus")}</TableHead>
                <TableHead>{t("colLastSignIn")}</TableHead>
                <TableHead className="text-right">{t("colManage")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((u) => (
                <TableRow key={u.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-foreground">{u.full_name || "-"}</span>
                      <Badge
                        variant="outline"
                        className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
                      >
                        {u.source === "sso" ? t("sourceSso") : t("sourceLocal")}
                      </Badge>
                    </div>
                    <div className="text-xs text-muted-foreground">{u.email}</div>
                  </TableCell>
                  <TableCell className="text-sm">{shortRole(u.role)}</TableCell>
                  <TableCell>
                    {u.is_active ? (
                      <Badge variant="secondary">{t("statusActive")}</Badge>
                    ) : (
                      <Badge variant="outline" className="text-muted-foreground">
                        {t("statusDisabled")}
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {u.last_login_at ? new Date(u.last_login_at).toLocaleDateString() : t("never")}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button variant="ghost" size="sm" onClick={() => setEditing(u)}>
                      {t("edit")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <CreateUserDialog open={createOpen} onClose={() => setCreateOpen(false)} />
      <EditUserDialog user={editing} onClose={() => setEditing(null)} />
    </section>
  );
}

// --- Create ----------------------------------------------------------------

function CreateUserDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const t = useTranslations("admin");
  const create = useCreateHRUser();
  const [form, setForm] = useState<CreateHRUserInput>({
    email: "",
    full_name: "",
    role: "hr_staff",
    password: "",
  });

  function reset() {
    setForm({ email: "", full_name: "", role: "hr_staff", password: "" });
    create.reset();
  }

  function close() {
    reset();
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    await create.mutateAsync(
      {
        ...form,
        email: form.email.trim(),
        full_name: form.full_name.trim(),
        store_id: form.store_id ? Number(form.store_id) : undefined,
        subregion: form.subregion?.trim() || undefined,
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("createTitle")}</DialogTitle>
          <DialogDescription>{t("createDesc")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3" noValidate>
          <Field label={t("fieldFullName")}>
            <Input
              value={form.full_name}
              onChange={(e) => setForm({ ...form, full_name: e.target.value })}
              placeholder={t("fullNamePlaceholder")}
              required
            />
          </Field>
          <Field label={t("fieldEmail")}>
            <Input
              type="email"
              value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
              placeholder={t("emailPlaceholder")}
              required
            />
          </Field>
          <Field label={t("fieldRole")}>
            <RoleSelect value={form.role} onChange={(role) => setForm({ ...form, role })} />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label={t("fieldStore")}>
              <Input
                type="number"
                value={form.store_id ?? ""}
                onChange={(e) =>
                  setForm({ ...form, store_id: e.target.value ? Number(e.target.value) : undefined })
                }
                placeholder={t("storePlaceholder")}
              />
            </Field>
            <Field label={t("fieldSubregion")}>
              <Input
                value={form.subregion ?? ""}
                onChange={(e) => setForm({ ...form, subregion: e.target.value })}
                placeholder={t("subregionPlaceholder")}
              />
            </Field>
          </div>
          <Field label={t("fieldTempPassword")}>
            <Input
              type="password"
              value={form.password}
              onChange={(e) => setForm({ ...form, password: e.target.value })}
              placeholder={t("tempPasswordPlaceholder")}
              autoComplete="new-password"
              required
            />
          </Field>

          {create.isError && (
            <p role="alert" className="text-xs font-medium text-destructive">
              {errMessage(create.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={create.isPending} className="gap-2">
              {create.isPending && <Loader2 className="size-4 animate-spin" />}
              {t("createAccount")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// --- Edit ------------------------------------------------------------------

function EditUserDialog({ user, onClose }: { user: HRUser | null; onClose: () => void }) {
  const t = useTranslations("admin");
  const update = useUpdateHRUser();
  const [fullName, setFullName] = useState("");
  const [role, setRole] = useState("hr_staff");
  const [storeId, setStoreId] = useState<string>("");
  const [subregion, setSubregion] = useState("");
  const [active, setActive] = useState(true);
  const [newPassword, setNewPassword] = useState("");
  const [hydratedFor, setHydratedFor] = useState<string | null>(null);

  // Hydrate the form when a different user is opened (no effect needed: derive
  // from the prop and reset on identity change).
  if (user && hydratedFor !== user.id) {
    setHydratedFor(user.id);
    setFullName(user.full_name);
    setRole(user.role);
    setStoreId(user.store_id != null ? String(user.store_id) : "");
    setSubregion(user.subregion ?? "");
    setActive(user.is_active);
    setNewPassword("");
    update.reset();
  }

  function close() {
    setHydratedFor(null);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!user) return;
    const input: UpdateHRUserInput = {
      full_name: fullName.trim(),
      role,
      is_active: active,
    };
    if (storeId.trim()) input.store_id = Number(storeId);
    if (subregion.trim()) input.subregion = subregion.trim();
    if (newPassword) input.password = newPassword;
    await update.mutateAsync({ id: user.id, input }, { onSuccess: close });
  }

  return (
    <Dialog open={!!user} onOpenChange={(o) => (o ? null : close())}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("editTitle", { email: user?.email ?? "" })}</DialogTitle>
          <DialogDescription>{t("editDesc")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3" noValidate>
          <Field label={t("fieldFullName")}>
            <Input value={fullName} onChange={(e) => setFullName(e.target.value)} required />
          </Field>
          <Field label={t("fieldRole")}>
            <RoleSelect value={role} onChange={setRole} />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label={t("fieldStore")}>
              <Input
                type="number"
                value={storeId}
                onChange={(e) => setStoreId(e.target.value)}
                placeholder={t("storePlaceholder")}
              />
            </Field>
            <Field label={t("fieldSubregion")}>
              <Input
                value={subregion}
                onChange={(e) => setSubregion(e.target.value)}
                placeholder={t("subregionPlaceholder")}
              />
            </Field>
          </div>
          <div className="flex items-center justify-between rounded-lg border border-hairline px-3 py-2.5">
            <div>
              <p className="text-sm font-medium text-foreground">{t("accountActive")}</p>
              <p className="text-xs text-muted-foreground">{t("accountActiveHelp")}</p>
            </div>
            <Switch checked={active} onCheckedChange={setActive} />
          </div>
          {/* SSO accounts have no password; password reset applies to local accounts only. */}
          {user?.source !== "sso" && (
            <Field label={t("fieldResetPassword")}>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder={t("resetPasswordPlaceholder")}
                autoComplete="new-password"
              />
            </Field>
          )}

          {update.isError && (
            <p role="alert" className="text-xs font-medium text-destructive">
              {errMessage(update.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="ghost" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={update.isPending} className="gap-2">
              {update.isPending ? <Loader2 className="size-4 animate-spin" /> : <KeyRound className="size-4" />}
              {t("saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// --- shared bits -----------------------------------------------------------

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-foreground">{label}</span>
      {children}
    </label>
  );
}

// RoleSelect offers the live role list from the dynamic-RBAC matrix (useRbacRoles)
// so a newly created role is immediately assignable. Built-in roles keep their
// curated i18n label (roleFull_*); custom roles fall back to the locale label.
function RoleSelect({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const t = useTranslations("admin");
  const locale = useLocale();
  const { data: roles } = useRbacRoles();
  const label = (r: { key: string; label_en: string; label_th: string }) =>
    t.has(`roleFull_${r.key}`) ? t(`roleFull_${r.key}`) : locale === "th" ? r.label_th : r.label_en;
  return (
    <Select value={value} onValueChange={(v) => onChange(v ?? value)}>
      <SelectTrigger className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {(roles ?? []).map((r) => (
          <SelectItem key={r.key} value={r.key}>
            {label(r)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
