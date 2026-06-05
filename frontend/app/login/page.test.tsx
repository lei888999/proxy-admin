import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LoginPage from "./page";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: pushMock }) }));
vi.mock("@/lib/api", () => ({ login: vi.fn().mockResolvedValue({ username: "admin" }) }));

beforeEach(() => {
  pushMock.mockClear();
});

describe("LoginPage", () => {
  it("渲染用户名和密码输入", () => {
    render(<LoginPage />);
    expect(screen.getByLabelText(/用户名/)).toBeInTheDocument();
    expect(screen.getByLabelText(/密码/)).toBeInTheDocument();
  });

  it("登录成功后跳转仪表盘", async () => {
    render(<LoginPage />);
    await userEvent.type(screen.getByLabelText(/用户名/), "admin");
    await userEvent.type(screen.getByLabelText(/密码/), "mnice7082");
    await userEvent.click(screen.getByRole("button", { name: /登录/ }));
    expect(pushMock).toHaveBeenCalledWith("/dashboard");
  });
});
