"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { logout, changePassword, UnauthorizedError } from "@/lib/api";
import { useStatus } from "@/lib/hooks";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Modal } from "@/components/modal";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import {
  User,
  LogOut,
  KeyRound,
  LayoutDashboard,
  ArrowDownToLine,
  ArrowUpFromLine,
  Users,
  SlidersHorizontal,
  type LucideIcon,
} from "lucide-react";

type NavItem = { href: string; label: string; icon: LucideIcon };

const NAV_GROUPS: { heading: string | null; items: NavItem[] }[] = [
  {
    heading: null,
    items: [{ href: "/dashboard", label: "概览", icon: LayoutDashboard }],
  },
  {
    heading: "代理",
    items: [
      { href: "/inbounds", label: "入站", icon: ArrowDownToLine },
      { href: "/outbounds", label: "出站", icon: ArrowUpFromLine },
      { href: "/users", label: "用户", icon: Users },
    ],
  },
  {
    heading: "系统",
    items: [{ href: "/config", label: "配置", icon: SlidersHorizontal }],
  },
];

// Breadcrumb crumbs per route, shown in the top bar for context.
const CRUMBS: Record<string, string[]> = {
  "/dashboard": ["概览"],
  "/inbounds": ["代理", "入站"],
  "/outbounds": ["代理", "出站"],
  "/users": ["代理", "用户"],
  "/config": ["系统", "配置"],
};

export function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  // Static export adds a trailing slash (/inbounds/), so normalize before
  // matching against the slash-free route keys used for active state + crumbs.
  const pathname = (usePathname() || "/").replace(/(.)\/$/, "$1");

  const { error } = useStatus();
  useEffect(() => {
    if (error instanceof UnauthorizedError) router.push("/login");
  }, [error, router]);

  const [pwOpen, setPwOpen] = useState(false);
  const [pwOld, setPwOld] = useState("");
  const [pwNew, setPwNew] = useState("");
  const [pwConfirm, setPwConfirm] = useState("");
  const [pwError, setPwError] = useState("");
  const [pwDone, setPwDone] = useState("");
  const [pwBusy, setPwBusy] = useState(false);

  function openPasswordModal() {
    setPwOld("");
    setPwNew("");
    setPwConfirm("");
    setPwError("");
    setPwDone("");
    setPwOpen(true);
  }

  async function onChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setPwError("");
    setPwDone("");
    if (pwNew !== pwConfirm) {
      setPwError("两次输入的新密码不一致");
      return;
    }
    setPwBusy(true);
    try {
      await changePassword(pwOld, pwNew);
      setPwDone("密码已修改，其他设备上的登录已失效。");
      setPwOld("");
      setPwNew("");
      setPwConfirm("");
    } catch (err) {
      setPwError(err instanceof Error ? err.message : "修改密码失败");
    } finally {
      setPwBusy(false);
    }
  }

  async function onLogout() {
    await logout();
    router.push("/login");
  }

  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-card">
        <div className="shrink-0 px-5 py-5">
          <span className="font-mono text-sm tracking-[0.18em] text-foreground uppercase">
            sing-box admin
          </span>
        </div>
        <nav className="flex-1 overflow-y-auto px-3 pb-4">
          {NAV_GROUPS.map((group, gi) => (
            <div key={group.heading ?? `g${gi}`} className="mb-4">
              {group.heading && (
                <p className="px-3 pb-1 font-mono text-[10px] tracking-[0.18em] text-muted-foreground uppercase">
                  {group.heading}
                </p>
              )}
              <div className="flex flex-col gap-1">
                {group.items.map((item) => {
                  const active = pathname === item.href;
                  const Icon = item.icon;
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={`relative flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-colors ${
                        active
                          ? "bg-secondary text-foreground"
                          : "text-muted-foreground hover:bg-secondary/50 hover:text-foreground"
                      }`}
                    >
                      {active && (
                        <span className="absolute top-1/2 left-0 h-4 w-0.5 -translate-y-1/2 rounded-full bg-primary shadow-[0_0_8px_rgba(255,255,255,0.6)]" />
                      )}
                      <Icon className="size-4 shrink-0" />
                      {item.label}
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between border-b border-border px-6">
          <nav aria-label="breadcrumb" className="flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground uppercase">
            {(CRUMBS[pathname] ?? []).map((crumb, i, arr) => (
              <span key={crumb} className={i === arr.length - 1 ? "text-foreground" : ""}>
                {crumb}
                {i < arr.length - 1 && <span className="ml-2 text-border">/</span>}
              </span>
            ))}
          </nav>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button variant="ghost" size="icon" className="rounded-full" aria-label="账户菜单">
                  <User />
                </Button>
              }
            />
            <DropdownMenuContent align="end" className="w-44">
              <DropdownMenuGroup>
                <DropdownMenuLabel>管理员</DropdownMenuLabel>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={openPasswordModal}>
                <KeyRound />
                修改密码
              </DropdownMenuItem>
              <DropdownMenuItem variant="destructive" onClick={onLogout}>
                <LogOut />
                退出登录
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </header>
        <main className="bg-grid min-w-0 flex-1 overflow-y-auto px-8 py-8">
          <div className="mx-auto w-full max-w-6xl">{children}</div>
        </main>
      </div>

      <Modal open={pwOpen} onClose={() => setPwOpen(false)} title="修改密码">
        <form onSubmit={onChangePassword} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="old-password" className="text-xs text-muted-foreground">当前密码</Label>
            <Input
              id="old-password"
              type="password"
              autoComplete="current-password"
              value={pwOld}
              onChange={(e) => setPwOld(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password" className="text-xs text-muted-foreground">新密码</Label>
            <Input
              id="new-password"
              type="password"
              autoComplete="new-password"
              value={pwNew}
              onChange={(e) => setPwNew(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">至少 8 个字符。</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-password" className="text-xs text-muted-foreground">确认新密码</Label>
            <Input
              id="confirm-password"
              type="password"
              autoComplete="new-password"
              value={pwConfirm}
              onChange={(e) => setPwConfirm(e.target.value)}
            />
          </div>
          {pwError && <p className="text-sm text-destructive">{pwError}</p>}
          {pwDone && <p className="text-sm text-muted-foreground">{pwDone}</p>}
          <div className="flex justify-end gap-3 pt-2">
            <Button type="button" variant="outline" className="rounded-full" onClick={() => setPwOpen(false)}>
              关闭
            </Button>
            <Button type="submit" className="rounded-full" disabled={pwBusy}>
              {pwBusy ? "提交中…" : "修改密码"}
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
