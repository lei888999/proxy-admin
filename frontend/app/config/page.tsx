"use client";

import { useState } from "react";
import { useConfig, useRouteRules } from "@/lib/hooks";
import { saveRouteRules, type RouteRule } from "@/lib/api";
import { AppShell } from "@/components/app-shell";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ArrowDown, ArrowUp, Loader2, Plus, Trash2 } from "lucide-react";

const ruleTypes: Record<string, { label: string; example: string }> = {
  DOMAIN: { label: "完整域名", example: "example.com" },
  "DOMAIN-SUFFIX": { label: "域名后缀", example: "example.com" },
  "DOMAIN-KEYWORD": { label: "域名关键字", example: "google" },
  "DOMAIN-WILDCARD": { label: "域名通配符", example: "*.example.com" },
  "DOMAIN-REGEX": { label: "域名正则", example: "^example[0-9]+\\.com$" },
  GEOSITE: { label: "域名分类", example: "google" },
  GEOIP: { label: "目标 IP 国家", example: "CN" },
  "IP-CIDR": { label: "目标 IP 网段", example: "8.8.8.0/24" },
  "IP-CIDR6": { label: "目标 IPv6 网段", example: "2001:db8::/32" },
  "IP-ASN": { label: "目标自治系统", example: "13335" },
  "SRC-IP-CIDR": { label: "来源 IP 网段", example: "192.168.1.0/24" },
  "SRC-GEOIP": { label: "来源 IP 国家", example: "CN" },
  "SRC-IP-ASN": { label: "来源自治系统", example: "13335" },
  "DST-PORT": { label: "目标端口", example: "80/443/8000-9000" },
  "SRC-PORT": { label: "来源端口", example: "1234" },
  "IN-PORT": { label: "客户端入站端口", example: "7890" },
  "PROCESS-NAME": { label: "进程名称", example: "chrome.exe" },
  "PROCESS-PATH": { label: "进程路径", example: "/usr/bin/curl" },
  "PROCESS-NAME-WILDCARD": { label: "进程名称通配符", example: "*telegram*" },
  "PROCESS-PATH-WILDCARD": { label: "进程路径通配符", example: "/usr/*/curl" },
  "PROCESS-NAME-REGEX": { label: "进程名称正则", example: "(?i)telegram" },
  "PROCESS-PATH-REGEX": { label: "进程路径正则", example: ".*bin/curl" },
  NETWORK: { label: "网络协议", example: "tcp 或 udp" },
  UID: { label: "Linux 用户 ID", example: "1000" },
  DSCP: { label: "DSCP 标记", example: "0–63" },
  "RULE-SET": { label: "远程规则集", example: "ads（英文、数字、下划线或短横线）" },
  MATCH: { label: "最终兜底", example: "无需匹配值，放在最后" },
};
const typeLabels = Object.fromEntries(Object.entries(ruleTypes).map(([type, info]) => [type, `${info.label} · ${type}`]));
const policyLabels = { DIRECT: "本机直连", REJECT: "拒绝连接", 节点: "节点", 自动选择: "自动选择", 故障切换: "故障切换" };
const noResolveTypes = new Set(["IP-CIDR", "IP-CIDR6", "GEOIP", "IP-ASN"]);

function RuleSelect({ id, value, items, disabled, onChange }: {
  id: string; value: string; items: Record<string, string>; disabled: boolean; onChange: (value: string) => void;
}) {
  return <Select items={items} value={value} disabled={disabled} onValueChange={(next) => next && onChange(next)}>
    <SelectTrigger id={id} className="w-full"><SelectValue /></SelectTrigger>
    <SelectContent>{Object.entries(items).map(([key, label]) => <SelectItem key={key} value={key}>{label}</SelectItem>)}</SelectContent>
  </Select>;
}

export default function ConfigPage() {
  const { data: content = "", error } = useConfig();
  const { data: savedRules, error: loadRulesError, isLoading: rulesLoading, mutate: refreshRules } = useRouteRules();
  const [draft, setDraft] = useState<RouteRule[] | null>(null);
  const [rulesNotice, setRulesNotice] = useState("");
  const [rulesBusy, setRulesBusy] = useState(false);
  const rules = draft ?? savedRules ?? [];
  const disabled = rulesBusy || rulesLoading || !!loadRulesError;

  function edit(next: RouteRule[]) {
    setDraft(next);
    setRulesNotice("");
  }
  function updateRule(index: number, patch: Partial<RouteRule>) {
    edit(rules.map((rule, i) => i === index ? { ...rule, ...patch } : rule));
  }
  function moveRule(index: number, delta: number) {
    const next = [...rules];
    [next[index], next[index + delta]] = [next[index + delta], next[index]];
    edit(next);
  }
  async function applyRules() {
    if (rules.some((rule) => rule.type !== "MATCH" && !rule.value.trim())) {
      setRulesNotice("请填写完整的匹配值，或删除空白行");
      return;
    }
    if (rules.some((rule, index) => rule.type === "MATCH" && index !== rules.length - 1)) {
      setRulesNotice("MATCH 规则只能有一条，并且必须放在最后");
      return;
    }
    setRulesBusy(true);
    setRulesNotice("");
    try {
      await saveRouteRules(rules);
      // Keep the saved draft visible while fetching normalized server values.
      await refreshRules();
      setDraft(null);
      setRulesNotice("客户端规则已保存，请在客户端更新订阅后生效。服务端无需重启。");
    } catch (err) {
      setRulesNotice(err instanceof Error ? err.message : "保存分流规则失败");
    } finally {
      setRulesBusy(false);
    }
  }

  return (
    <AppShell>
      <h1 className="mb-6 text-2xl font-normal tracking-tight">配置</h1>
      <Card className="rounded-lg">
        <CardHeader>
          <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">服务端 config.json（只读）</CardTitle>
          <p className="text-sm text-muted-foreground">服务端按用户绑定的出口转发，不再按域名、地区或 IP 分流。</p>
        </CardHeader>
        <CardContent>
          <textarea aria-label="服务端配置" value={content} readOnly spellCheck={false} className="h-96 w-full rounded-lg border border-input bg-secondary p-3 font-mono text-sm text-foreground outline-none" />
          {error && <p className="mt-3 text-sm text-destructive">加载配置失败</p>}
        </CardContent>
      </Card>
      <Card className="mt-6 rounded-lg">
        <CardHeader className="flex-wrap gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <CardTitle className="font-mono text-xs tracking-wider text-muted-foreground uppercase">客户端自定义分流</CardTitle>
            <p className="mt-2 text-sm text-muted-foreground">仅写入客户端订阅，适用于 Clash Verge / Mihomo。按从上到下的顺序匹配，优先于默认国内直连规则。</p>
          </div>
          <Button variant="outline" size="sm" onClick={() => {
            const next = [...rules];
            const matchIndex = next.findIndex((rule) => rule.type === "MATCH");
            next.splice(matchIndex < 0 ? next.length : matchIndex, 0, { type: "DOMAIN-SUFFIX", value: "", policy: "节点" });
            edit(next);
          }} disabled={disabled}><Plus />添加规则</Button>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-xs text-muted-foreground">服务器地址直连和 IPv6 阻断优先保护连接。MATCH 只修改默认规则后的兜底策略。进程、UID、DSCP 规则是否生效取决于客户端平台与运行权限。</p>
          {rulesLoading && <p className="text-sm text-muted-foreground">加载分流规则中…</p>}
          {rules.map((rule, index) => (
            <div key={index} className="space-y-3 rounded-lg border border-border p-3">
              <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_10rem_auto] lg:items-end">
                <div className="min-w-0 space-y-1">
                  <Label htmlFor={`route-type-${index}`} className="text-xs text-muted-foreground">规则类型 {index + 1}</Label>
                  <RuleSelect id={`route-type-${index}`} items={typeLabels} value={rule.type} disabled={disabled} onChange={(type) => updateRule(index, {
                    type, value: "", noResolve: false,
                    provider: type === "RULE-SET" ? { url: "", behavior: "domain", format: "yaml" } : undefined,
                  })} />
                </div>
                <div className="min-w-0 space-y-1">
                  <Label htmlFor={`route-value-${index}`} className="text-xs text-muted-foreground">匹配值 {index + 1}</Label>
                  <Input id={`route-value-${index}`} placeholder={ruleTypes[rule.type]?.example} value={rule.value} disabled={disabled || rule.type === "MATCH"} onChange={(e) => updateRule(index, { value: e.target.value })} />
                </div>
                <div className="space-y-1">
                  <Label htmlFor={`route-policy-${index}`} className="text-xs text-muted-foreground">客户端策略 {index + 1}</Label>
                  <RuleSelect id={`route-policy-${index}`} items={policyLabels} value={rule.policy} disabled={disabled} onChange={(policy) => updateRule(index, { policy })} />
                </div>
                <div className="flex gap-1">
                  <Button variant="ghost" size="icon" aria-label={`上移规则 ${index + 1}`} disabled={disabled || index === 0} onClick={() => moveRule(index, -1)}><ArrowUp /></Button>
                  <Button variant="ghost" size="icon" aria-label={`下移规则 ${index + 1}`} disabled={disabled || index === rules.length - 1} onClick={() => moveRule(index, 1)}><ArrowDown /></Button>
                  <Button variant="ghost" size="icon" aria-label={`删除规则 ${index + 1}`} disabled={disabled} onClick={() => edit(rules.filter((_, i) => i !== index))}><Trash2 /></Button>
                </div>
              </div>
              {noResolveTypes.has(rule.type) && <div className="flex items-center gap-2">
                <Checkbox id={`no-resolve-${index}`} checked={!!rule.noResolve} disabled={disabled} onCheckedChange={(checked) => updateRule(index, { noResolve: checked })} />
                <Label htmlFor={`no-resolve-${index}`} className="text-xs text-muted-foreground">no-resolve：匹配时不额外解析域名</Label>
              </div>}
              {rule.type === "RULE-SET" && rule.provider && <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_9rem_7rem]">
                <div className="space-y-1"><Label htmlFor={`provider-url-${index}`}>规则集 HTTPS 地址</Label>
                  <Input id={`provider-url-${index}`} value={rule.provider.url} placeholder="https://example.com/rules.yaml" disabled={disabled} onChange={(e) => updateRule(index, { provider: { ...rule.provider!, url: e.target.value } })} />
                </div>
                <div className="space-y-1"><Label htmlFor={`provider-behavior-${index}`}>规则集行为</Label>
                  <RuleSelect id={`provider-behavior-${index}`} value={rule.provider.behavior} items={{ domain: "域名 domain", ipcidr: "IP ipcidr", classical: "混合 classical" }} disabled={disabled} onChange={(behavior) => updateRule(index, { provider: { ...rule.provider!, behavior } })} />
                </div>
                <div className="space-y-1"><Label htmlFor={`provider-format-${index}`}>文件格式</Label>
                  <RuleSelect id={`provider-format-${index}`} value={rule.provider.format} items={{ yaml: "YAML", text: "Text", mrs: "MRS" }} disabled={disabled} onChange={(format) => updateRule(index, { provider: { ...rule.provider!, format } })} />
                </div>
                <p className="text-xs text-muted-foreground sm:col-span-3">客户端经「节点」下载，每 24 小时更新。MRS 仅支持 domain / ipcidr。</p>
              </div>}
              <p className="break-all font-mono text-xs text-muted-foreground">{rule.type}{rule.type !== "MATCH" && `,${rule.value || "…"}`},{rule.policy}{rule.noResolve && ",no-resolve"}</p>
            </div>
          ))}
          {!rulesLoading && rules.length === 0 && <p className="text-sm text-muted-foreground">暂无自定义规则。默认规则仍在客户端执行：Gemini 代理、私网与国内直连、其余走节点。</p>}
          {loadRulesError && <p className="text-sm text-destructive">加载分流规则失败，请刷新页面重试。</p>}
          {rulesNotice && <p role="status" className="text-sm text-muted-foreground">{rulesNotice}</p>}
          <div className="flex justify-end pt-2"><Button className="rounded-full" onClick={applyRules} disabled={disabled}>
            {rulesBusy && <Loader2 className="animate-spin" />}{rulesBusy ? "保存中…" : "保存客户端规则"}
          </Button></div>
        </CardContent>
      </Card>
    </AppShell>
  );
}
