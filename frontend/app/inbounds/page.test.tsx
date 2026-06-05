import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import InboundsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/inbounds",
}));

const listInboundsMock = vi.fn();
const listTypesMock = vi.fn();
const createInboundMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listInbounds: () => listInboundsMock(),
  listInboundTypes: () => listTypesMock(),
  createInbound: (...a: unknown[]) => createInboundMock(...a),
  deleteInbound: vi.fn(),
  listUsers: vi.fn().mockResolvedValue([]),
  createUser: vi.fn(),
  deleteUser: vi.fn(),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listInboundsMock.mockReset().mockResolvedValue([]);
  listTypesMock.mockReset().mockResolvedValue([
    { type: "vless-reality", label: "VLESS-Reality", network: "tcp", defaultPort: 8443 },
    { type: "hysteria2", label: "Hysteria2", network: "udp", defaultPort: 443 },
  ]);
  createInboundMock.mockReset().mockResolvedValue({ id: 1, type: "hysteria2", tag: "h1", port: 443, network: "udp", publicInfo: {}, users: [] });
});

describe("InboundsPage", () => {
  it("渲染协议下拉与默认 vless 字段", async () => {
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByLabelText(/协议/)).toBeInTheDocument());
    expect(screen.getByLabelText(/握手域名/)).toBeInTheDocument();
  });

  it("切到 hysteria2 显示限速字段并提交 params", async () => {
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByLabelText(/协议/)).toBeInTheDocument());
    await userEvent.selectOptions(screen.getByLabelText(/协议/), "hysteria2");
    expect(screen.getByLabelText(/上行/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText(/标签/), "h1");
    await userEvent.click(screen.getByRole("button", { name: /新建入站/ }));
    await waitFor(() => expect(createInboundMock).toHaveBeenCalled());
    expect(createInboundMock.mock.calls[0][0]).toBe("hysteria2");
  });

  it("按类型展示入站卡片 publicInfo", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, type: "vless-reality", tag: "v1", port: 8443, network: "tcp", publicInfo: { realityPublicKey: "PUB", shortId: "ab", serverName: "x", flow: "f" }, users: [] },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("v1")).toBeInTheDocument());
    expect(screen.getByText("PUB")).toBeInTheDocument();
  });
});
