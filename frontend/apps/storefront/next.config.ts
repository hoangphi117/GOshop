import path from "path";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Minimal container image: `next build` emits .next/standalone with only
  // the traced runtime files (server.js + trimmed node_modules) — the runner
  // stage runs `node apps/storefront/server.js` with no pnpm/full install.
  output: "standalone",
  // Monorepo: trace from the pnpm workspace root so @goshop/* packages in
  // ../.. are bundled. Docs pattern: path.join(__dirname, "../../").
  outputFileTracingRoot: path.join(__dirname, "../../"),
  images: {
    // DB/API carry absolute browser-facing media URLs
    // (S3_PUBLIC_BASE_URL=http://localhost/goshop-media). The internal image
    // optimizer fetches that URL FROM the storefront container, where
    // "localhost" is itself — fetch failed -> 500. Plain <img> and
    // product-detail pages work, so serve sources as-is and let the edge
    // (7d immutable cache) do the caching.
    // ponytail: ceiling = no resize/AVIF. Upgrade = give the edge a
    // browser-and-container resolvable host (network alias + hosts entry),
    // sign media URLs with it, and re-enable the optimizer.
    unoptimized: true,
    dangerouslyAllowLocalIP: true,
    remotePatterns: [
      {
        protocol: "https",
        hostname: "images.unsplash.com",
      },
      {
        protocol: "https",
        hostname: "plus.unsplash.com",
      },
      {
        protocol: "https",
        hostname: "**",
      },
      {
        protocol: "http",
        hostname: "localhost",
      },
      {
        protocol: "http",
        hostname: "host.docker.internal",
        port: "9000",
      },
      {
        protocol: "http",
        hostname: "localhost",
        port: "8080",
      },
      {
        protocol: "http",
        hostname: "127.0.0.1",
      },
      {
        protocol: "http",
        hostname: "127.0.0.1",
        port: "9000",
      },
      {
        protocol: "http",
        hostname: "127.0.0.1",
        port: "8080",
      },
      {
        protocol: "http",
        hostname: "minio",
        port: "9000",
      },
    ],
  },
  async rewrites() {
    const backendUrl = (
      process.env.INTERNAL_API_URL ||
      process.env.NEXT_PUBLIC_API_URL ||
      "http://localhost:8080"
    ).replace(/\/$/, "");

    return [
      {
        source: "/uploads/:path*",
        destination: `${backendUrl}/uploads/:path*`,
      },
    ];
  },
};

export default nextConfig;
