"use client";

import { useEffect, useState } from "react";
import { getConfig, saveConfig } from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function ConfigPage() {
  const [content, setContent] = useState("");
  const [error, setError] = useState("");
  const [ok, setOk] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    getConfig().then(setContent).catch(() => setError("加载配置失败"));
  }, []);

  async function onSave() {
    setBusy(true);
    setError("");
    setOk(false);
    try {
      await saveConfig(content);
      setOk(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "保存失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">配置</h1>
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
            config.json
          </CardTitle>
        </CardHeader>
        <CardContent>
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            spellCheck={false}
            className="h-96 w-full rounded-lg border border-input bg-secondary p-3 font-mono text-sm text-foreground outline-none focus-visible:border-ring"
            placeholder='{ "log": { "level": "info" } }'
          />
          {error && <p className="mt-3 text-sm text-destructive">{error}</p>}
          {ok && <p className="mt-3 text-sm text-muted-foreground">已保存</p>}
          <div className="mt-4">
            <Button className="rounded-full" disabled={busy} onClick={onSave}>
              保存配置
            </Button>
          </div>
        </CardContent>
      </Card>
    </AppShell>
  );
}
