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
vi.mock("@/lib/api", () => ({
  getStatus: () => getStatusMock(),
  logout: () => logoutMock(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  pushMock.mockClear();
  logoutMock.mockClear();
});

describe("AppShell", () => {
  it("renders grouped nav labels and a scrollable main", () => {
    render(<AppShell><div>content</div></AppShell>);
    // group heading
    expect(screen.getByText("概览", { selector: "p" })).toBeInTheDocument();
    // nav destinations
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
});
