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

export interface InboundType {
  type: string;
  label: string;
  network: string;
  defaultPort: number;
}

export interface Inbound {
  id: number;
  type: string;
  tag: string;
  port: number;
  network: string;
  publicInfo: Record<string, unknown>;
}

export interface User {
  id: number;
  name: string;
  uuid: string;
  password: string;
  subToken: string;
  upBytes: number;
  downBytes: number;
  inboundIds: number[];
  inboundTags: string[];
}

export async function listInbounds(): Promise<Inbound[]> {
  const res = await request("/api/inbounds");
  if (!res.ok) throw new Error("加载入站失败");
  return res.json();
}

export async function listInboundTypes(): Promise<InboundType[]> {
  const res = await request("/api/inbound-types");
  if (!res.ok) throw new Error("加载协议类型失败");
  return res.json();
}

export async function createInbound(
  type: string,
  tag: string,
  port: number,
  params: Record<string, unknown>,
): Promise<Inbound> {
  const res = await request("/api/inbounds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ type, tag, port, params }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建入站失败");
  }
  return res.json();
}

export async function deleteInbound(id: number): Promise<void> {
  const res = await request(`/api/inbounds/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("删除入站失败");
}

export async function listUsers(): Promise<User[]> {
  const res = await request("/api/users");
  if (!res.ok) throw new Error("加载用户失败");
  return res.json();
}

export async function createUser(name: string, inboundIds: number[]): Promise<User> {
  const res = await request("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "创建用户失败");
  }
  return res.json();
}

export async function updateUser(id: number, name: string, inboundIds: number[]): Promise<User> {
  const res = await request(`/api/users/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds }),
  });
  if (!res.ok) throw new Error("更新用户失败");
  return res.json();
}

export async function resetUserCreds(id: number): Promise<User> {
  const res = await request(`/api/users/${id}/reset`, { method: "POST" });
  if (!res.ok) throw new Error("重置凭证失败");
  return res.json();
}

export async function deleteUser(id: number): Promise<void> {
  const res = await request(`/api/users/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("删除用户失败");
}

export async function updateInbound(id: number, tag: string, port: number, params: Record<string, unknown>): Promise<Inbound> {
  const res = await request(`/api/inbounds/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tag, port, params }),
  });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.error || "更新入站失败");
  }
  return res.json();
}

export async function resetInboundKeys(id: number): Promise<Inbound> {
  const res = await request(`/api/inbounds/${id}/reset-keys`, { method: "POST" });
  if (!res.ok) throw new Error("重置密钥失败");
  return res.json();
}

export async function applySingbox(): Promise<SingboxStatus> {
  const res = await request("/api/singbox/apply", { method: "POST" });
  if (!res.ok) {
    const b = await res.json().catch(() => ({}));
    throw new Error(b.detail || b.error || "应用失败");
  }
  return res.json();
}

export interface LiveTraffic {
  up: number;
  down: number;
}

export async function getLiveTraffic(): Promise<LiveTraffic> {
  const res = await request("/api/traffic/live");
  if (!res.ok) throw new Error("加载实时流量失败");
  return res.json();
}

export async function resetUserTraffic(id: number): Promise<void> {
  const res = await request(`/api/users/${id}/reset-traffic`, { method: "POST" });
  if (!res.ok) throw new Error("重置流量失败");
}
