import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import DashboardPage from "./page";
import { UnauthorizedError } from "@/lib/api";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: pushMock }) }));

const getStatusMock = vi.fn();
const logoutMock = vi.fn();
vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return { ...actual, getStatus: () => getStatusMock(), logout: () => logoutMock() };
});

beforeEach(() => {
  pushMock.mockClear();
  getStatusMock.mockReset();
  logoutMock.mockReset();
});

describe("DashboardPage", () => {
  it("shows sing-box status when authorized", async () => {
    getStatusMock.mockResolvedValue({ installed: true, version: "1.9.0", running: false });
    render(<DashboardPage />);
    await waitFor(() => expect(screen.getByText(/1\.9\.0/)).toBeInTheDocument());
  });

  it("redirects to login on 401", async () => {
    getStatusMock.mockRejectedValue(new UnauthorizedError());
    render(<DashboardPage />);
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/login"));
  });
});
