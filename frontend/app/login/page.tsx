"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { login } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

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
    } catch {
      setError("用户名或密码错误");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background px-6">
      <div className="w-full max-w-sm">
        <p className="mb-6 text-center font-mono text-xs tracking-[0.18em] text-muted-foreground uppercase">
          sing-box admin
        </p>
        <Card className="rounded-lg">
          <CardHeader>
            <CardTitle className="text-lg font-normal tracking-tight">登录</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={onSubmit} className="space-y-5">
              <div className="space-y-2">
                <Label
                  htmlFor="username"
                  className="font-mono text-xs tracking-wider text-muted-foreground uppercase"
                >
                  Username
                </Label>
                <Input
                  id="username"
                  className="h-10"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label
                  htmlFor="password"
                  className="font-mono text-xs tracking-wider text-muted-foreground uppercase"
                >
                  Password
                </Label>
                <Input
                  id="password"
                  type="password"
                  className="h-10"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" className="h-10 w-full rounded-full" disabled={loading}>
                {loading ? "..." : "Sign in"}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </main>
  );
}
