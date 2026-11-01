import { createFileRoute, redirect } from '@tanstack/react-router'
import { AuthenticatedLayout } from '@/components/layout/authenticated-layout'
import { isStaffOrAdmin } from '@/lib/auth-utils'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated')({
  beforeLoad: ({ location }) => {
    const { accessToken, user } = useAuthStore.getState().auth
    if (!accessToken) {
      // Store a path, not a full URL: sign-in's navigate() only accepts paths.
      const redirect =
        location.pathname + (location.searchStr ? `?${location.searchStr}` : '')
      throw redirect({
        to: '/sign-in',
        search: {
          redirect,
        },
      })
    }

    // Protect all dashboard authenticated routes against Member/Customer roles
    if (user && !isStaffOrAdmin(user.level)) {
      throw redirect({
        to: '/403',
      })
    }
  },
  component: AuthenticatedLayout,
})
