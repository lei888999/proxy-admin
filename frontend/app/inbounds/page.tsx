"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listInbounds, listInboundTypes, createInbound, updateInbound, resetInboundKeys, deleteInbound,
  Inbound, InboundType,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";

type FormState = {
  id?: number;
  type: string;
  tag: string;
  port: string;
  handshake: string;
  handshakePort: string;
  sni: string;
  up: string;
  down: string;
};

const emptyForm: FormState = {
  type: "vless-reality", tag: "", port: "8443",
  handshake: "www.microsoft.com", handshakePort: "443",
  sni: "bing.com", up: "100", down: "100",
};

export default function InboundsPage() {
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [types, setTypes] = useState<InboundType[]>([]);
  const [form, setForm] = useState<FormState | null>(null);
  const [confirmReset, setConfirmReset] = useState<Inbound | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Inbound | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listInbounds().then(setInbounds).catch(() => setError("加载入站失败"));
  }, []);
  useEffect(() => {
    refresh();
    listInboundTypes().then(setTypes).catch(() => {});
  }, [refresh]);

  function openCreate() {
    setError("");
    setForm({ ...emptyForm });
  }
  function openEdit(ib: Inbound) {
    setError("");
    const pi = ib.publicInfo;
    setForm({
      id: ib.id, type: ib.type, tag: ib.tag, port: String(ib.port),
      handshake: String(pi.serverName ?? "www.microsoft.com"),
      handshakePort: String(pi.handshakePort ?? "443"),
      sni: String(pi.serverName ?? "bing.com"),
      up: String(pi.upMbps ?? "100"),
      down: String(pi.downMbps ?? "100"),
    });
  }

  function setType(t: string | null) {
    if (!form || !t) return;
    const info = types.find((x) => x.type === t);
    setForm({ ...form, type: t, port: info ? String(info.defaultPort) : form.port });
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError("");
    const params: Record<string, unknown> =
      form.type === "hysteria2"
        ? { serverName: form.sni, upMbps: Number(form.up), downMbps: Number(form.down) }
        : { handshake: form.handshake, handshakePort: Number(form.handshakePort) };
    try {
      if (form.id) await updateInbound(form.id, form.tag, Number(form.port), params);
      else await createInbound(form.type, form.tag, Number(form.port), params);
      setForm(null);
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function doReset() {
    if (!confirmReset) return;
    await resetInboundKeys(confirmReset.id);
    setConfirmReset(null);
    refresh();
  }
  async function doDelete() {
    if (!confirmDelete) return;
    await deleteInbound(confirmDelete.id);
    setConfirmDelete(null);
    refresh();
  }

  const isEdit = !!form?.id;

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">入站</h1>
        <Button className="rounded-full" onClick={openCreate}>新建入站</Button>
      </div>

      <div className="space-y-4">
        {inbounds.map((ib) => (
          <Card key={ib.id} className="rounded-lg">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base font-normal">
                  {ib.tag} <span className="text-muted-foreground">· {ib.type} · :{ib.port}/{ib.network}</span>
                </CardTitle>
                <div className="flex gap-2">
                  <Button variant="outline" className="rounded-full" onClick={() => openEdit(ib)}>编辑</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => setConfirmReset(ib)}>重置密钥</Button>
                  <Button variant="outline" className="rounded-full" onClick={() => setConfirmDelete(ib)}>删除</Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <div className="divide-y divide-border">
                {Object.entries(ib.publicInfo).map(([k, v]) => (
                  <div key={k} className="flex justify-between gap-4 py-1">
                    <span className="font-mono text-xs text-muted-foreground uppercase">{k}</span>
                    <span className="truncate font-mono text-xs">{String(v)}</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        ))}
        {inbounds.length === 0 && <p className="text-sm text-muted-foreground">暂无入站，点右上角新建。</p>}
      </div>

      <Modal open={form !== null} onClose={() => setForm(null)} title={isEdit ? "编辑入站" : "新建入站"}>
        {form && (
          <form onSubmit={submit} className="space-y-4">
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">协议</Label>
              <Select value={form.type} onValueChange={setType} disabled={isEdit}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {types.map((t) => (
                    <SelectItem key={t.type} value={t.type}>{t.label}</SelectItem>
                  ))}
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
            {form.type === "hysteria2" ? (
              <div className="flex gap-4">
                <div className="flex-1 space-y-2">
                  <Label htmlFor="sni" className="text-xs text-muted-foreground">SNI</Label>
                  <Input id="sni" value={form.sni} onChange={(e) => setForm({ ...form, sni: e.target.value })} />
                </div>
                <div className="w-24 space-y-2">
                  <Label htmlFor="up" className="text-xs text-muted-foreground">上行</Label>
                  <Input id="up" value={form.up} onChange={(e) => setForm({ ...form, up: e.target.value })} />
                </div>
                <div className="w-24 space-y-2">
                  <Label htmlFor="down" className="text-xs text-muted-foreground">下行</Label>
                  <Input id="down" value={form.down} onChange={(e) => setForm({ ...form, down: e.target.value })} />
                </div>
              </div>
            ) : (
              <div className="flex gap-4">
                <div className="flex-1 space-y-2">
                  <Label htmlFor="hs" className="text-xs text-muted-foreground">握手域名</Label>
                  <Input id="hs" value={form.handshake} onChange={(e) => setForm({ ...form, handshake: e.target.value })} />
                </div>
                <div className="w-28 space-y-2">
                  <Label htmlFor="hsp" className="text-xs text-muted-foreground">握手端口</Label>
                  <Input id="hsp" value={form.handshakePort} onChange={(e) => setForm({ ...form, handshakePort: e.target.value })} />
                </div>
              </div>
            )}
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="flex justify-end gap-3 pt-2">
              <Button type="button" variant="outline" className="rounded-full" onClick={() => setForm(null)}>取消</Button>
              <Button type="submit" className="rounded-full" disabled={busy}>{isEdit ? "保存" : "创建"}</Button>
            </div>
          </form>
        )}
      </Modal>

      <Modal open={confirmReset !== null} onClose={() => setConfirmReset(null)} title="重置密钥">
        <p className="mb-4 text-sm text-muted-foreground">
          重置后会生成新的 reality 密钥 / 证书，已分发的旧客户端将失效。确定继续？
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="outline" className="rounded-full" onClick={() => setConfirmReset(null)}>取消</Button>
          <Button className="rounded-full" onClick={doReset}>确认重置</Button>
        </div>
      </Modal>

      <Modal open={confirmDelete !== null} onClose={() => setConfirmDelete(null)} title="删除入站">
        <p className="mb-4 text-sm text-muted-foreground">
          确定删除入站 <span className="font-mono">{confirmDelete?.tag}</span>？此操作不可撤销。
        </p>
        <div className="flex justify-end gap-3">
          <Button variant="outline" className="rounded-full" onClick={() => setConfirmDelete(null)}>取消</Button>
          <Button className="rounded-full" onClick={doDelete}>确认删除</Button>
        </div>
      </Modal>
    </AppShell>
  );
}
