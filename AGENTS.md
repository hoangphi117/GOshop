# GOshop Development Conventions

## Language & Localization
- **UI & Text Convention:** All user interface text, page titles, navigation labels, form labels, placeholders, validation error messages, alerts, and notification toasts must be written in **English**.
- Do not hardcode Vietnamese strings in UI components, notifications, or toasts unless specifically requested by the user.

## Architecture Guidelines
- **Monorepo Structure:** Turborepo + pnpm workspaces.
  - `frontend/packages/api-client`: OpenAPI type-safe client generator (`@goshop/api-client`).
  - `frontend/apps/dashboard`: Admin Dashboard (Vite + React 19 + TanStack Router + TanStack Query + Tailwind CSS).
  - `frontend/apps/storefront`: Storefront (Next.js App Router).
- **TypeScript:** Strict type checking. No `any` casting. Form schemas must match input and output types for seamless React Hook Form integration.
- **Authentication:** JWT Access Token + Refresh Token with automatic silent refresh & retry mechanism in `@goshop/api-client`.
