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
vi.mock("@/lib/api", () => ({
  listInbounds: () => listInboundsMock(),
  createInbound: (...a: unknown[]) => createInboundMock(...a),
  deleteInbound: (...a: unknown[]) => deleteInboundMock(...a),
  listUsers: vi.fn().mockResolvedValue([]),
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
