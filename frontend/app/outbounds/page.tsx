"use client";

import { useState } from "react";
import { createOutbound, updateOutbound, deleteOutbound, Outbound } from "@/lib/api";
import { useOutbounds } from "@/lib/hooks";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card } from "@/components/ui/card";
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { Pencil, Trash2, ArrowUpFromLine, Plus } from "lucide-react";

type Form = { id?: number; type: string; tag: string; server: string; port: string; username: string; password: string };
const empty: Form = { type: "socks5", tag: "", server: "", port: "1080", username: "", password: "" };

export default function OutboundsPage() {
  const { data: outbounds = [], mutate: refresh } = useOutbounds();
  const [form, setForm] = useState<Form | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Outbound | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError("");
    const payload = {
      type: form.type, tag: form.tag, server: form.server,
      port: Number(form.port), username: form.username, password: form.password,
    };
    try {
      if (form.id) await updateOutbound(form.id, payload);
      else await createOutbound(payload);
      setForm(null);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function doDelete() {
    if (!confirmDelete) return;
    await deleteOutbound(confirmDelete.id);
    setConfirmDelete(null);
    refresh();
  }

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">出站</h1>
        <Button className="h-9 rounded-full px-5" onClick={() => { setError(""); setForm({ ...empty }); }}><Plus />新建出站</Button>
      </div>

      <Card className="overflow-hidden rounded-lg p-0">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent [&>th]:font-mono [&>th]:text-xs [&>th]:font-normal [&>th]:tracking-wider [&>th]:text-muted-foreground [&>th]:uppercase">
              <TableHead>标签</TableHead>
              <TableHead>类型</TableHead>
              <TableHead>服务器</TableHead>
              <TableHead>端口</TableHead>
              <TableHead>用户名</TableHead>
              <TableHead className="text-right">操作</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {outbounds.map((o) => (
              <TableRow key={o.id}>
                <TableCell className="font-medium">{o.tag}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{o.type}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{o.server}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">{o.port}</TableCell>
                <TableCell className="text-xs text-muted-foreground">{o.username || "—"}</TableCell>
                <TableCell>
                  <div className="flex items-center justify-end gap-0.5">
                    <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label="编辑" onClick={() => { setError(""); setForm({ id: o.id, type: o.type, tag: o.tag, server: o.server, port: String(o.port), username: o.username, password: o.password }); }}>
                      <Pencil />
                    </Button>
                    <Button variant="ghost" size="icon" aria-label="删除" className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive" onClick={() => setConfirmDelete(o)}>
                      <Trash2 />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {outbounds.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={6} className="p-0">
                  <EmptyState
                    icon={ArrowUpFromLine}
                    title="还没有出站"
                    description="出站用于把流量转发到上游节点；不配置时用户默认直连。"
                    action={<Button className="rounded-full" onClick={() => { setError(""); setForm({ ...empty }); }}>添加出站</Button>}
                  />
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Modal open={form !== null} onClose={() => setForm(null)} title={form?.id ? "编辑出站" : "新建出站"}>
        {form && (
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">类型</Label>
              <Select items={{ socks5: "SOCKS5", http: "HTTP" }} value={form.type} onValueChange={(v) => v && setForm({ ...form, type: v })}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="socks5">SOCKS5</SelectItem>
                  <SelectItem value="http">HTTP</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex gap-4">
              <div className="flex-1 space-y-2">
                <Label htmlFor="tag" className="text-xs text-muted-foreground">标签</Label>
                <Input id="tag" value={form.tag} onChange={(e) => setForm({ ...form, tag: e.target.value })} />
              </div>
              <div className="w-28 space-y-2">
                <Label htmlFor="port" className="text-xs text-muted-foreground">端口</Label>
                <Input id="port" value={form.port} onChange={(e) => setForm({ ...form, port: e.target.value })} />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="server" className="text-xs text-muted-foreground">服务器</Label>
              <Input id="server" value={form.server} onChange={(e) => setForm({ ...form, server: e.target.value })} />
            </div>
            <div className="flex gap-4">
              <div className="flex-1 space-y-2">
                <Label htmlFor="username" className="text-xs text-muted-foreground">用户名（可选）</Label>
                <Input id="username" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
              </div>
              <div className="flex-1 space-y-2">
                <Label htmlFor="password" className="text-xs text-muted-foreground">密码（可选）</Label>
                <Input id="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
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

      <Modal open={confirmDelete !== null} onClose={() => setConfirmDelete(null)} title="删除出站">
        <p className="mb-4 text-sm text-muted-foreground">
          确定删除出站 <span className="font-mono">{confirmDelete?.tag}</span>？引用它的用户将回退为直连。
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="outline" className="rounded-full" onClick={() => setConfirmDelete(null)}>取消</Button>
          <Button className="rounded-full" onClick={doDelete}>确认删除</Button>
        </div>
      </Modal>
    </AppShell>
  );
}
