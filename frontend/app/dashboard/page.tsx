"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getStatus, logout, UnauthorizedError, SingboxStatus } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DashboardPage() {
  const router = useRouter();
  const [status, setStatus] = useState<SingboxStatus | null>(null);

  useEffect(() => {
    getStatus()
      .then(setStatus)
      .catch((err) => {
        if (err instanceof UnauthorizedError) {
          router.push("/login");
        }
      });
  }, [router]);

  async function onLogout() {
    await logout();
    router.push("/login");
  }

  return (
    <main className="min-h-screen bg-background">
      <div className="mx-auto max-w-5xl px-6 py-10">
        <header className="mb-8 flex items-center justify-between">
          <div>
            <p className="font-mono text-xs tracking-[0.18em] text-muted-foreground uppercase">
              sing-box admin
            </p>
            <h1 className="mt-1 text-2xl font-normal tracking-tight">Dashboard</h1>
          </div>
          <Button variant="outline" className="rounded-full" onClick={onLogout}>
            Logout
          </Button>
        </header>
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
              <div className="divide-y divide-border">
                <div className="flex items-center justify-between py-3">
                  <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
                    已安装
                  </span>
                  <span className="text-sm">{status.installed ? "是" : "否"}</span>
                </div>
                <div className="flex items-center justify-between py-3">
                  <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
                    版本
                  </span>
                  <span className="font-mono text-sm">{status.version || "—"}</span>
                </div>
                <div className="flex items-center justify-between py-3">
                  <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
                    运行中
                  </span>
                  <span className="inline-flex items-center gap-2 text-sm">
                    <span
                      className="size-1.5 rounded-full"
                      style={{ background: status.running ? "var(--sunset)" : "var(--muted-foreground)" }}
                    />
                    {status.running ? "运行中" : "已停止"}
                  </span>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
