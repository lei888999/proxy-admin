import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  images: { unoptimized: true },
  // Dev-only proxy so the browser sees same-origin /api (cookies + no CORS).
  // rewrites are ignored during `next build` with output:'export', which is fine
  // because production serves /api from the same Go binary.
  async rewrites() {
    return [{ source: "/api/:path*", destination: "http://localhost:8080/api/:path*" }];
  },
};

export default nextConfig;
