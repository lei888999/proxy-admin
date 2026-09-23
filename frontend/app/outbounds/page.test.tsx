import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import OutboundsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/outbounds",
}));

const listMock = vi.fn();
const createMock = vi.fn();
const deleteMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listOutbounds: () => listMock(),
  createOutbound: (...a: unknown[]) => createMock(...a),
  updateOutbound: vi.fn().mockResolvedValue({}),
  deleteOutbound: (...a: unknown[]) => deleteMock(...a),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1", running: false, hasConfig: true }),
  logout: vi.fn(),
  changePassword: vi.fn().mockResolvedValue(undefined),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listMock.mockReset().mockResolvedValue([]);
  createMock.mockReset().mockResolvedValue({ id: 1 });
  deleteMock.mockReset().mockResolvedValue({});
});

describe("OutboundsPage", () => {
  it("新建出站提交字段", async () => {
    render(<OutboundsPage />);
    await userEvent.click(await screen.findByRole("button", { name: /新建出站/ }));
    await userEvent.type(screen.getByLabelText(/标签/), "proxyA");
    await userEvent.type(screen.getByLabelText(/服务器/), "1.2.3.4");
    await userEvent.clear(screen.getByLabelText(/端口/));
    await userEvent.type(screen.getByLabelText(/端口/), "1080");
    await userEvent.click(screen.getByRole("button", { name: /创建/ }));
    await waitFor(() => expect(createMock).toHaveBeenCalled());
    const arg = createMock.mock.calls[0][0];
    expect(arg.tag).toBe("proxyA");
    expect(arg.server).toBe("1.2.3.4");
    expect(arg.port).toBe(1080);
  });

  it("删除出站需二次确认", async () => {
    listMock.mockResolvedValue([{ id: 5, tag: "proxyA", type: "socks5", server: "1.2.3.4", port: 1080, username: "", password: "" }]);
    render(<OutboundsPage />);
    await waitFor(() => expect(screen.getByText("proxyA")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: /^删除$/ }));
    expect(deleteMock).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /确认删除/ }));
    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith(5));
  });
});
