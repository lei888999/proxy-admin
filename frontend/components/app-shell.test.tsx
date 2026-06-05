import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppShell } from "./app-shell";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => "/dashboard",
}));
const getStatusMock = vi.fn().mockResolvedValue({ installed: true, version: "1.14.0", running: false, hasConfig: false });
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
  it("renders nav items 概览 and 配置", () => {
    render(<AppShell><div>内容</div></AppShell>);
    expect(screen.getByRole("link", { name: /概览/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /配置/ })).toBeInTheDocument();
    expect(screen.getByText("内容")).toBeInTheDocument();
  });

  it("logout button calls logout and redirects", async () => {
    render(<AppShell><div /></AppShell>);
    await userEvent.click(screen.getByRole("button", { name: /退出登录/ }));
    expect(logoutMock).toHaveBeenCalled();
    expect(pushMock).toHaveBeenCalledWith("/login");
  });
});
