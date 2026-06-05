import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import UsersPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/users",
}));

const listUsersMock = vi.fn();
const listInboundsMock = vi.fn();
const createUserMock = vi.fn();
const deleteUserMock = vi.fn();
vi.mock("@/lib/api", () => ({
  listUsers: () => listUsersMock(),
  listInbounds: () => listInboundsMock(),
  createUser: (...a: unknown[]) => createUserMock(...a),
  updateUser: vi.fn(),
  resetUserCreds: vi.fn(),
  deleteUser: (...a: unknown[]) => deleteUserMock(...a),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  listUsersMock.mockReset().mockResolvedValue([]);
  listInboundsMock.mockReset().mockResolvedValue([
    { id: 1, type: "vless-reality", tag: "v1", port: 8443, network: "tcp", publicInfo: {} },
  ]);
  createUserMock.mockReset().mockResolvedValue({ id: 1 });
  deleteUserMock.mockReset().mockResolvedValue(undefined);
});

describe("UsersPage", () => {
  it("列表显示用户的 UUID 与明文密码", async () => {
    listUsersMock.mockResolvedValue([
      { id: 1, name: "alice", uuid: "the-uuid", password: "the-pass", subToken: "t", upBytes: 0, downBytes: 0, inboundIds: [1], inboundTags: ["v1"] },
    ]);
    render(<UsersPage />);
    await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
    expect(screen.getByText("the-uuid")).toBeInTheDocument();
    expect(screen.getByText("the-pass")).toBeInTheDocument();
  });

  it("新建用户 modal 勾选入站并提交", async () => {
    render(<UsersPage />);
    await userEvent.click(await screen.findByRole("button", { name: /新建用户/ }));
    await userEvent.type(screen.getByLabelText(/名称/), "bob");
    await userEvent.click(screen.getByLabelText(/v1/));
    await userEvent.click(screen.getByRole("button", { name: /创建/ }));
    await waitFor(() => expect(createUserMock).toHaveBeenCalled());
    expect(createUserMock.mock.calls[0][0]).toBe("bob");
    expect(createUserMock.mock.calls[0][1]).toEqual([1]);
  });

  it("删除用户需二次确认", async () => {
    listUsersMock.mockResolvedValue([
      { id: 7, name: "alice", uuid: "u", password: "p", subToken: "t", upBytes: 0, downBytes: 0, inboundIds: [], inboundTags: [] },
    ]);
    render(<UsersPage />);
    await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: /^删除$/ }));
    expect(deleteUserMock).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /确认删除/ }));
    await waitFor(() => expect(deleteUserMock).toHaveBeenCalledWith(7));
  });

  it("订阅按钮复制订阅链接", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    listUsersMock.mockResolvedValue([
      { id: 1, name: "alice", uuid: "u", password: "p", subToken: "tok123", upBytes: 0, downBytes: 0, inboundIds: [], inboundTags: [] },
    ]);
    render(<UsersPage />);
    await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: /订阅/ }));
    expect(writeText).toHaveBeenCalledWith(expect.stringContaining("/sub/tok123"));
  });
});
