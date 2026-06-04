import { describe, it, expect, vi, beforeEach } from "vitest";
import { login, getStatus, UnauthorizedError } from "./api";

beforeEach(() => {
  vi.restoreAllMocks();
});

describe("api", () => {
  it("login posts credentials with cookies included", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ username: "admin" }), { status: 200 })
    );
    vi.stubGlobal("fetch", fetchMock);

    const res = await login("admin", "mnice7082");
    expect(res.username).toBe("admin");
    const [, init] = fetchMock.mock.calls[0];
    expect(init.credentials).toBe("include");
    expect(init.method).toBe("POST");
  });

  it("getStatus throws UnauthorizedError on 401", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("", { status: 401 })));
    await expect(getStatus()).rejects.toBeInstanceOf(UnauthorizedError);
  });

  it("getStatus returns parsed status on 200", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ installed: true, version: "1.9.0", running: false }), {
          status: 200,
        })
      )
    );
    const st = await getStatus();
    expect(st.version).toBe("1.9.0");
  });
});
