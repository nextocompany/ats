"use client";

// Area management — dynamic store groupings backing the area visibility scope.
// Create areas, assign stores, and assign the area_hr users who cover them.
// Gated to area.admin (mirrors the backend).
import { useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { Loader2, MapPin, Plus, ShieldAlert, Trash2, Users } from "lucide-react";
import { toast } from "sonner";

import { PageHeader } from "@/components/shell/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import {
  useAreas,
  useArea,
  useCreateArea,
  useDeleteArea,
  useHRUsers,
  useMe,
  useSetAreaMembers,
  useSetAreaStores,
  useStores,
  useUpdateArea,
} from "@/lib/queries";
import { canManageAreas } from "@/lib/roles";

export default function AreasPage() {
  const t = useTranslations("areas");
  const { data: me, isLoading: meLoading } = useMe();
  const allowed = canManageAreas(me);

  const { data: areas, isLoading } = useAreas(allowed);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [newName, setNewName] = useState("");

  const createArea = useCreateArea();
  const deleteArea = useDeleteArea();

  if (meLoading) return <Skeleton className="h-64 w-full" />;
  if (!allowed) {
    return (
      <div className="flex flex-col items-center gap-3 py-24 text-center text-muted-foreground">
        <ShieldAlert className="h-8 w-8" />
        <p>{t("denied")}</p>
      </div>
    );
  }

  async function onCreate() {
    const name = newName.trim();
    if (!name) return;
    try {
      const a = await createArea.mutateAsync(name);
      setNewName("");
      setSelectedId(a.id);
      toast.success(t("created"));
    } catch {
      toast.error(t("createFailed"));
    }
  }

  async function onDelete(id: string) {
    if (!confirm(t("confirmDelete"))) return;
    try {
      await deleteArea.mutateAsync(id);
      if (selectedId === id) setSelectedId(null);
      toast.success(t("deleted"));
    } catch {
      toast.error(t("deleteFailed"));
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("subtitle")} />

      <div className="grid gap-6 lg:grid-cols-[320px_1fr]">
        {/* Left: list + create */}
        <div className="space-y-4">
          <div className="flex gap-2">
            <Input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder={t("namePlaceholder")}
              onKeyDown={(e) => e.key === "Enter" && onCreate()}
            />
            <Button onClick={onCreate} disabled={createArea.isPending || !newName.trim()}>
              {createArea.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
            </Button>
          </div>

          {isLoading ? (
            <Skeleton className="h-48 w-full" />
          ) : (
            <ul className="space-y-1">
              {(areas ?? []).map((a) => (
                <li key={a.id}>
                  <button
                    onClick={() => setSelectedId(a.id)}
                    className={`flex w-full items-center justify-between rounded-md border px-3 py-2 text-left text-sm transition ${
                      selectedId === a.id ? "border-primary bg-accent-soft" : "border-border hover:bg-muted"
                    }`}
                  >
                    <span className="flex items-center gap-2">
                      <span className={a.active ? "" : "text-muted-foreground line-through"}>{a.name}</span>
                    </span>
                    <span className="text-xs text-muted-foreground">{t("storeCount", { n: a.store_count })}</span>
                  </button>
                </li>
              ))}
              {(areas ?? []).length === 0 && <p className="px-1 py-4 text-sm text-muted-foreground">{t("empty")}</p>}
            </ul>
          )}
        </div>

        {/* Right: editor */}
        <div>
          {selectedId ? (
            <AreaEditor areaId={selectedId} onDelete={() => onDelete(selectedId)} />
          ) : (
            <div className="flex h-full min-h-48 items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
              {t("selectPrompt")}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function AreaEditor({ areaId, onDelete }: { areaId: string; onDelete: () => void }) {
  const t = useTranslations("areas");
  const { data: area, isLoading } = useArea(areaId);
  const { data: stores } = useStores();
  const { data: users } = useHRUsers(true);

  const update = useUpdateArea();
  const setStores = useSetAreaStores();
  const setMembers = useSetAreaMembers();

  const [name, setName] = useState("");
  const [storeSel, setStoreSel] = useState<Set<number>>(new Set());
  const [memberSel, setMemberSel] = useState<Set<string>>(new Set());
  const [storeFilter, setStoreFilter] = useState("");

  // Hydrate local edit state from the loaded area using the render-phase "reset
  // state when a prop changes" pattern (not useEffect — avoids cascading renders).
  const [hydratedId, setHydratedId] = useState<string | null>(null);
  if (area && hydratedId !== area.id) {
    setName(area.name);
    setStoreSel(new Set(area.store_nos ?? []));
    setMemberSel(new Set(area.member_ids ?? []));
    setHydratedId(area.id);
  }

  // area_hr is the role that covers areas; offer those (plus anyone already a member).
  const candidateUsers = useMemo(
    () => (users ?? []).filter((u) => u.is_active && (u.role === "area_hr" || memberSel.has(u.id))),
    [users, memberSel],
  );
  const filteredStores = useMemo(() => {
    const q = storeFilter.trim().toLowerCase();
    return (stores ?? []).filter(
      (s) => !q || s.store_name.toLowerCase().includes(q) || String(s.store_no).includes(q),
    );
  }, [stores, storeFilter]);

  if (isLoading || !area) return <Skeleton className="h-96 w-full" />;

  async function saveName() {
    try {
      await update.mutateAsync({ id: areaId, name: name.trim() });
      toast.success(t("saved"));
    } catch {
      toast.error(t("saveFailed"));
    }
  }
  async function toggleActive(next: boolean) {
    try {
      await update.mutateAsync({ id: areaId, active: next });
    } catch {
      toast.error(t("saveFailed"));
    }
  }
  async function saveStores() {
    try {
      await setStores.mutateAsync({ id: areaId, storeNos: [...storeSel] });
      toast.success(t("storesSaved"));
    } catch {
      toast.error(t("saveFailed"));
    }
  }
  async function saveMembers() {
    try {
      await setMembers.mutateAsync({ id: areaId, userIds: [...memberSel] });
      toast.success(t("membersSaved"));
    } catch {
      toast.error(t("saveFailed"));
    }
  }

  return (
    <div className="space-y-6 rounded-lg border p-5">
      {/* Identity */}
      <div className="flex flex-wrap items-center gap-3">
        <Input value={name} onChange={(e) => setName(e.target.value)} className="max-w-xs" />
        <Button variant="outline" onClick={saveName} disabled={update.isPending || !name.trim()}>
          {t("rename")}
        </Button>
        <label className="flex items-center gap-2 text-sm">
          <Switch checked={area.active} onCheckedChange={toggleActive} />
          {t("active")}
        </label>
        <Button variant="ghost" className="ml-auto text-destructive" onClick={onDelete}>
          <Trash2 className="mr-1 h-4 w-4" /> {t("delete")}
        </Button>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {/* Stores */}
        <section className="space-y-3">
          <h3 className="flex items-center gap-2 text-sm font-semibold">
            <MapPin className="h-4 w-4 text-primary" /> {t("stores")} ({storeSel.size})
          </h3>
          <Input value={storeFilter} onChange={(e) => setStoreFilter(e.target.value)} placeholder={t("filterStores")} />
          <div className="max-h-72 space-y-1 overflow-y-auto rounded-md border p-2">
            {filteredStores.map((s) => (
              <label key={s.store_no} className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-muted">
                <input
                  type="checkbox"
                  checked={storeSel.has(s.store_no)}
                  onChange={(e) => {
                    const next = new Set(storeSel);
                    if (e.target.checked) next.add(s.store_no);
                    else next.delete(s.store_no);
                    setStoreSel(next);
                  }}
                />
                <span className="font-mono text-xs text-muted-foreground">{s.store_no}</span>
                <span>{s.store_name}</span>
              </label>
            ))}
          </div>
          <Button onClick={saveStores} disabled={setStores.isPending}>
            {setStores.isPending ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : null}
            {t("saveStores")}
          </Button>
        </section>

        {/* Members (area_hr) */}
        <section className="space-y-3">
          <h3 className="flex items-center gap-2 text-sm font-semibold">
            <Users className="h-4 w-4 text-primary" /> {t("members")} ({memberSel.size})
          </h3>
          <div className="max-h-72 space-y-1 overflow-y-auto rounded-md border p-2">
            {candidateUsers.map((u) => (
              <label key={u.id} className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-muted">
                <input
                  type="checkbox"
                  checked={memberSel.has(u.id)}
                  onChange={(e) => {
                    const next = new Set(memberSel);
                    if (e.target.checked) next.add(u.id);
                    else next.delete(u.id);
                    setMemberSel(next);
                  }}
                />
                <span>{u.full_name || u.email}</span>
              </label>
            ))}
            {candidateUsers.length === 0 && <p className="px-2 py-3 text-sm text-muted-foreground">{t("noAreaHr")}</p>}
          </div>
          <Button onClick={saveMembers} disabled={setMembers.isPending}>
            {setMembers.isPending ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : null}
            {t("saveMembers")}
          </Button>
        </section>
      </div>
    </div>
  );
}
