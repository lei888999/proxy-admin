"use client";

import { useState } from "react";
import {
  createInbound, updateInbound, resetInboundKeys, deleteInbound, Inbound, InboundType,
} from "@/lib/api";
import { useInbounds, useInboundTypes } from "@/lib/hooks";
import { AppShell } from "@/components/app-shell";
import { Modal } from "@/components/modal";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { fieldLabel } from "@/lib/field-labels";
import { copyText } from "@/lib/utils";
import { ArrowDownToLine, Copy, Pencil, KeyRound, Trash2, Plus } from "lucide-react";

type FormState = {
  id?: number;
  type: string;
  tag: string;
  port: string;
  params: Record<string, string | number>;
};

const fallbackType: InboundType = {
  type: "vless-reality", label: "VLESS-Reality", network: "tcp", defaultPort: 4443,
  fields: [
    { name: "handshake", label: "握手域名", type: "text", default: "www.cloudflare.com" },
    { name: "handshakePort", label: "握手端口", type: "number", default: 443 },
  ],
};

function defaultsFor(type: InboundType | undefined): Record<string, string | number> {
  return Object.fromEntries((type?.fields ?? []).map((f) => [f.name, f.default ?? ""]));
}

export default function InboundsPage() {
  const { data: inbounds = [], mutate: refresh } = useInbounds();
  const { data: types = [] } = useInboundTypes();
  const [form, setForm] = useState<FormState | null>(null);
  const [confirmReset, setConfirmReset] = useState<Inbound | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Inbound | null>(null);
  const [error, setError] = useState("");
  const [warning, setWarning] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);

  async function finishMutation(configApplyError: string | undefined, success: string) {
    setNotice(success);
    setWarning(configApplyError ? `已保存，但应用到 sing-box 失败：${configApplyError}` : "");
    // A completed write must not look like a failure just because the follow-up
    // list refresh failed. The old fire-and-forget refresh hid this failure and
    // gave the user no visible proof that creation completed.
    try {
      await refresh();
    } catch {
      setWarning("已保存，但列表刷新失败；请刷新页面确认最新状态。");
    }
  }

  function openCreate() {
    setError("");
    const type = types[0] ?? fallbackType;
    setForm({ type: type.type, tag: "", port: String(type.defaultPort), params: defaultsFor(type) });
  }
  function openEdit(ib: Inbound) {
    setError("");
    const type = types.find((t) => t.type === ib.type) ?? fallbackType;
    const params = Object.fromEntries((type.fields ?? []).map((f) => [f.name, String(ib.publicInfo[f.name] ?? f.default ?? "")]));
    setForm({
      id: ib.id, type: ib.type, tag: ib.tag, port: String(ib.port),
      params,
    });
  }

  function setType(t: string | null) {
    if (!form || !t) return;
    const info = types.find((x) => x.type === t);
    setForm({ ...form, type: t, port: info ? String(info.defaultPort) : form.port, params: defaultsFor(info) });
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!form) return;
    setBusy(true);
    setError("");
    const type = types.find((t) => t.type === form.type) ?? fallbackType;
    const params: Record<string, unknown> = Object.fromEntries(
      (type.fields ?? []).map((field) => [
        field.name,
        field.type === "number" ? Number(form.params[field.name]) : String(form.params[field.name] ?? ""),
      ]),
    );
    try {
      const res = form.id
        ? await updateInbound(form.id, form.tag, Number(form.port), params)
        : await createInbound(form.type, form.tag, Number(form.port), params);
      setForm(null);
      await finishMutation(res.configApplyError, form.id ? "入站已保存。" : "入站已创建。");
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  async function doReset() {
    if (!confirmReset) return;
    setError("");
    try {
      const res = await resetInboundKeys(confirmReset.id);
      setConfirmReset(null);
      await finishMutation(res.configApplyError, "入站密钥已重置。");
    } catch (err) {
      setError(err instanceof Error ? err.message : "重置密钥失败");
    }
  }
  async function doDelete() {
    if (!confirmDelete) return;
    setError("");
    try {
      const res = await deleteInbound(confirmDelete.id);
      setConfirmDelete(null);
      await finishMutation(res.configApplyError, "入站已删除。");
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除入站失败");
    }
  }

  const isEdit = !!form?.id;

  return (
    <AppShell>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-normal tracking-tight">入站</h1>
        <Button className="h-9 rounded-full px-5" onClick={openCreate}><Plus />新建入站</Button>
      </div>

      {notice && <p className="mb-4 rounded-lg border border-border bg-secondary px-4 py-3 text-sm text-foreground">{notice}</p>}
      {warning && <p className="mb-4 rounded-lg border border-border bg-secondary px-4 py-3 text-sm text-muted-foreground">{warning}</p>}
      {error && <p className="mb-4 text-sm text-destructive">{error}</p>}

      <div className="space-y-4">
        {inbounds.map((ib) => (
          <Card key={ib.id} className="rounded-lg">
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base font-normal">
                  {ib.tag} <span className="text-muted-foreground">· {ib.type} · :{ib.port}/{ib.network}</span>
                </CardTitle>
                <div className="flex items-center gap-0.5">
                  <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label="编辑" title="编辑" onClick={() => openEdit(ib)}>
                    <Pencil />
                  </Button>
                  <Button variant="ghost" size="icon" className="text-muted-foreground" aria-label="重置密钥" title="重置 reality 密钥 / 证书" onClick={() => setConfirmReset(ib)}>
                    <KeyRound />
                  </Button>
                  <Button variant="ghost" size="icon" aria-label="删除" title="删除" className="text-muted-foreground hover:bg-destructive/10 hover:text-destructive" onClick={() => setConfirmDelete(ib)}>
                    <Trash2 />
                  </Button>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <dl className="grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2 lg:grid-cols-3">
                {Object.entries(ib.publicInfo).map(([k, v]) => {
                  const val = String(v);
                  const long = val.length > 18;
                  return (
                    <div key={k} className="min-w-0">
                      <dt className="font-mono text-[11px] tracking-wider text-muted-foreground uppercase">
                        {fieldLabel(k)}
                      </dt>
                      <dd className="mt-0.5 flex items-center gap-1">
                        <span className="truncate font-mono text-sm" title={val}>{val}</span>
                        {long && (
                          <Button
                            variant="ghost"
                            size="icon-xs"
                            className="shrink-0 text-muted-foreground"
                            aria-label={`复制${fieldLabel(k)}`}
                            onClick={() => copyText(val)}
                          >
                            <Copy />
                          </Button>
                        )}
                      </dd>
                    </div>
                  );
                })}
              </dl>
            </CardContent>
          </Card>
        ))}
        {inbounds.length === 0 && (
          <Card className="rounded-lg">
            <EmptyState
              icon={ArrowDownToLine}
              title="还没有入站"
              description="入站是客户端连入的代理入口，先创建一个开始。"
              action={<Button className="rounded-full" onClick={openCreate}>添加入站</Button>}
            />
          </Card>
        )}
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
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {(types.find((t) => t.type === form.type)?.fields ?? fallbackType.fields).map((field) => (
                <div key={field.name} className="space-y-2">
                  <Label htmlFor={`param-${field.name}`} className="text-xs text-muted-foreground">{field.label}</Label>
                  <Input
                    id={`param-${field.name}`}
                    type={field.type === "number" ? "number" : "text"}
                    value={String(form.params[field.name] ?? "")}
                    required={field.required}
                    onChange={(e) => setForm({ ...form, params: { ...form.params, [field.name]: e.target.value } })}
                  />
                </div>
              ))}
            </div>
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
