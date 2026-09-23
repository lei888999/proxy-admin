"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { login } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(username, password);
      router.push("/dashboard");
    } catch (err) {
      // Show the server's message so a throttled login says how long to wait
      // instead of looking like a wrong password.
      setError(err instanceof Error ? err.message : "用户名或密码错误");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="grid min-h-screen bg-background lg:grid-cols-2">
      <section className="hidden flex-col justify-between border-r border-border p-12 lg:flex">
        <span className="font-mono text-xs tracking-[0.2em] text-muted-foreground uppercase">
          sing-box admin
        </span>
        <div>
          <h1 className="max-w-md text-5xl leading-tight font-normal tracking-[-0.03em] text-foreground">
            掌控你的 sing-box 代理
          </h1>
          <p className="mt-5 max-w-sm text-base leading-relaxed text-muted-foreground">
            一处管理服务端配置、启停与运行状态。
          </p>
        </div>
        <span className="font-mono text-xs tracking-wider text-muted-foreground">
          自建 · 单二进制 · 部署即用
        </span>
      </section>

      <section className="flex items-center justify-center p-8">
        <div className="w-full max-w-sm">
          <p className="mb-2 font-mono text-xs tracking-[0.18em] text-muted-foreground uppercase lg:hidden">
            sing-box admin
          </p>
          <h2 className="mb-1 text-2xl font-normal tracking-tight">登录</h2>
          <p className="mb-8 text-sm text-muted-foreground">使用管理员账号继续</p>
          <form onSubmit={onSubmit} className="space-y-5">
            <div className="space-y-2">
              <Label htmlFor="username" className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
                用户名
              </Label>
              <Input id="username" className="h-11" value={username} onChange={(e) => setUsername(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password" className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
                密码
              </Label>
              <Input
                id="password"
                type="password"
                className="h-11"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" className="h-11 w-full rounded-full" disabled={loading}>
              {loading ? "登录中…" : "登录"}
            </Button>
          </form>
        </div>
      </section>
    </main>
  );
}
