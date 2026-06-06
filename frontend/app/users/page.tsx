"use client";

import { useState } from "react";
import {
  createUser, updateUser, resetUserCreds, deleteUser, resetUserTraffic, User,
} from "@/lib/api";
import { useUsers, useInbounds, useOutbounds } from "@/lib/hooks";
import { formatBytes, copyText } from "@/lib/utils";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/components/ui/table";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Copy, Pencil, KeyRound, RotateCcw, Trash2, Users as UsersIcon, Plus } from "lucide-react";

type Form = { id?: number; name: string; inboundIds: number[]; outboundId: number | null };

export default function UsersPage() {
  const { data: users = [], mutate: refresh } = useUsers();
  const { data: inbounds = [] } = useInbounds();
  const { data: outbounds = [] } = useOutbounds();
  const [form, setForm] = useState<Form | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState<User | null>(null);
  const [confirmResetTraffic, setConfirmResetTraffic] = useState<User | null>(null);

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
      if (form.id) await updateUser(form.id, form.name, form.inboundIds, form.outboundId);
      else await createUser(form.name, form.inboundIds, form.outboundId);
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
  async function doResetTraffic() {
    if (!confirmResetTraffic) return;
    await resetUserTraffic(confirmResetTraffic.id);
    setConfirmResetTraffic(null);
    refresh();
  }

  function copySub(token: string) {
    copyText(`${window.location.origin}/sub/${token}`);
  }

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">用户</h1>
        <Button className="h-9 rounded-full px-5" onClick={() => { setError(""); setForm({ name: "", inboundIds: [], outboundId: null }); }}>
          <Plus />新建用户
        </Button>
      </div>

      <Card className="overflow-hidden rounded-lg p-0">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent [&>th]:font-mono [&>th]:text-xs [&>th]:font-normal [&>th]:tracking-wider [&>th]:text-muted-foreground [&>th]:uppercase">
              <TableHead>名称</TableHead>
              <TableHead>UUID</TableHead>
              <TableHead>密码</TableHead>
              <TableHead>入站</TableHead>
              <TableHead>流量</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {users.map((u) => (
              <TableRow key={u.id}>
                <TableCell className="font-medium">{u.name}</TableCell>
                <TableCell className="max-w-[12rem] truncate font-mono text-xs text-muted-foreground">{u.uuid}</TableCell>
                <TableCell className="max-w-[10rem] truncate font-mono text-xs text-muted-foreground">{u.password}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{u.inboundTags.join(", ") || "—"}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground whitespace-nowrap">
                  ↑{formatBytes(u.upBytes)} ↓{formatBytes(u.downBytes)}
                </TableCell>
                <TableCell>
                  <div className="flex items-center justify-end gap-0.5">
                    <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label="订阅" title="复制订阅链接" onClick={() => copySub(u.subToken)}>
                      <Copy />
                    </Button>
                    <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label="编辑" title="编辑" onClick={() => { setError(""); setForm({ id: u.id, name: u.name, inboundIds: u.inboundIds, outboundId: u.outboundId }); }}>
                      <Pencil />
                    </Button>
                    <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label="重置凭证" title="重置凭证（轮换 UUID/密码/订阅 token）" onClick={() => onReset(u.id)}>
                      <KeyRound />
                    </Button>
                    <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label="重置流量" title="重置累计流量" onClick={() => setConfirmResetTraffic(u)}>
                      <RotateCcw />
                    </Button>
                    <Button variant="ghost" size="icon" aria-label="删除" title="删除" className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive" onClick={() => setConfirmDelete(u)}>
                      <Trash2 />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {users.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={6} className="p-0">
                  <EmptyState
                    icon={UsersIcon}
                    title="还没有用户"
                    description="创建用户后会生成 UUID / 密码与订阅链接。"
                    action={
                      <Button className="rounded-full" onClick={() => { setError(""); setForm({ name: "", inboundIds: [], outboundId: null }); }}>
                        添加用户
                      </Button>
                    }
                  />
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
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
              <div className="space-y-1.5">
                {inbounds.map((ib) => {
                  const checked = form.inboundIds.includes(ib.id);
                  return (
                    <label
                      key={ib.id}
                      className={`flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2.5 text-sm transition-colors ${
                        checked
                          ? "border-primary/40 bg-secondary"
                          : "border-border hover:bg-secondary/50"
                      }`}
                    >
                      <Checkbox aria-label={ib.tag} checked={checked} onCheckedChange={() => toggle(ib.id)} />
                      <span>{ib.tag}</span>
                      <span className="font-mono text-xs text-muted-foreground">{ib.type}</span>
                    </label>
                  );
                })}
                {inbounds.length === 0 && <p className="text-xs text-muted-foreground">还没有入站</p>}
              </div>
            </div>
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">出站</Label>
              <Select
                items={{
                  direct: "直连",
                  ...Object.fromEntries(outbounds.map((o) => [String(o.id), `${o.tag} · ${o.type}`])),
                }}
                value={form.outboundId == null ? "direct" : String(form.outboundId)}
                onValueChange={(v) => setForm({ ...form, outboundId: v === "direct" ? null : Number(v) })}
              >
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="direct">直连</SelectItem>
                  {outbounds.map((o) => (
                    <SelectItem key={o.id} value={String(o.id)}>{o.tag} · {o.type}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
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

      <Modal open={confirmResetTraffic !== null} onClose={() => setConfirmResetTraffic(null)} title="重置流量">
        <p className="mb-4 text-sm text-muted-foreground">
          确定清零用户 <span className="font-mono">{confirmResetTraffic?.name}</span> 的累计流量？
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="outline" className="rounded-full" onClick={() => setConfirmResetTraffic(null)}>取消</Button>
          <Button className="rounded-full" onClick={doResetTraffic}>确认重置</Button>
        </div>
      </Modal>
    </AppShell>
  );
}
