import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import DashboardPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
  usePathname: () => "/dashboard",
}));

const getStatusMock = vi.fn();
const startMock = vi.fn();
const stopMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getStatus: () => getStatusMock(),
  startSingbox: () => startMock(),
  stopSingbox: () => stopMock(),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  pushMock.mockClear();
  getStatusMock.mockReset();
  startMock.mockReset();
  stopMock.mockReset();
});

describe("DashboardPage", () => {
  it("显示运行状态与版本", async () => {
    getStatusMock.mockResolvedValue({ installed: true, version: "1.13.13", running: true, hasConfig: true });
    render(<DashboardPage />);
    await waitFor(() => expect(screen.getByText(/运行中/)).toBeInTheDocument());
    expect(screen.getByText(/1\.13\.13/)).toBeInTheDocument();
  });

  it("点击启动调用 startSingbox", async () => {
    getStatusMock.mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true });
    startMock.mockResolvedValue({ installed: true, version: "1.13.13", running: true, hasConfig: true });
    render(<DashboardPage />);
    const btn = await screen.findByRole("button", { name: /启动/ });
    await userEvent.click(btn);
    expect(startMock).toHaveBeenCalled();
  });
});
