import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LogsPage from "./page";

const logsHook = vi.fn();
const refresh = vi.fn();
vi.mock("@/lib/hooks", () => ({
  useConnectionLogs: (auto: boolean) => logsHook(auto),
  useUsers: () => ({ data: [{ id: 7, name: "alice" }] }),
}));
vi.mock("@/components/app-shell", () => ({ AppShell: ({ children }: { children: React.ReactNode }) => children }));
const sample = {
  available: true, truncated: false, updatedAt: "2026-10-10T12:00:00Z",
  entries: [
    { id: "1", time: "INFO[0001]", user: "u7", inbound: "vless", source: "192.0.2.1:1234", target: "kiro.example:443", outbounds: [{ type: "http", tag: "美国出口", target: "kiro.example:443" }], error: "proxy returned 502" },
    { id: "2", time: "INFO[0002]", user: "u8", inbound: "vless", source: "", target: "example.com:443", outbounds: [{ type: "direct", tag: "direct", target: "example.com:443" }], error: "" },
    { id: "3", time: "INFO[0003]", user: "", inbound: "vless", source: "", target: "", outbounds: [], error: "TLS handshake failed" },
  ],
};
beforeEach(() => {
  refresh.mockReset();
  logsHook.mockReset().mockReturnValue({ data: sample, mutate: refresh, isLoading: false, isValidating: false });
});
describe("Connection logs", () => {
  it("shows actual proxy, VPS direct and unknown separately, with failures", () => {
    render(<LogsPage />);
    expect(screen.getByText("上游出站 · 美国出口")).toBeInTheDocument();
    expect(screen.getByText("服务器直连（VPS 出口）")).toBeInTheDocument();
    expect(screen.getByText("未记录出站")).toBeInTheDocument();
    expect(screen.getByText("alice (u7)")).toBeInTheDocument();
    expect(screen.getByText("proxy returned 502")).toBeInTheDocument();
    expect(screen.getByText(/客户端本机直连不会经过服务器/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("搜索日志"), { target: { value: "KIRO" } });
    expect(screen.queryByText("服务器直连（VPS 出口）")).not.toBeInTheDocument();
    expect(screen.getByText("上游出站 · 美国出口")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "暂停自动刷新" }));
    expect(logsHook).toHaveBeenLastCalledWith(false);
    fireEvent.click(screen.getByRole("button", { name: "刷新日志" }));
    expect(refresh).toHaveBeenCalled();
  });
  it("filters failures without labelling them direct", async () => {
    const user = userEvent.setup();
    render(<LogsPage />);
    await user.click(screen.getByLabelText("出口筛选"));
    await user.click(screen.getByRole("option", { name: "有错误" }));
    expect(screen.queryByText("服务器直连（VPS 出口）")).not.toBeInTheDocument();
    expect(screen.getByText("TLS handshake failed")).toBeInTheDocument();
  });
  it("distinguishes failed reads from an empty log", () => {
    logsHook.mockReturnValue({ error: new Error("unavailable"), mutate: refresh });
    render(<LogsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("加载连接日志失败");
    expect(screen.queryByText(/尚未生成 sing-box 日志/)).not.toBeInTheDocument();
  });
});
