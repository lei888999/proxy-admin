import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ConfigPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/config",
}));
const getConfigMock = vi.fn();
const saveConfigMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getConfig: () => getConfigMock(),
  saveConfig: (c: string) => saveConfigMock(c),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.14.0", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  getConfigMock.mockReset();
  saveConfigMock.mockReset();
});

describe("ConfigPage", () => {
  it("载入既有配置到文本框", async () => {
    getConfigMock.mockResolvedValue('{"log":{}}');
    render(<ConfigPage />);
    await waitFor(() => expect(screen.getByRole("textbox")).toHaveValue('{"log":{}}'));
  });

  it("保存调用 saveConfig", async () => {
    getConfigMock.mockResolvedValue("");
    saveConfigMock.mockResolvedValue(undefined);
    render(<ConfigPage />);
    const ta = await screen.findByRole("textbox");
    await userEvent.type(ta, '{{"log":{{}}}');
    await userEvent.click(screen.getByRole("button", { name: /保存配置/ }));
    await waitFor(() => expect(saveConfigMock).toHaveBeenCalled());
  });
});
