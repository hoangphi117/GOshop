/// <reference types="vitest/config" />
import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import { playwright } from '@vitest/browser-playwright'

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
      // Route dev traffic through the nginx edge (:80), not the gateway (:8080).
      //
      // The gateway rejects key-less requests with 400 "missing API Key" on the
      // authenticated routes (/users/me, /admin/*), and nginx is the only layer
      // that injects X-API-KEY server-side (map $http_x_api_key in
      // nginx/templates/00-api-key.conf.template). Proxying straight to :8080
      // skipped that injection, so every authenticated screen failed to load.
      // Going through :80 also reuses the edge's CORS + preflight handling, so
      // dev and prod take the same path.
      //
      // ws: true keeps the /api/v1/ws upgrade working (nginx forwards it to
      // ws-gateway:8085).
      '/api': {
        target: 'http://localhost:80',
        changeOrigin: true,
        ws: true,
      },
      '/uploads': {
        target: 'http://localhost:80',
        changeOrigin: true,
      },
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
