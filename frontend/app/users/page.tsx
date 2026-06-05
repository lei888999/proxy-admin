"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listUsers, listInbounds, createUser, updateUser, resetUserCreds, deleteUser,
  User, Inbound,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";

type Form = { id?: number; name: string; inboundIds: number[] };

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [form, setForm] = useState<Form | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<User | null>(null);

  const refresh = useCallback(() => {
    listUsers().then(setUsers).catch(() => setError("加载用户失败"));
  }, []);
  useEffect(() => {
    refresh();
    listInbounds().then(setInbounds).catch(() => {});
  }, [refresh]);

  function toggle(id: number) {
    if (!form) return;
    const has = form.inboundIds.includes(id);
    setForm({ ...form, inboundIds: has ? form.inboundIds.filter((x) => x !== id) : [...form.inboundIds, id] });
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError("");
    try {
      if (form.id) await updateUser(form.id, form.name, form.inboundIds);
      else await createUser(form.name, form.inboundIds);
      setForm(null);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function onReset(id: number) {
    await resetUserCreds(id);
    refresh();
  }
  async function doDelete() {
    if (!confirmDelete) return;
    await deleteUser(confirmDelete.id);
    setConfirmDelete(null);
    refresh();
  }

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">用户</h1>
        <Button className="rounded-full" onClick={() => { setError(""); setForm({ name: "", inboundIds: [] }); }}>
          新建用户
        </Button>
      </div>

      <Card className="rounded-lg">
        <CardContent className="pt-4">
          <div className="divide-y divide-border">
            <div className="grid grid-cols-[1fr_2fr_2fr_2fr_auto] gap-4 py-2 font-mono text-xs text-muted-foreground uppercase">
              <span>名称</span><span>UUID</span><span>密码</span><span>入站</span><span></span>
            </div>
            {users.map((u) => (
              <div key={u.id} className="grid grid-cols-[1fr_2fr_2fr_2fr_auto] items-center gap-4 py-3 text-sm">
                <span>{u.name}</span>
                <span className="truncate font-mono text-xs">{u.uuid}</span>
                <span className="truncate font-mono text-xs">{u.password}</span>
                <span className="truncate text-xs text-muted-foreground">{u.inboundTags.join(", ") || "—"}</span>
                <span className="flex gap-2">
                  <Button variant="outline" className="rounded-full" onClick={() => { setError(""); setForm({ id: u.id, name: u.name, inboundIds: u.inboundIds }); }}>编辑</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => onReset(u.id)}>重置凭证</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => setConfirmDelete(u)}>删除</Button>
                </span>
              </div>
            ))}
            {users.length === 0 && <p className="py-3 text-sm text-muted-foreground">暂无用户。</p>}
          </div>
        </CardContent>
      </Card>

      <Modal open={form !== null} onClose={() => setForm(null)} title={form?.id ? "编辑用户" : "新建用户"}>
        {form && (
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name" className="text-xs text-muted-foreground">名称</Label>
              <Input id="name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">所属入站</Label>
              <div className="space-y-1">
                {inbounds.map((ib) => (
                  <label key={ib.id} className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      aria-label={ib.tag}
                      checked={form.inboundIds.includes(ib.id)}
                      onChange={() => toggle(ib.id)}
                    />
                    {ib.tag} <span className="text-muted-foreground">· {ib.type}</span>
                  </label>
                ))}
                {inbounds.length === 0 && <p className="text-xs text-muted-foreground">还没有入站</p>}
              </div>
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="flex justify-end gap-3 pt-2">
              <Button type="button" variant="outline" className="rounded-full" onClick={() => setForm(null)}>取消</Button>
              <Button type="submit" className="rounded-full" disabled={busy}>{form.id ? "保存" : "创建"}</Button>
            </div>
          </form>
        )}
      </Modal>

      <Modal open={confirmDelete !== null} onClose={() => setConfirmDelete(null)} title="删除用户">
        <p className="mb-4 text-sm text-muted-foreground">
          确定删除用户 <span className="font-mono">{confirmDelete?.name}</span>？此操作不可撤销。
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="outline" className="rounded-full" onClick={() => setConfirmDelete(null)}>取消</Button>
          <Button className="rounded-full" onClick={doDelete}>确认删除</Button>
        </div>
      </Modal>
    </AppShell>
  );
}
