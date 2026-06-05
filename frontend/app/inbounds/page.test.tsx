import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import InboundsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/inbounds",
}));

const listInboundsMock = vi.fn();
const createInboundMock = vi.fn();
const deleteInboundMock = vi.fn();
const listUsersMock = vi.fn().mockResolvedValue([]);
vi.mock("@/lib/api", () => ({
  listInbounds: () => listInboundsMock(),
  createInbound: (...a: unknown[]) => createInboundMock(...a),
  deleteInbound: (...a: unknown[]) => deleteInboundMock(...a),
  listUsers: () => listUsersMock(),
  createUser: vi.fn(),
  deleteUser: vi.fn(),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listInboundsMock.mockReset();
  createInboundMock.mockReset();
  deleteInboundMock.mockReset();
  listUsersMock.mockClear();
});

describe("InboundsPage", () => {
  it("渲染入站列表", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, tag: "v1", port: 443, flow: "xtls-rprx-vision", realityPublicKey: "PUB", realityShortId: "deadbeef", serverName: "www.microsoft.com", users: [] },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("v1")).toBeInTheDocument());
    expect(screen.getByText(/443/)).toBeInTheDocument();
  });

  it("用预加载的用户渲染，不在挂载时再拉取", async () => {
    listInboundsMock.mockResolvedValue([
      { id: 1, tag: "v1", port: 443, flow: "xtls-rprx-vision", realityPublicKey: "PUB", realityShortId: "deadbeef", serverName: "www.microsoft.com", users: [{ id: 9, inboundId: 1, name: "alice", uuid: "u-1" }] },
    ]);
    render(<InboundsPage />);
    await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
    expect(listUsersMock).not.toHaveBeenCalled();
  });

  it("新建入站调用 createInbound", async () => {
    listInboundsMock.mockResolvedValue([]);
    createInboundMock.mockResolvedValue({ id: 1, tag: "v2", port: 8443, users: [] });
    render(<InboundsPage />);
    await screen.findByRole("button", { name: /新建入站/ });
    await userEvent.type(screen.getByLabelText(/标签/), "v2");
    await userEvent.type(screen.getByLabelText(/端口/), "8443");
    await userEvent.click(screen.getByRole("button", { name: /新建入站/ }));
    await waitFor(() => expect(createInboundMock).toHaveBeenCalled());
  });
});
