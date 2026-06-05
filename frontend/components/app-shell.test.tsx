import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AppShell } from "./app-shell";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/dashboard",
}));

vi.mock("@/lib/api", () => ({
  getStatus: vi.fn().mockResolvedValue({ installed: true, version: "1.13.13", running: false, hasConfig: true }),
  logout: vi.fn(),
  UnauthorizedError: class extends Error {},
}));

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
});
