"use client";

// Super_admin console for dynamic RBAC: edit the role→permission matrix and the
// per-role data scope that the backend enforces. The backend is the real gate;
// this UI mirrors rbac_roles / rbac_role_permissions. Permissions are a fixed code
// catalog (read-only here); roles + their grants + scope are data-driven and
// editable. The super_admin role is a hard code bypass and is shown locked.
import { useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Loader2, Lock, Plus, ShieldCheck, Trash2 } from "lucide-react";

import {
  useCreateRole,
  useDeleteRole,
  useRbacPermissions,
  useRbacRoles,
  useUpdateRole,
} from "@/lib/queries";
import type { RbacPermission, RbacRole, RbacRoleInput } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const SUPER_ADMIN = "super_admin";
const SCOPE_KINDS = ["all", "subregion", "store"] as const;
// Stable category ordering for the permission matrix; unknown categories fall to
// the end (in encounter order) so a new backend category still renders.
const CATEGORY_ORDER = ["system", "reporting", "operations", "candidates", "hiring", "approvals"];

function errMessage(e: unknown): string | null {
  return e instanceof Error ? e.message : null;
}

export function RolesPermissions() {
  const t = useTranslations("admin");
  const locale = useLocale();
  const { data: roles, isLoading: rolesLoading } = useRbacRoles();
  const { data: permissions, isLoading: permsLoading } = useRbacPermissions();
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<RbacRole | null>(null);

  const roleLabel = (r: RbacRole) =>
    t.has(`role_${r.key}`) ? t(`role_${r.key}`) : locale === "th" ? r.label_th : r.label_en;
  const scopeLabel = (kind: string) => (t.has(`scope_${kind}`) ? t(`scope_${kind}`) : kind);
  const isLoading = rolesLoading || permsLoading;

  return (
    <section className="rounded-xl bg-card ring-1 ring-hairline">
      <header className="flex items-start justify-between gap-4 border-b border-hairline px-6 py-4">
        <div>
          <p className="eyebrow">{t("rbacEyebrow")}</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">{t("rbacTitle")}</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">{t("rbacDesc")}</p>
        </div>
        <Button className="gap-2" onClick={() => setCreateOpen(true)}>
          <Plus className="size-4" />
          {t("addRole")}
        </Button>
      </header>

      <div className="px-2 py-2 sm:px-4">
        {isLoading ? (
          <div className="space-y-2 p-4">
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
            <Skeleton className="h-9 w-full" />
          </div>
        ) : !roles || roles.length === 0 ? (
          <p className="px-4 py-10 text-center text-sm text-muted-foreground">{t("rolesEmpty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("colRole")}</TableHead>
                <TableHead>{t("colScope")}</TableHead>
                <TableHead>{t("colPermissions")}</TableHead>
                <TableHead className="text-right">{t("colManage")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {roles.map((r) => {
                const locked = r.key === SUPER_ADMIN;
                return (
                  <TableRow key={r.key}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-foreground">{roleLabel(r)}</span>
                        {r.is_builtin && (
                          <Badge variant="outline" className="text-muted-foreground">
                            {t("builtinBadge")}
                          </Badge>
                        )}
                      </div>
                      <div className="font-mono text-xs text-muted-foreground">{r.key}</div>
                    </TableCell>
                    <TableCell className="text-sm">{scopeLabel(r.scope_kind)}</TableCell>
                    <TableCell className="text-sm text-muted-foreground">
                      {locked ? (
                        <span className="inline-flex items-center gap-1.5 text-foreground">
                          <ShieldCheck className="size-4 text-brand" />
                          {t("allPermissions")}
                        </span>
                      ) : (
                        t("permissionCount", { count: r.permissions.length })
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      {locked ? (
                        <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                          <Lock className="size-3.5" />
                          {t("locked")}
                        </span>
                      ) : (
                        <Button variant="ghost" size="sm" onClick={() => setEditing(r)}>
                          {t("edit")}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        )}
      </div>

      <RoleDialog
        mode="create"
        open={createOpen}
        permissions={permissions ?? []}
        onClose={() => setCreateOpen(false)}
      />
      <RoleDialog
        mode="edit"
        role={editing}
        open={!!editing}
        permissions={permissions ?? []}
        onClose={() => setEditing(null)}
      />
    </section>
  );
}

// --- Create / Edit dialog ---------------------------------------------------

type RoleDialogProps = {
  open: boolean;
  permissions: RbacPermission[];
  onClose: () => void;
} & ({ mode: "create"; role?: undefined } | { mode: "edit"; role: RbacRole | null });

function RoleDialog({ mode, role, open, permissions, onClose }: RoleDialogProps) {
  const t = useTranslations("admin");
  const create = useCreateRole();
  const update = useUpdateRole();
  const del = useDeleteRole();

  const [key, setKey] = useState("");
  const [labelEn, setLabelEn] = useState("");
  const [labelTh, setLabelTh] = useState("");
  const [scope, setScope] = useState<string>("store");
  const [perms, setPerms] = useState<Set<string>>(new Set());
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [hydratedFor, setHydratedFor] = useState<string | null>(null);

  // Hydrate from the edited role on identity change (no effect needed: derive from
  // the prop). The create form resets to blanks on each open.
  const identity = mode === "edit" ? (role?.key ?? null) : open ? "__create__" : null;
  if (identity !== hydratedFor) {
    setHydratedFor(identity);
    setConfirmDelete(false);
    create.reset();
    update.reset();
    del.reset();
    if (mode === "edit" && role) {
      setKey(role.key);
      setLabelEn(role.label_en);
      setLabelTh(role.label_th);
      setScope(role.scope_kind);
      setPerms(new Set(role.permissions));
    } else {
      setKey("");
      setLabelEn("");
      setLabelTh("");
      setScope("store");
      setPerms(new Set());
    }
  }

  const isBuiltin = mode === "edit" && !!role?.is_builtin;
  const busy = create.isPending || update.isPending || del.isPending;
  const activeError = errMessage(create.error) || errMessage(update.error) || errMessage(del.error);

  // Group permissions by category, ordered by CATEGORY_ORDER then encounter order,
  // each group sorted by the catalog `sort`.
  const grouped = useMemo(() => groupByCategory(permissions), [permissions]);

  function toggle(permKey: string, on: boolean) {
    setPerms((prev) => {
      const next = new Set(prev);
      if (on) next.add(permKey);
      else next.delete(permKey);
      return next;
    });
  }

  function close() {
    setHydratedFor(null);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const permissions = [...perms];
    if (mode === "create") {
      const input: RbacRoleInput = {
        key: key.trim(),
        label_en: labelEn.trim(),
        label_th: labelTh.trim(),
        scope_kind: scope,
        permissions,
      };
      await create.mutateAsync(input, { onSuccess: close });
    } else if (role) {
      const input: RbacRoleInput = {
        label_en: labelEn.trim(),
        label_th: labelTh.trim(),
        scope_kind: scope,
        permissions,
      };
      await update.mutateAsync({ key: role.key, input }, { onSuccess: close });
    }
  }

  async function remove() {
    if (mode !== "edit" || !role) return;
    await del.mutateAsync(role.key, { onSuccess: close });
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : close())}>
      <DialogContent className="max-h-[88vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {mode === "create" ? t("createRoleTitle") : t("editRoleTitle", { role: labelEn || key })}
          </DialogTitle>
          <DialogDescription>
            {mode === "create" ? t("createRoleDesc") : t("editRoleDesc")}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-4" noValidate>
          {mode === "create" && (
            <Field label={t("fieldRoleKey")} hint={t("roleKeyHint")}>
              <Input
                value={key}
                onChange={(e) => setKey(e.target.value)}
                placeholder={t("roleKeyPlaceholder")}
                autoComplete="off"
                required
              />
            </Field>
          )}

          {/* Built-in role labels come from the i18n catalog; only custom roles
              expose editable labels. */}
          {!isBuiltin && (
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("fieldLabelEn")}>
                <Input value={labelEn} onChange={(e) => setLabelEn(e.target.value)} required />
              </Field>
              <Field label={t("fieldLabelTh")}>
                <Input value={labelTh} onChange={(e) => setLabelTh(e.target.value)} required />
              </Field>
            </div>
          )}

          <Field label={t("fieldScope")} hint={t("scopeHint")}>
            <Select value={scope} onValueChange={(v) => setScope(v ?? scope)}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SCOPE_KINDS.map((k) => (
                  <SelectItem key={k} value={k}>
                    {t(`scope_${k}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>

          <fieldset className="space-y-3">
            <legend className="text-xs font-medium text-foreground">{t("fieldPermissions")}</legend>
            {grouped.map(([category, perms_]) => (
              <PermGroup
                key={category}
                category={category}
                permissions={perms_}
                selected={perms}
                onToggle={toggle}
              />
            ))}
          </fieldset>

          {activeError && (
            <p role="alert" className="text-xs font-medium text-destructive">
              {activeError}
            </p>
          )}

          <DialogFooter className="flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
            {/* Delete (custom roles only) — inline confirm to avoid a second dialog. */}
            <div>
              {mode === "edit" && !isBuiltin && role ? (
                confirmDelete ? (
                  <div className="flex items-center gap-2 text-xs">
                    <span className="text-muted-foreground">{t("deleteRoleConfirm")}</span>
                    <Button type="button" variant="destructive" size="sm" disabled={busy} onClick={remove}>
                      {del.isPending && <Loader2 className="size-3.5 animate-spin" />}
                      {t("deleteRoleYes")}
                    </Button>
                    <Button type="button" variant="ghost" size="sm" onClick={() => setConfirmDelete(false)}>
                      {t("cancel")}
                    </Button>
                  </div>
                ) : (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="gap-1.5 text-destructive hover:text-destructive"
                    onClick={() => setConfirmDelete(true)}
                  >
                    <Trash2 className="size-3.5" />
                    {t("deleteRole")}
                  </Button>
                )
              ) : (
                <span />
              )}
            </div>
            <div className="flex justify-end gap-2">
              <Button type="button" variant="ghost" onClick={close}>
                {t("cancel")}
              </Button>
              <Button type="submit" disabled={busy} className="gap-2">
                {(create.isPending || update.isPending) && <Loader2 className="size-4 animate-spin" />}
                {mode === "create" ? t("createRoleBtn") : t("saveRole")}
              </Button>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

// --- Permission group (one category) ----------------------------------------

function PermGroup({
  category,
  permissions,
  selected,
  onToggle,
}: {
  category: string;
  permissions: RbacPermission[];
  selected: Set<string>;
  onToggle: (key: string, on: boolean) => void;
}) {
  const t = useTranslations("admin");
  const locale = useLocale();
  const catLabel = t.has(`cat_${category}`) ? t(`cat_${category}`) : category;

  return (
    <div className="rounded-lg border border-hairline">
      <p className="border-b border-hairline px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {catLabel}
      </p>
      <div className="grid gap-x-4 gap-y-2.5 p-3 sm:grid-cols-2">
        {permissions.map((p) => {
          const label = locale === "th" ? p.label_th : p.label_en;
          return (
            <label key={p.key} className="flex cursor-pointer items-start gap-2.5 text-sm">
              <Checkbox
                checked={selected.has(p.key)}
                onCheckedChange={(on) => onToggle(p.key, !!on)}
                className="mt-0.5"
              />
              <span className="leading-tight text-foreground">{label}</span>
            </label>
          );
        })}
      </div>
    </div>
  );
}

// --- helpers ----------------------------------------------------------------

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-foreground">{label}</span>
      {children}
      {hint && <span className="block text-xs text-muted-foreground">{hint}</span>}
    </label>
  );
}

// groupByCategory returns [category, permissions][] ordered by CATEGORY_ORDER then
// first-seen order, each group sorted by the catalog `sort` field.
function groupByCategory(permissions: RbacPermission[]): [string, RbacPermission[]][] {
  const byCat = new Map<string, RbacPermission[]>();
  for (const p of permissions) {
    const list = byCat.get(p.category) ?? [];
    list.push(p);
    byCat.set(p.category, list);
  }
  const order = (c: string) => {
    const i = CATEGORY_ORDER.indexOf(c);
    return i === -1 ? CATEGORY_ORDER.length : i;
  };
  return [...byCat.entries()]
    .sort((a, b) => order(a[0]) - order(b[0]))
    .map(([cat, list]) => [cat, [...list].sort((a, b) => a.sort - b.sort)] as [string, RbacPermission[]]);
}
