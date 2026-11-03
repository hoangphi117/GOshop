import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'
import { tryRefreshSession } from '@/lib/api'
import { isStaffOrAdmin } from '@/lib/auth-utils'
import { sanitizeRedirectTarget } from '@/lib/auth-redirect'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: async ({ location, search }) => {
    const store = useAuthStore.getState().auth
    // Always attempt the refresh path: isTokenFresh inside tryRefreshSession
    // returns fast when the token is usable, and restores soft-expired
    // sessions (access token dead, refresh token alive) before route render,
    // so an expired access token no longer bounces to sign-in.
    const hasToken = await tryRefreshSession()
    if (!hasToken) {
      // Store a path, not a full URL: sign-in's navigate() only accepts paths.
      // Not named `redirect`: that would shadow the router helper below.
      // Never nest: if already on sign-in, drop the target instead of wrapping
      // the current ?redirect=... URL again (caused the exponential URL bug).
      const existing = (search as { redirect?: string })?.redirect
      const sanitized = sanitizeRedirectTarget(existing)
      const currentPath = location.pathname + (location.searchStr ? `?${location.searchStr}` : '')
      const target = sanitized ?? (currentPath.includes('/sign-in') ? undefined : currentPath)
      throw redirect({
        to: '/sign-in',
        search: target ? { redirect: target } : undefined,
      })
    }

    // Protect all dashboard authenticated routes against Member/Customer roles
    if (store.user && !isStaffOrAdmin(store.user.level)) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: AuthenticatedLayout,
})
