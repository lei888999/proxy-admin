export class UnauthorizedError extends Error {}

export interface SingboxStatus {
  installed: boolean;
  version: string;
  /** The PANEL-MANAGED process only. */
  running: boolean;
  hasConfig: boolean;
  /**
   * A sing-box running outside the panel. The panel never touches it, but it
   * explains a start that fails on a bound port and the Clash-API errors the
   * traffic poller would otherwise log.
   */
  external: boolean;
}

/**
 * A mutation's response. `configApplyError` means the change WAS saved but
 * applying it to sing-box failed — a warning to show, not a failure to retry.
 */
export type Mutated<T> = T & { configApplyError?: string };

export type ApplyWarning = { configApplyError?: string };

async function request(path: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(path, { credentials: "include", ...init });
  if (res.status === 401) {
    throw new UnauthorizedError("unauthorized");
  }
  return res;
}

/** errorFrom reads the server's message, falling back to a generic one. */
async function errorFrom(res: Response, fallback: string): Promise<Error> {
  const body = await res.json().catch(() => ({}));
  return new Error(body.detail || body.error || fallback);
}

export async function login(username: string, password: string): Promise<{ username: string }> {
  const res = await fetch("/api/auth/login", {
    credentials: "include",
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    // 401 here is a wrong password, not an expired session, so it must not be
    // turned into UnauthorizedError; 429 carries the lockout message.
    throw await errorFrom(res, "用户名或密码错误");
  }
  // The browser has already accepted Set-Cookie when fetch resolves. Parsing a
  // decorative JSON body after that must not turn a successful login into a
  // false failure if a reverse proxy strips/truncates the response body.
  return { username };
}

// Mutations have committed once their response has a 2xx status. The returned
// JSON is only optional display data (the apply warning/entity), so do not
// falsely claim "创建失败" merely because an intermediary damaged that body.
async function mutationJSON<T>(res: Response): Promise<T> {
  return res.json().catch(() => ({}) as T);
}

export async function logout(): Promise<void> {
  await request("/api/auth/logout", { method: "POST" });
}

export async function changePassword(oldPassword: string, newPassword: string): Promise<void> {
  const res = await request("/api/auth/password", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ oldPassword, newPassword }),
  });
  if (!res.ok) throw await errorFrom(res, "修改密码失败");
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
  if (!res.ok) throw await errorFrom(res, "操作失败");
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
  fields: InboundField[];
}

export interface InboundField {
  name: string;
  label: string;
  type: "text" | "number";
  default?: string | number;
  required?: boolean;
}

export interface Inbound {
  id: number;
  type: string;
  tag: string;
  port: number;
  network: string;
  publicInfo: Record<string, unknown>;
  /** Set when the stored settings no longer decode, so the row is not just blank. */
  settingsError?: string;
}

export interface User {
  id: number;
  name: string;
  uuid: string;
  password: string;
  subToken: string;
  upBytes: number;
  downBytes: number;
  outboundId: number | null;
  inboundIds: number[];
  inboundTags: string[];
}

export interface Outbound {
  id: number;
  tag: string;
  type: string;
  server: string;
  port: number;
  username: string;
  password: string;
}

export interface OutboundProbe {
  tag: string;
  server: string;
  port: number;
  reachable: boolean;
  latencyMs?: number;
  error?: string;
}

export async function probeOutbounds(): Promise<OutboundProbe[]> {
  const res = await request("/api/traffic/diagnostics/outbounds");
  if (!res.ok) throw await errorFrom(res, "线路检测失败");
  const body = await res.json();
  return body.probes as OutboundProbe[];
}

export async function listOutbounds(): Promise<Outbound[]> {
  const res = await request("/api/outbounds");
  if (!res.ok) throw new Error("加载出站失败");
  return res.json();
}

export async function createOutbound(o: Omit<Outbound, "id">): Promise<Mutated<Outbound>> {
  const res = await request("/api/outbounds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(o),
  });
  if (!res.ok) throw await errorFrom(res, "创建出站失败");
  return mutationJSON<Mutated<Outbound>>(res);
}

export async function updateOutbound(id: number, o: Omit<Outbound, "id">): Promise<Mutated<Outbound>> {
  const res = await request(`/api/outbounds/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(o),
  });
  if (!res.ok) throw await errorFrom(res, "更新出站失败");
  return mutationJSON<Mutated<Outbound>>(res);
}

export async function deleteOutbound(id: number): Promise<ApplyWarning> {
  const res = await request(`/api/outbounds/${id}`, { method: "DELETE" });
  if (!res.ok) throw await errorFrom(res, "删除出站失败");
  return mutationJSON<ApplyWarning>(res);
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
): Promise<Mutated<Inbound>> {
  const res = await request("/api/inbounds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ type, tag, port, params }),
  });
  if (!res.ok) throw await errorFrom(res, "创建入站失败");
  return mutationJSON<Mutated<Inbound>>(res);
}

export async function deleteInbound(id: number): Promise<ApplyWarning> {
  const res = await request(`/api/inbounds/${id}`, { method: "DELETE" });
  if (!res.ok) throw await errorFrom(res, "删除入站失败");
  return mutationJSON<ApplyWarning>(res);
}

export async function listUsers(): Promise<User[]> {
  const res = await request("/api/users");
  if (!res.ok) throw new Error("加载用户失败");
  return res.json();
}

export async function createUser(name: string, inboundIds: number[], outboundId: number | null): Promise<Mutated<User>> {
  const res = await request("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds, outboundId }),
  });
  if (!res.ok) throw await errorFrom(res, "创建用户失败");
  return mutationJSON<Mutated<User>>(res);
}

export async function updateUser(id: number, name: string, inboundIds: number[], outboundId: number | null): Promise<Mutated<User>> {
  const res = await request(`/api/users/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, inboundIds, outboundId }),
  });
  if (!res.ok) throw await errorFrom(res, "更新用户失败");
  return mutationJSON<Mutated<User>>(res);
}

export async function resetUserCreds(id: number): Promise<Mutated<User>> {
  const res = await request(`/api/users/${id}/reset`, { method: "POST" });
  if (!res.ok) throw await errorFrom(res, "重置凭证失败");
  return mutationJSON<Mutated<User>>(res);
}

export async function deleteUser(id: number): Promise<ApplyWarning> {
  const res = await request(`/api/users/${id}`, { method: "DELETE" });
  if (!res.ok) throw await errorFrom(res, "删除用户失败");
  return mutationJSON<ApplyWarning>(res);
}

export async function updateInbound(id: number, tag: string, port: number, params: Record<string, unknown>): Promise<Mutated<Inbound>> {
  const res = await request(`/api/inbounds/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tag, port, params }),
  });
  if (!res.ok) throw await errorFrom(res, "更新入站失败");
  return mutationJSON<Mutated<Inbound>>(res);
}

export async function resetInboundKeys(id: number): Promise<Mutated<Inbound>> {
  const res = await request(`/api/inbounds/${id}/reset-keys`, { method: "POST" });
  if (!res.ok) throw await errorFrom(res, "重置密钥失败");
  return mutationJSON<Mutated<Inbound>>(res);
}

export async function applySingbox(): Promise<SingboxStatus> {
  const res = await request("/api/singbox/apply", { method: "POST" });
  if (!res.ok) throw await errorFrom(res, "应用失败");
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
