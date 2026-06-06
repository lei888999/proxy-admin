"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getStatus, startSingbox, stopSingbox, applySingbox, getLiveTraffic, listInbounds, listOutbounds, listUsers, LiveTraffic, UnauthorizedError, SingboxStatus } from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { StatCard } from "@/components/stat-card";
import { ArrowUp, ArrowDown } from "lucide-react";

export default function DashboardPage() {
  const router = useRouter();
  const [status, setStatus] = useState<SingboxStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [live, setLive] = useState<LiveTraffic>({ up: 0, down: 0 });
  const [counts, setCounts] = useState({ inbounds: 0, outbounds: 0, users: 0 });

  const refresh = useCallback(() => {
    getStatus()
      .then(setStatus)
      .catch((err) => {
        if (err instanceof UnauthorizedError) router.push("/login");
      });
  }, [router]);

  useEffect(refresh, [refresh]);
  useEffect(() => {
    Promise.all([
      listInbounds().catch(() => []),
      listOutbounds().catch(() => []),
      listUsers().catch(() => []),
    ]).then(([ins, outs, us]) =>
      setCounts({ inbounds: ins.length, outbounds: outs.length, users: us.length })
    );
  }, []);
  useEffect(() => {
    let active = true;
    const tick = () => getLiveTraffic().then((l) => active && setLive(l)).catch(() => {});
    tick();
    const id = setInterval(tick, 3000);
    return () => {
      active = false;
      clearInterval(id);
    };
  }, []);

  async function run(action: () => Promise<SingboxStatus>) {
    setBusy(true);
    setError("");
    try {
      setStatus(await action());
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
            {status === null ? (
              <p className="text-sm text-muted-foreground">加载中…</p>
            ) : (
              <>
                {statusLine()}
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
      </div>
    </AppShell>
  );
}
