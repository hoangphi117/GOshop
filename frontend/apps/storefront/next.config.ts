import path from "path";
import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Minimal container image: `next build` emits .next/standalone with only
  // the traced runtime files (server.js + trimmed node_modules) — the runner
  // stage runs `node apps/storefront/server.js` with no pnpm/full install.
  // Minimal container image: `next build` emits .next/standalone with only
  // the traced runtime files (server.js + trimmed node_modules) — the runner
  // stage runs `node apps/storefront/server.js` with no pnpm/full install.
  //
  // Vercel does its own packaging and rejects standalone: the build dies
  // reading .next/next-server.js.nft.json, which Vercel never writes. So it is
  // opt-in, set by the Docker template only.
  output: process.env.NEXT_OUTPUT_STANDALONE === "true" ? "standalone" : undefined,
  // Monorepo: trace from the pnpm workspace root so @goshop/* packages in
  // ../.. are bundled. Docs pattern: path.join(__dirname, "../../").
  outputFileTracingRoot: path.join(__dirname, "../../"),
  // Dev-only: the Cloudflare tunnel exposes this server under a hostname that
  // does not match the origin, and Next 16 rejects every dev resource (HMR,
  // server actions) from an unknown origin. Production builds ignore this.
  allowedDevOrigins: ["dev.goshop.dpdns.org"],
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
        // Dev-through-tunnel: `next dev` runs on the host with no server in
        // front of it, so a browser on the public tunnel origin
        // (dev.goshop.dpdns.org) would have to call the API at localhost — a
        // public -> loopback fetch, which Chrome blocks as Local Network
        // Access ("Permission was denied ... loopback address space"). Proxying
        // /api/v1 through the dev server keeps the browser same-origin; the
        // upstream is INTERNAL_API_URL (the local edge on :80), which injects
        // X-API-KEY. Dormant in the containerized deploy: nginx owns /api/v1
        // before the storefront location, so Next never sees these paths.
        source: "/api/v1/:path*",
        destination: `${backendUrl}/api/v1/:path*`,
      },
      {
        source: "/uploads/:path*",
        destination: `${backendUrl}/uploads/:path*`,
      },
    ];
  },
};

export default nextConfig;
