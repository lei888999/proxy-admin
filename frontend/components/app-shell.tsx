"use client";

import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { getStatus, logout, UnauthorizedError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";
import { User, LogOut } from "lucide-react";

const NAV_GROUPS: { heading: string; items: { href: string; label: string }[] }[] = [
  {
    heading: "概览",
    items: [
      { href: "/dashboard", label: "概览" },
      { href: "/inbounds", label: "入站" },
      { href: "/outbounds", label: "出站" },
      { href: "/users", label: "用户" },
    ],
  },
  {
    heading: "系统",
    items: [{ href: "/config", label: "配置" }],
  },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    getStatus().catch((err) => {
      if (err instanceof UnauthorizedError) router.push("/login");
    });
  }, [router]);

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
          {NAV_GROUPS.map((group) => (
            <div key={group.heading} className="mb-4">
              <p className="px-3 pb-1 font-mono text-[10px] tracking-[0.18em] text-muted-foreground uppercase">
                {group.heading}
              </p>
              <div className="flex flex-col gap-1">
                {group.items.map((item) => {
                  const active = pathname === item.href;
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={`rounded-lg px-3 py-2 text-sm transition-colors ${
                        active ? "bg-secondary text-foreground" : "text-muted-foreground hover:text-foreground"
                      }`}
                    >
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
        <header className="flex h-14 shrink-0 items-center justify-end border-b border-border px-6">
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
              <DropdownMenuItem variant="destructive" onClick={onLogout}>
                <LogOut />
                退出登录
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </header>
        <main className="min-w-0 flex-1 overflow-y-auto px-8 py-8">
          <div className="mx-auto w-full max-w-6xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
