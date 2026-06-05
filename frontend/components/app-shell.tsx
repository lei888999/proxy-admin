"use client";

import { useEffect } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { getStatus, logout, UnauthorizedError } from "@/lib/api";

const NAV = [
  { href: "/dashboard", label: "概览" },
  { href: "/inbounds", label: "入站" },
  { href: "/config", label: "配置" },
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
    <div className="flex min-h-screen bg-background text-foreground">
      <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-card">
        <div className="px-5 py-5">
          <span className="font-mono text-sm tracking-[0.18em] text-foreground uppercase">
            sing-box admin
          </span>
        </div>
        <nav className="flex flex-col gap-1 px-3">
          {NAV.map((item) => {
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
        </nav>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center justify-between border-b border-border px-6">
          <span className="text-sm text-muted-foreground">sing-box 管理面板</span>
          <button
            onClick={onLogout}
            className="rounded-full border border-border px-3 py-1 text-sm text-foreground transition-colors hover:bg-secondary"
          >
            退出登录
          </button>
        </header>
        <main className="min-w-0 flex-1 px-6 py-8">
          <div className="mx-auto max-w-4xl">{children}</div>
        </main>
      </div>
    </div>
  );
}
