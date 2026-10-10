"use client";

import { useState } from "react";
import { useConnectionLogs, useUsers } from "@/lib/hooks";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableHeader, TableHead, TableBody, TableRow, TableCell } from "@/components/ui/table";

const filters = { all: "全部出口", direct: "服务器直连", proxy: "上游出站", unknown: "未记录出站", error: "有错误" };

export default function LogsPage() {
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const { data, error, isLoading, isValidating, mutate } = useConnectionLogs(autoRefresh);
  const { data: users = [] } = useUsers();
  const userNames = new Map(users.map((user) => [`u${user.id}`, user.name]));
  const search = query.trim().toLowerCase();
  const entries = (data?.entries ?? []).filter((entry) => {
    if (filter === "direct" && !entry.outbounds.some((o) => o.type === "direct")) return false;
    if (filter === "proxy" && !entry.outbounds.some((o) => o.type !== "direct")) return false;
    if (filter === "unknown" && entry.outbounds.length > 0) return false;
    if (filter === "error" && !entry.error) return false;
    return [entry.id, entry.target, entry.source, entry.user, userNames.get(entry.user), entry.inbound, entry.error,
      ...entry.outbounds.flatMap((o) => [o.tag, o.type, o.target])].join(" ").toLowerCase().includes(search);
  });

  return <AppShell>
    <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h1 className="text-2xl font-normal tracking-tight">连接日志</h1>
      <div className="flex gap-2">
        <Button variant="outline" onClick={() => setAutoRefresh((current) => !current)}>{autoRefresh ? "暂停自动刷新" : "开启自动刷新"}</Button>
        <Button variant="outline" disabled={isValidating} onClick={() => mutate()}>{isValidating ? "刷新中…" : "刷新日志"}</Button>
      </div>
    </div>
    <p className="mb-2 text-sm text-muted-foreground">服务器直连 = 使用 VPS 本身出口；上游出站 = sing-box 日志记录的实际出站标签。已选择出站不代表连接成功或出口 IP 属于美国，错误会单独显示。</p>
    <p className="mb-6 text-sm text-muted-foreground">客户端本机直连不会经过服务器，请在 Clash Verge 的连接记录中查看。HTTPS 通常只能看到域名和端口，不能看到完整 URL；仅传入 IP 时显示 IP。此页不推测命中规则。</p>
    <Card><CardContent className="space-y-4 pt-5">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <div className="flex-1 space-y-1">
          <Label htmlFor="log-search">搜索日志</Label>
          <Input id="log-search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="域名、IP、用户、出站、连接 ID 或错误，例如 kiro" />
        </div>
        <div className="space-y-1">
          <Label htmlFor="log-filter">出口筛选</Label>
          <Select items={filters} value={filter} onValueChange={(value) => value && setFilter(value)}>
            <SelectTrigger id="log-filter" className="w-full sm:w-44"><SelectValue /></SelectTrigger>
            <SelectContent>{Object.entries(filters).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent>
          </Select>
        </div>
      </div>
      <p className="text-xs text-muted-foreground">{autoRefresh ? "每 3 秒刷新" : "自动刷新已暂停"} · 最近最多 300 条连接（读取日志末尾 1 MiB） · 当前显示 {entries.length} 条{data?.updatedAt && ` · 读取于 ${new Date(data.updatedAt).toLocaleTimeString()}`}</p>
      {data?.truncated && <p className="text-xs text-muted-foreground">仅展示日志尾部，较早的连接上下文可能不完整。未记录出站不等于直连。</p>}
      {error && <p role="alert" className="text-sm text-destructive">加载连接日志失败，请重试。{data && " 下方为上次读取结果。"}</p>}
      <Table>
        <TableHeader><TableRow>
          <TableHead>日志时间 / ID</TableHead><TableHead>用户 / 入站</TableHead><TableHead>目标</TableHead><TableHead>实际出站</TableHead><TableHead>错误</TableHead>
        </TableRow></TableHeader>
        <TableBody>{entries.map((entry, index) => <TableRow key={`${entry.id}-${index}`}>
          <TableCell className="align-top font-mono text-xs"><div>{entry.time}</div><div className="text-muted-foreground">#{entry.id}</div></TableCell>
          <TableCell className="align-top text-xs"><div>{userNames.get(entry.user) || entry.user || "未记录用户"}{userNames.has(entry.user) && ` (${entry.user})`}</div><div className="text-muted-foreground">{entry.inbound || "未记录入站"}</div><div className="text-muted-foreground">{entry.source}</div></TableCell>
          <TableCell className="max-w-72 whitespace-normal break-all align-top font-mono text-xs">{entry.target || "未记录目标"}</TableCell>
          <TableCell className="max-w-80 whitespace-normal break-all align-top text-xs">
            {entry.outbounds.length === 0 ? "未记录出站" : entry.outbounds.map((outbound, i) => <div key={i} className="mb-2 last:mb-0">
              <div>{outbound.type === "direct" ? "服务器直连（VPS 出口）" : `上游出站 · ${outbound.tag}`}</div>
              <div className="text-muted-foreground">{outbound.type} / {outbound.tag}</div>
              <div className="font-mono text-muted-foreground">{outbound.target}</div>
            </div>)}
          </TableCell>
          <TableCell className="max-w-96 whitespace-normal break-all align-top text-xs text-destructive">{entry.error || "—"}</TableCell>
        </TableRow>)}</TableBody>
      </Table>
      {isLoading ? <p className="text-sm text-muted-foreground">加载日志中…</p> : !error && entries.length === 0 && <p className="text-sm text-muted-foreground">{!data?.available ? "尚未生成 sing-box 日志，请启动代理并发起请求。" : "没有匹配的连接记录。日志级别需为 info 或 debug。"}</p>}
      <p className="text-xs text-muted-foreground">时间显示原始日志前缀；INFO[秒数] 为 sing-box 启动后的累计秒数。同一连接的 DNS 等附属出站可能一并列出。</p>
    </CardContent></Card>
  </AppShell>;
}
