import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
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

      // Fetch user profile immediately
      try {
        const { data: profileRes } = await api.GET('/users/me')
        if (profileRes?.data) {
          const u = profileRes.data
          auth.setUser({
            uuid: u.uuid,
            email: u.email || email,
            fullName: u.full_name,
            level: u.level,
            status: u.status,
          })
        }
      } catch {
        // Fallback user if me endpoint not yet ready
        auth.setUser({
          email,
          fullName: email.split('@')[0],
        })
      }

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
