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
  it("renders username and password fields", () => {
    render(<LoginPage />);
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument();
  });

  it("redirects to dashboard after successful login", async () => {
    render(<LoginPage />);
    await userEvent.type(screen.getByLabelText(/username/i), "admin");
    await userEvent.type(screen.getByLabelText(/password/i), "mnice7082");
    await userEvent.click(screen.getByRole("button", { name: /sign in/i }));
    expect(pushMock).toHaveBeenCalledWith("/dashboard");
  });
});
