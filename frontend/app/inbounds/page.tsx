"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listInbounds, listInboundTypes, createInbound, deleteInbound,
  listUsers, createUser, deleteUser,
  Inbound, InboundType, SingboxUser,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function InboundsPage() {
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [types, setTypes] = useState<InboundType[]>([]);
  const [type, setType] = useState("vless-reality");
  const [tag, setTag] = useState("");
  const [port, setPort] = useState("8443");
  const [handshake, setHandshake] = useState("www.microsoft.com");
  const [sni, setSni] = useState("bing.com");
  const [up, setUp] = useState("100");
  const [down, setDown] = useState("100");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listInbounds().then(setInbounds).catch(() => setError("加载入站失败"));
  }, []);
  useEffect(() => {
    refresh();
    listInboundTypes().then((ts) => {
      setTypes(ts);
      if (ts[0]) setType(ts[0].type);
    }).catch(() => {});
  }, [refresh]);

  function onTypeChange(t: string) {
    setType(t);
    const info = types.find((x) => x.type === t);
    if (info) setPort(String(info.defaultPort));
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const params: Record<string, unknown> =
      type === "hysteria2"
        ? { serverName: sni, upMbps: Number(up), downMbps: Number(down) }
        : { handshake };
    try {
      await createInbound(type, tag, Number(port), params);
      setTag("");
      refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建失败");
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(id: number) {
    await deleteInbound(id);
    refresh();
  }

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">入站</h1>

      <Card className="mb-6 rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">新建入站</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onCreate} className="flex flex-wrap items-end gap-4">
            <div className="space-y-2">
              <Label htmlFor="type" className="text-xs text-muted-foreground">协议</Label>
              <select
                id="type"
                value={type}
                onChange={(e) => onTypeChange(e.target.value)}
                className="h-10 rounded-lg border border-input bg-secondary px-2 text-sm text-foreground"
              >
                {types.map((t) => (
                  <option key={t.type} value={t.type}>{t.label}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="tag" className="text-xs text-muted-foreground">标签</Label>
              <Input id="tag" className="h-10 w-40" value={tag} onChange={(e) => setTag(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="port" className="text-xs text-muted-foreground">端口</Label>
              <Input id="port" className="h-10 w-24" value={port} onChange={(e) => setPort(e.target.value)} />
            </div>
            {type === "hysteria2" ? (
              <>
                <div className="space-y-2">
                  <Label htmlFor="sni" className="text-xs text-muted-foreground">SNI</Label>
                  <Input id="sni" className="h-10 w-40" value={sni} onChange={(e) => setSni(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="up" className="text-xs text-muted-foreground">上行 Mbps</Label>
                  <Input id="up" className="h-10 w-24" value={up} onChange={(e) => setUp(e.target.value)} />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="down" className="text-xs text-muted-foreground">下行 Mbps</Label>
                  <Input id="down" className="h-10 w-24" value={down} onChange={(e) => setDown(e.target.value)} />
                </div>
              </>
            ) : (
              <div className="space-y-2">
                <Label htmlFor="hs" className="text-xs text-muted-foreground">握手域名</Label>
                <Input id="hs" className="h-10 w-56" value={handshake} onChange={(e) => setHandshake(e.target.value)} />
              </div>
            )}
            <Button type="submit" className="h-10 rounded-full" disabled={busy}>新建入站</Button>
          </form>
          {error && <p className="mt-3 text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>

      <div className="space-y-4">
        {inbounds.map((ib) => (
          <InboundCard key={ib.id} inbound={ib} onDelete={() => onDelete(ib.id)} />
        ))}
        {inbounds.length === 0 && <p className="text-sm text-muted-foreground">暂无入站，先新建一个。</p>}
      </div>
    </AppShell>
  );
}

function InboundCard({ inbound, onDelete }: { inbound: Inbound; onDelete: () => void }) {
  const [users, setUsers] = useState<SingboxUser[]>(inbound.users ?? []);
  const [name, setName] = useState("");

  const reloadUsers = useCallback(() => {
    listUsers(inbound.id).then(setUsers).catch(() => {});
  }, [inbound.id]);

  async function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!name) return;
    await createUser(inbound.id, name);
    setName("");
    reloadUsers();
  }
  async function onRemove(id: number) {
    await deleteUser(id);
    reloadUsers();
  }

  const credLabel = inbound.type === "hysteria2" ? "密码" : "UUID";

  const field = (k: string, v: unknown) => (
    <div className="flex justify-between gap-4 py-1">
      <span className="font-mono text-xs text-muted-foreground uppercase">{k}</span>
      <span className="truncate font-mono text-xs">{String(v)}</span>
    </div>
  );

  return (
    <Card className="rounded-lg">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-normal">
            {inbound.tag}{" "}
            <span className="text-muted-foreground">· {inbound.type} · :{inbound.port}/{inbound.network}</span>
          </CardTitle>
          <Button variant="outline" className="rounded-full" onClick={onDelete}>删除</Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="mb-4 border-b border-border pb-3">
          {Object.entries(inbound.publicInfo).map(([k, v]) => field(k, v))}
        </div>
        <p className="mb-2 font-mono text-xs text-muted-foreground uppercase">用户（{credLabel}）</p>
        <div className="divide-y divide-border">
          {users.map((u) => (
            <div key={u.id} className="flex items-center justify-between gap-4 py-2">
              <span className="text-sm">{u.name}</span>
              <span className="truncate font-mono text-xs text-muted-foreground">{u.credential}</span>
              <Button variant="outline" className="rounded-full" onClick={() => onRemove(u.id)}>删除</Button>
            </div>
          ))}
          {users.length === 0 && <p className="py-2 text-sm text-muted-foreground">暂无用户</p>}
        </div>
        <form onSubmit={onAdd} className="mt-3 flex items-center gap-3">
          <Input className="h-9 w-40" placeholder="用户名称" value={name} onChange={(e) => setName(e.target.value)} />
          <Button type="submit" className="h-9 rounded-full">添加用户</Button>
        </form>
      </CardContent>
    </Card>
  );
}
