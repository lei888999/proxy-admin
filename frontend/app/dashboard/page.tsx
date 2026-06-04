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
    <main className="min-h-screen bg-muted p-8">
      <div className="mx-auto max-w-2xl space-y-4">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold">Dashboard</h1>
          <Button variant="outline" onClick={onLogout}>
            Logout
          </Button>
        </div>
        <Card>
          <CardHeader>
            <CardTitle>sing-box 状态</CardTitle>
          </CardHeader>
          <CardContent>
            {status === null ? (
              <p>加载中…</p>
            ) : (
              <ul className="space-y-1">
                <li>已安装：{status.installed ? "是" : "否"}</li>
                <li>版本：{status.version || "—"}</li>
                <li>运行中：{status.running ? "是" : "否"}</li>
              </ul>
            )}
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
