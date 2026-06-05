import { describe, it, expect, vi, beforeEach } from "vitest";
import { login, getStatus, UnauthorizedError, startSingbox, getConfig, saveConfig, listInbounds, listInboundTypes, createInbound, applySingbox } from "./api";

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

  it("listInbounds returns array", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify([{ id: 1, tag: "v1" }]), { status: 200 })));
    const ins = await listInbounds();
    expect(ins[0].tag).toBe("v1");
  });

  it("listInboundTypes returns array", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify([{ type: "hysteria2", label: "Hysteria2", network: "udp", defaultPort: 443 }]), { status: 200 })));
    const ts = await listInboundTypes();
    expect(ts[0].type).toBe("hysteria2");
  });

  it("createInbound posts type/tag/port/params", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 1 }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    await createInbound("hysteria2", "h1", 443, { upMbps: 50 });
    const [, init] = fetchMock.mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body.type).toBe("hysteria2");
    expect(body.params.upMbps).toBe(50);
  });

  it("applySingbox throws backend error on failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: "sing-box not installed" }), { status: 400 })));
    await expect(applySingbox()).rejects.toThrow(/not installed/);
  });
});
