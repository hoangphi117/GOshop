import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { isStaffOrAdmin } from '@/lib/auth-utils'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

export function useAuth() {
  const queryClient = useQueryClient()
  const { auth } = useAuthStore()

  const userQuery = useQuery({
    queryKey: ['currentUser', auth.accessToken],
    queryFn: async () => {
      if (!auth.accessToken) return null
      const { data, error } = await api.GET('/users/me')
      if (error || !data?.data) {
        throw new Error(error?.error || 'Failed to fetch user profile')
      }
      const u = data.data
      if (!isStaffOrAdmin(u.level)) {
        auth.reset()
        throw new Error('Access denied. Administrator privileges required.')
      }
      const userProfile: AuthUser = {
        uuid: u.uuid,
        email: u.email || '',
        fullName: u.full_name,
        level: u.level,
        status: u.status,
      }
      auth.setUser(userProfile)
      return userProfile
    },
    enabled: Boolean(auth.accessToken),
    staleTime: 5 * 60 * 1000,
  })

  const loginMutation = useMutation({
    mutationFn: async ({ email, password }: { email: string; password: string }) => {
      const { data, error } = await api.POST('/api/v1/auth/login', {
        body: { email, password },
      })

      if (error || !data?.data) {
        const errorMsg =
          (error as { detail?: string; error?: string })?.detail ||
          (error as { detail?: string; error?: string })?.error ||
          'Sign in failed. Please check your credentials and try again.'
        throw new Error(errorMsg)
      }

      const loginData = data.data
      auth.setTokens(loginData.access_token || '', loginData.refresh_token || '')

      // Fetch user profile immediately & verify role permissions
      const { data: profileRes, error: profileErr } = await api.GET('/users/me')
      if (profileErr || !profileRes?.data) {
        auth.reset()
        throw new Error('Failed to retrieve user profile. Access denied.')
      }

      const u = profileRes.data
      if (!isStaffOrAdmin(u.level)) {
        auth.reset()
        throw new Error('Access denied. This dashboard is restricted to administrators and staff members.')
      }

      const userProfile: AuthUser = {
        uuid: u.uuid,
        email: u.email || email,
        fullName: u.full_name,
        level: u.level,
        status: u.status,
      }
      auth.setUser(userProfile)

      queryClient.invalidateQueries({ queryKey: ['currentUser'] })
      return loginData
    },
  })

  const logoutMutation = useMutation({
    mutationFn: async () => {
      try {
        if (auth.refreshToken) {
          await api.POST('/api/v1/auth/logout', {
            body: { refresh_token: auth.refreshToken },
          })
        }
      } catch {
        // Ignore logout error on server
      } finally {
        auth.reset()
        queryClient.clear()
      }
    },
  })

  // Sign-in page itself: if a redirect target carries a nested sign-in value,
  // drop it so re-mounts never re-wrap ?redirect=/sign-in?redirect=... .
  // The API middleware already attempts a silent refresh on 401 before
  // calling onUnauthorized, so reaching the guards with no token means the
  // refresh token is gone or invalid and sign-in is the correct destination.
  return {
    user: auth.user,
    isAuthenticated: Boolean(auth.accessToken),
    login: loginMutation.mutateAsync,
    isLoggingIn: loginMutation.isPending,
    loginError: loginMutation.error,
    logout: logoutMutation.mutateAsync,
    isLoggingOut: logoutMutation.isPending,
    userQuery,
  }
}
