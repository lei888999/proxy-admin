import { describe, it, expect, vi, beforeEach } from "vitest";
import { login, getStatus, UnauthorizedError, startSingbox, getConfig, saveConfig } from "./api";

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

  it("startSingbox posts and returns status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ installed: true, version: "1.13.13", running: true, hasConfig: true }), { status: 200 })
      )
    );
    const st = await startSingbox();
    expect(st.running).toBe(true);
  });

  it("startSingbox throws with backend detail on 400", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "invalid config", detail: "bad inbound" }), { status: 400 }))
    );
    await expect(startSingbox()).rejects.toThrow(/bad inbound/);
  });

  it("getConfig returns content string", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ content: "{}" }), { status: 200 })));
    expect(await getConfig()).toBe("{}");
  });

  it("saveConfig PUTs content", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await saveConfig('{"log":{}}');
    const [, init] = fetchMock.mock.calls[0];
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body).content).toBe('{"log":{}}');
  });
});
