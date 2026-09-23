import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import ConfigPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/config",
}));
const getConfigMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getConfig: () => getConfigMock(),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  changePassword: vi.fn().mockResolvedValue(undefined),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  getConfigMock.mockReset();
});

describe("ConfigPage", () => {
  it("载入配置且文本框只读", async () => {
    getConfigMock.mockResolvedValue('{"log":{}}');
    render(<ConfigPage />);
    await waitFor(() => expect(screen.getByRole("textbox")).toHaveValue('{"log":{}}'));
    expect(screen.getByRole("textbox")).toHaveAttribute("readonly");
    expect(screen.queryByRole("button", { name: /保存/ })).toBeNull();
  });
});
