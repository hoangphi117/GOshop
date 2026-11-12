/// <reference types="vitest/config" />
import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import { playwright } from '@vitest/browser-playwright'

const PROXY_TARGET = process.env.DASHBOARD_PROXY_TARGET ?? 'http://localhost:8080'

// https://vite.dev/config/
// Served under /admin/ behind the prod edge (nginx strips the prefix);
// dev URL becomes http://localhost:5173/admin/.
export default defineConfig({
  base: '/admin/',
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
    }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      // The dev-host topology has no nginx container, so these must point at the
      // gateway on :8080 directly. Overridable for anyone still running the
      // fully-containerised `make dev` (set DASHBOARD_PROXY_TARGET=http://localhost:80).
      //
      // The gateway reads API_KEY from its own env, so it does not need the
      // edge's X-API-KEY injection. ws: true keeps /api/v1/ws upgrades working;
      // the gateway forwards them to WS_GATEWAY_URL (localhost:8085 on host).
      '/api': {
        target: PROXY_TARGET,
        changeOrigin: true,
        ws: true,
      },
      '/uploads': {
        target: PROXY_TARGET,
        changeOrigin: true,
      },
      // Images are served by MinIO through the edge in prod. dev-host has no
      // edge, so stop here and let the browser hit MinIO's published port —
      // set up by the S3_PUBLIC_BASE_URL override below.
      ...(PROXY_TARGET === 'http://localhost:8080'
        ? {}
        : {
            '/goshop-media': {
              target: PROXY_TARGET,
              changeOrigin: true,
            },
          }),
    },
  },
  test: {
    silent: 'passed-only',
    unstubEnvs: true,
    browser: {
      enabled: true,
      provider: playwright(),
      instances: [{ browser: 'chromium' }],
    },
    coverage: {
      // include: ['src/**/*.{js,jsx,ts,tsx}'], // Uncomment to expand the report to all src/**/* so untested modules appear as 0% coverage.
      exclude: [
        'src/components/ui/**',
        'src/assets/**',
        'src/tanstack-table.d.ts',
        'src/routeTree.gen.ts',
        'src/test-utils/**',
        'src/routes/**',
      ],
    },
  },
})
