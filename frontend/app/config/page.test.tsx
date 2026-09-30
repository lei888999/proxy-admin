import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { SWRConfig } from "swr";
import userEvent from "@testing-library/user-event";
import ConfigPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }), usePathname: () => "/config",
}));
const getConfigMock = vi.fn();
const listRulesMock = vi.fn();
const saveRulesMock = vi.fn();
vi.mock("@/lib/api", () => ({
  getConfig: () => getConfigMock(),
  listRouteRules: () => listRulesMock(),
  saveRouteRules: (...args: unknown[]) => saveRulesMock(...args),
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(), changePassword: vi.fn().mockResolvedValue(undefined),
  UnauthorizedError: class extends Error {},
}));

beforeEach(() => {
  getConfigMock.mockReset().mockResolvedValue('{"log":{}}');
  listRulesMock.mockReset().mockResolvedValue([]);
  saveRulesMock.mockReset().mockResolvedValue({ ok: true });
});
function renderPage() {
  return render(<SWRConfig value={{ provider: () => new Map() }}><ConfigPage /></SWRConfig>);
}

describe("ConfigPage", () => {
  it("显示只读服务端配置和客户端规则说明", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByLabelText("服务端配置")).toHaveValue('{"log":{}}'));
    expect(screen.getByLabelText("服务端配置")).toHaveAttribute("readonly");
    expect(screen.getByText(/仅写入客户端订阅/)).toBeInTheDocument();
  });
  it("按调整后的顺序保存客户端规则", async () => {
    const first = { type: "DOMAIN", value: "example.com", policy: "REJECT" };
    const second = { type: "IP-CIDR", value: "8.8.8.0/24", policy: "节点", noResolve: true };
    listRulesMock.mockResolvedValue([first, second]);
    renderPage();
    await screen.findByDisplayValue("example.com");
    fireEvent.click(screen.getByRole("button", { name: "上移规则 2" }));
    fireEvent.click(screen.getByRole("button", { name: "保存客户端规则" }));
    await waitFor(() => expect(saveRulesMock).toHaveBeenCalledWith([second, first]));
    expect(await screen.findByText(/请在客户端更新订阅/)).toBeInTheDocument();
  });
  it("阻止空白规则提交并显示服务端校验错误", async () => {
    renderPage();
    await waitFor(() => expect(screen.getByRole("button", { name: "添加规则" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: "添加规则" }));
    fireEvent.click(screen.getByRole("button", { name: "保存客户端规则" }));
    expect(screen.getByRole("status")).toHaveTextContent("匹配值");
    expect(saveRulesMock).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("匹配值 1"), { target: { value: "bad/domain" } });
    saveRulesMock.mockRejectedValue(new Error("第 1 条规则：域名格式不正确"));
    fireEvent.click(screen.getByRole("button", { name: "保存客户端规则" }));
    expect(await screen.findByText("第 1 条规则：域名格式不正确")).toBeInTheDocument();
    expect(screen.getByLabelText("匹配值 1")).toHaveValue("bad/domain");
  });
  it("切换为远程规则集并保存完整来源配置", async () => {
    const user = userEvent.setup();
    renderPage();
    await waitFor(() => expect(screen.getByRole("button", { name: "添加规则" })).toBeEnabled());
    await user.click(screen.getByRole("button", { name: "添加规则" }));
    await user.click(screen.getByLabelText("规则类型 1"));
    await user.click(await screen.findByRole("option", { name: "远程规则集 · RULE-SET" }));
    await user.type(screen.getByLabelText("匹配值 1"), "ads");
    await user.type(screen.getByLabelText("规则集 HTTPS 地址"), "https://example.com/ads.yaml");
    await user.click(screen.getByLabelText("客户端策略 1"));
    await user.click(await screen.findByRole("option", { name: "拒绝连接" }));
    await user.click(screen.getByRole("button", { name: "保存客户端规则" }));
    await waitFor(() => expect(saveRulesMock).toHaveBeenCalledWith([{
      type: "RULE-SET", value: "ads", policy: "REJECT", noResolve: false,
      provider: { url: "https://example.com/ads.yaml", behavior: "domain", format: "yaml" },
    }]));
  });

});
