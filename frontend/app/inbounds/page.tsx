"use client";

import { useCallback, useEffect, useState } from "react";
import {
  listInbounds, createInbound, deleteInbound,
  listUsers, createUser, deleteUser,
  Inbound, SingboxUser,
} from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function InboundsPage() {
  const [inbounds, setInbounds] = useState<Inbound[]>([]);
  const [tag, setTag] = useState("");
  const [port, setPort] = useState("");
  const [handshake, setHandshake] = useState("www.microsoft.com");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(() => {
    listInbounds().then(setInbounds).catch(() => setError("加载入站失败"));
  }, []);
  useEffect(refresh, [refresh]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await createInbound(tag, Number(port), handshake);
      setTag(""); setPort("");
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
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            新建入站（VLESS + Reality）
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onCreate} className="flex flex-wrap items-end gap-4">
            <div className="space-y-2">
              <Label htmlFor="tag" className="text-xs text-muted-foreground">标签</Label>
              <Input id="tag" className="h-10 w-40" value={tag} onChange={(e) => setTag(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="port" className="text-xs text-muted-foreground">端口</Label>
              <Input id="port" className="h-10 w-28" value={port} onChange={(e) => setPort(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="hs" className="text-xs text-muted-foreground">握手域名</Label>
              <Input id="hs" className="h-10 w-56" value={handshake} onChange={(e) => setHandshake(e.target.value)} />
            </div>
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
  const [users, setUsers] = useState<SingboxUser[]>([]);
  const [name, setName] = useState("");

  const refresh = useCallback(() => {
    listUsers(inbound.id).then(setUsers).catch(() => {});
  }, [inbound.id]);
  useEffect(refresh, [refresh]);

  async function onAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!name) return;
    await createUser(inbound.id, name);
    setName("");
    refresh();
  }
  async function onRemove(id: number) {
    await deleteUser(id);
    refresh();
  }

  const field = (k: string, v: string) => (
    <div className="flex justify-between gap-4 py-1">
      <span className="font-mono text-xs text-muted-foreground uppercase">{k}</span>
      <span className="truncate font-mono text-xs">{v}</span>
    </div>
  );

  return (
    <Card className="rounded-lg">
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-base font-normal">
            {inbound.tag} <span className="text-muted-foreground">:{inbound.port}</span>
          </CardTitle>
          <Button variant="outline" className="rounded-full" onClick={onDelete}>删除</Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="mb-4 border-b border-border pb-3">
          {field("公钥", inbound.realityPublicKey)}
          {field("short id", inbound.realityShortId)}
          {field("sni", inbound.serverName)}
          {field("flow", inbound.flow)}
        </div>
        <p className="mb-2 font-mono text-xs text-muted-foreground uppercase">用户</p>
        <div className="divide-y divide-border">
          {users.map((u) => (
            <div key={u.id} className="flex items-center justify-between gap-4 py-2">
              <span className="text-sm">{u.name}</span>
              <span className="truncate font-mono text-xs text-muted-foreground">{u.uuid}</span>
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
