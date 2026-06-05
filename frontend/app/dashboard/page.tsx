"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getStatus, startSingbox, stopSingbox, applySingbox, getLiveTraffic, LiveTraffic, UnauthorizedError, SingboxStatus } from "@/lib/api";
import { formatBytes } from "@/lib/utils";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DashboardPage() {
  const router = useRouter();
  const [status, setStatus] = useState<SingboxStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [live, setLive] = useState<LiveTraffic>({ up: 0, down: 0 });

  const refresh = useCallback(() => {
    getStatus()
      .then(setStatus)
      .catch((err) => {
        if (err instanceof UnauthorizedError) router.push("/login");
      });
  }, [router]);

  useEffect(refresh, [refresh]);
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

  const row = (label: string, value: React.ReactNode) => (
    <div className="flex items-center justify-between py-3">
      <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">{label}</span>
      <span className="text-sm">{value}</span>
    </div>
  );

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">概览</h1>
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
              <div className="divide-y divide-border">
                {row("已安装", status.installed ? "是" : "否")}
                {row("版本", <span className="font-mono">{status.version || "—"}</span>)}
                {row("配置", status.hasConfig ? "已就绪" : "未配置")}
                {row(
                  "运行状态",
                  <span className="inline-flex items-center gap-2">
                    <span
                      className="size-1.5 rounded-full"
                      style={{ background: status.running ? "var(--sunset)" : "var(--muted-foreground)" }}
                    />
                    {status.running ? "运行中" : "已停止"}
                  </span>
                )}
              </div>
              {error && <p className="mt-4 text-sm text-destructive">{error}</p>}
              <div className="mt-6 flex gap-3">
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

      <Card className="mt-6 rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            实时吞吐
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-10">
            <div>
              <p className="font-mono text-xs tracking-wider text-muted-foreground uppercase">上行</p>
              <p className="text-2xl font-normal">{formatBytes(live.up)}/s</p>
            </div>
            <div>
              <p className="font-mono text-xs tracking-wider text-muted-foreground uppercase">下行</p>
              <p className="text-2xl font-normal">{formatBytes(live.down)}/s</p>
            </div>
          </div>
        </CardContent>
      </Card>
    </AppShell>
  );
}
