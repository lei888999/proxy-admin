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
const resetKeysMock = vi.fn();
const deleteInboundMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listInbounds: () => listInboundsMock(),
  listInboundTypes: () => listTypesMock(),
  createInbound: (...a: unknown[]) => createInboundMock(...a),
  updateInbound: vi.fn(),
  resetInboundKeys: (...a: unknown[]) => resetKeysMock(...a),
  deleteInbound: (...a: unknown[]) => deleteInboundMock(...a),
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
  createInboundMock.mockReset().mockResolvedValue({ id: 1 });
  resetKeysMock.mockReset().mockResolvedValue({ id: 1 });
  deleteInboundMock.mockReset().mockResolvedValue(undefined);
});

describe("InboundsPage", () => {
  it("点新建入站打开 modal 并能提交（默认协议）", async () => {
    render(<InboundsPage />);
    await userEvent.click(await screen.findByRole("button", { name: /新建入站/ }));
    await userEvent.type(screen.getByLabelText(/标签/), "v9");
    await userEvent.click(screen.getByRole("button", { name: /创建/ }));
    await waitFor(() => expect(createInboundMock).toHaveBeenCalled());
    expect(createInboundMock.mock.calls[0][0]).toBe("vless-reality");
  });

  it("重置密钥需二次确认", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, type: "vless-reality", tag: "v1", port: 8443, network: "tcp", publicInfo: { realityPublicKey: "PUB" } },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("v1")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: /重置密钥/ }));
    expect(resetKeysMock).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /确认重置/ }));
    await waitFor(() => expect(resetKeysMock).toHaveBeenCalledWith(1));
  });

  it("删除入站需二次确认", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, type: "vless-reality", tag: "v1", port: 8443, network: "tcp", publicInfo: {} },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("v1")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: /^删除$/ }));
    expect(deleteInboundMock).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /确认删除/ }));
    await waitFor(() => expect(deleteInboundMock).toHaveBeenCalledWith(1));
  });
});
