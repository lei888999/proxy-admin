"use client";

import { useConfig } from "@/lib/hooks";
import { AppShell } from "@/components/app-shell";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function ConfigPage() {
  const { data: content = "", error } = useConfig();

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">配置</h1>
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            config.json（由入站/用户自动生成，只读）
          </CardTitle>
        </CardHeader>
        <CardContent>
          <textarea
            value={content}
            readOnly
            spellCheck={false}
            className="h-96 w-full rounded-lg border border-input bg-secondary p-3 font-mono text-sm text-foreground outline-none"
          />
          {error && <p className="mt-3 text-sm text-destructive">加载配置失败</p>}
        </CardContent>
      </Card>
    </AppShell>
  );
}
