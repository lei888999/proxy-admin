"use client";

import { useState } from "react";
import { startSingbox, stopSingbox, applySingbox, SingboxStatus } from "@/lib/api";
import { useStatus, useLiveTraffic, useInbounds, useOutbounds, useUsers, useOutboundProbes } from "@/lib/hooks";
import { formatBytes } from "@/lib/utils";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StatCard } from "@/components/stat-card";
import { ArrowUp, ArrowDown, Activity } from "lucide-react";

export default function DashboardPage() {
  const { data: status, mutate: mutateStatus } = useStatus();
  const { data: live = { up: 0, down: 0 } } = useLiveTraffic();
  const { data: inbounds } = useInbounds();
  const { data: outbounds } = useOutbounds();
  const { data: users } = useUsers();
  const { data: probes = [], mutate: refreshProbes } = useOutboundProbes();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const counts = {
    inbounds: inbounds?.length ?? 0,
    outbounds: outbounds?.length ?? 0,
    users: users?.length ?? 0,
  };

  async function run(action: () => Promise<SingboxStatus>) {
    setBusy(true);
    setError("");
    try {
      mutateStatus(await action(), { revalidate: false });
    } catch (e) {
      setError(e instanceof Error ? e.message : "操作失败");
    } finally {
      setBusy(false);
    }
  }

  const statusLine = () => {
    if (!status) return null;
    const parts = [
      status.installed ? `版本 ${status.version || "未知"}` : "未安装",
      status.hasConfig ? "配置已就绪" : "未配置",
    ];
    return (
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <span
          className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm ${
            status.running ? "bg-sunset/15 text-sunset" : "bg-secondary text-muted-foreground"
          }`}
        >
          <span
            className={`size-1.5 rounded-full ${status.running ? "animate-pulse" : ""}`}
            style={{ background: status.running ? "var(--sunset)" : "var(--muted-foreground)" }}
          />
          {status.running ? "运行中" : "已停止"}
        </span>
        <span className="font-mono text-xs text-muted-foreground">{parts.join(" · ")}</span>
      </div>
    );
  };

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">概览</h1>
      <div className="mb-6 grid gap-4 sm:grid-cols-3">
        <StatCard label="入站" value={counts.inbounds} sub="代理入口" />
        <StatCard label="出站" value={counts.outbounds} sub="转发节点" />
        <StatCard label="用户" value={counts.users} sub="已配置账户" />
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <Card className="rounded-lg">
          <CardHeader>
            <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
              sing-box 状态
            </CardTitle>
          </CardHeader>
          <CardContent>
            {!status ? (
              <p className="text-sm text-muted-foreground">加载中…</p>
            ) : (
              <>
                {statusLine()}
                {status.external && (
                  <p className="mt-4 rounded-lg border border-border bg-secondary px-3 py-2 text-xs text-muted-foreground">
                    检测到一个不由面板管理的 sing-box 进程。面板不会启停它，但它会占用代理端口，
                    并让流量统计读不到 Clash API。请先手动停掉它，再用面板启动。
                  </p>
                )}
                {error && <p className="mt-4 text-sm text-destructive">{error}</p>}
                <div className="mt-6 flex flex-wrap gap-3">
                  <Button
                    className="rounded-full"
                    disabled={busy || !status.installed || !status.hasConfig || status.running}
                    onClick={() => run(startSingbox)}
                  >
                    启动
                  </Button>
                  <Button
                    variant="outline"
                    className="rounded-full"
                    disabled={busy || !status.running}
                    onClick={() => run(stopSingbox)}
                  >
                    停止
                  </Button>
                  <Button
                    variant="outline"
                    className="rounded-full"
                    disabled={busy || !status.installed}
                    onClick={() => run(applySingbox)}
                  >
                    应用并重启
                  </Button>
                </div>
              </>
            )}
          </CardContent>
        </Card>

        <Card className="rounded-lg">
          <CardHeader>
            <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
              实时吞吐
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex gap-10">
              <div>
                <p className="flex items-center gap-1 font-mono text-xs tracking-wider text-muted-foreground uppercase">
                  <ArrowUp className="size-3" /> 上行
                </p>
                <p className="mt-1 text-2xl font-normal tabular-nums">{formatBytes(live.up)}/s</p>
              </div>
              <div>
                <p className="flex items-center gap-1 font-mono text-xs tracking-wider text-muted-foreground uppercase">
                  <ArrowDown className="size-3" /> 下行
                </p>
                <p className="mt-1 text-2xl font-normal tabular-nums">{formatBytes(live.down)}/s</p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="rounded-lg">
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
              上游线路检测
            </CardTitle>
            <Button variant="ghost" size="sm" className="gap-1 text-xs" onClick={() => refreshProbes()}>
              <Activity className="size-3" />重新检测
            </Button>
          </CardHeader>
          <CardContent>
            {probes.length === 0 ? (
              <p className="text-sm text-muted-foreground">暂无已配置的上游出站。</p>
            ) : (
              <div className="space-y-3">
                {probes.map((p) => (
                  <div key={p.tag} className="flex items-center justify-between gap-3 text-sm">
                    <span className="truncate font-mono">{p.tag}</span>
                    <span className={p.reachable ? "text-foreground" : "text-destructive"}>
                      {p.reachable ? `${p.latencyMs ?? 0} ms` : "不可达"}
                    </span>
                  </div>
                ))}
              </div>
            )}
            <p className="mt-4 text-xs text-muted-foreground">仅表示 VPS 到上游服务器的 TCP 建连时间，不代表客户端上传质量。</p>
          </CardContent>
        </Card>
      </div>
    </AppShell>
  );
}
