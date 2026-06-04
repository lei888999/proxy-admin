export class UnauthorizedError extends Error {}

export interface SingboxStatus {
  installed: boolean;
  version: string;
  running: boolean;
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
