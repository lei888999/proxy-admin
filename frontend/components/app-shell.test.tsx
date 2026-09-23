import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppShell } from "./app-shell";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => "/dashboard",
}));

const getStatusMock = vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true });
const logoutMock = vi.fn().mockResolvedValue(undefined);
const changePasswordMock = vi.fn().mockResolvedValue(undefined);
vi.mock("@/lib/api", () => ({
  getStatus: () => getStatusMock(),
  logout: () => logoutMock(),
  changePassword: (...a: unknown[]) => changePasswordMock(...a),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  pushMock.mockClear();
  logoutMock.mockClear();
  changePasswordMock.mockClear();
});

describe("AppShell", () => {
  it("renders grouped nav labels and a scrollable main", () => {
    render(<AppShell><div>content</div></AppShell>);
    // group heading
    expect(screen.getByText("代理", { selector: "p" })).toBeInTheDocument();
    // nav destinations
    expect(screen.getByRole("link", { name: "概览" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "入站" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "用户" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "配置" })).toBeInTheDocument();
    // main is the scroll container
    const main = screen.getByRole("main");
    expect(main.className).toContain("overflow-y-auto");
    // content rendered
    expect(screen.getByText("content")).toBeInTheDocument();
  });

  it("account menu logs out and redirects", async () => {
    render(<AppShell><div /></AppShell>);
    await userEvent.click(screen.getByRole("button", { name: /账户菜单/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: /退出登录/ }));
    expect(logoutMock).toHaveBeenCalled();
    expect(pushMock).toHaveBeenCalledWith("/login");
  });

  it("account menu opens a password change form and submits it", async () => {
    render(<AppShell><div /></AppShell>);
    await userEvent.click(screen.getByRole("button", { name: /账户菜单/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: /修改密码/ }));

    await userEvent.type(screen.getByLabelText("当前密码"), "mnice7082");
    await userEvent.type(screen.getByLabelText("新密码"), "a-much-longer-secret");
    await userEvent.type(screen.getByLabelText("确认新密码"), "a-much-longer-secret");
    await userEvent.click(screen.getByRole("button", { name: "修改密码" }));

    expect(changePasswordMock).toHaveBeenCalledWith("mnice7082", "a-much-longer-secret");
  });

  it("rejects a mismatched confirmation without calling the API", async () => {
    render(<AppShell><div /></AppShell>);
    await userEvent.click(screen.getByRole("button", { name: /账户菜单/ }));
    await userEvent.click(await screen.findByRole("menuitem", { name: /修改密码/ }));

    await userEvent.type(screen.getByLabelText("当前密码"), "mnice7082");
    await userEvent.type(screen.getByLabelText("新密码"), "a-much-longer-secret");
    await userEvent.type(screen.getByLabelText("确认新密码"), "typo-typo-typo");
    await userEvent.click(screen.getByRole("button", { name: "修改密码" }));

    expect(changePasswordMock).not.toHaveBeenCalled();
    expect(screen.getByText("两次输入的新密码不一致")).toBeInTheDocument();
  });
});
