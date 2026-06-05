export class UnauthorizedError extends Error {}

export interface SingboxStatus {
  installed: boolean;
  version: string;
  running: boolean;
  hasConfig: boolean;
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(path, { credentials: "include", ...init });
  if (res.status === 401) {
    throw new UnauthorizedError("unauthorized");
  }
  return res;
}

export async function login(username: string, password: string): Promise<{ username: string }> {
  const res = await request("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    throw new Error("login failed");
  }
  return res.json();
}

export async function logout(): Promise<void> {
  await request("/api/auth/logout", { method: "POST" });
}

export async function getStatus(): Promise<SingboxStatus> {
  const res = await request("/api/status");
  if (!res.ok) {
    throw new Error("failed to load status");
  }
  return res.json();
}

async function statusAction(path: string): Promise<SingboxStatus> {
  const res = await request(path, { method: "POST" });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.detail || body.error || "操作失败");
  }
  return res.json();
}

export function startSingbox(): Promise<SingboxStatus> {
  return statusAction("/api/singbox/start");
}

export function stopSingbox(): Promise<SingboxStatus> {
  return statusAction("/api/singbox/stop");
}

export async function getConfig(): Promise<string> {
  const res = await request("/api/singbox/config");
  if (!res.ok) throw new Error("加载配置失败");
  const body = await res.json();
  return body.content as string;
}

export async function saveConfig(content: string): Promise<void> {
  const res = await request("/api/singbox/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || "保存失败");
  }
}
