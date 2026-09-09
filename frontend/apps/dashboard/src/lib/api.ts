import { createApiClient } from '@goshop/api-client'
import { useAuthStore } from '@/stores/auth-store'

export const api = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080',
  getToken: () => useAuthStore.getState().auth.accessToken,
  getRefreshToken: () => useAuthStore.getState().auth.refreshToken,
  getApiKey: () => import.meta.env.VITE_API_KEY || 'abcd-1234-1111',
  onTokenRefreshed: (tokens) => {
    useAuthStore.getState().auth.setTokens(tokens.accessToken, tokens.refreshToken)
  },
  onUnauthorized: () => {
    useAuthStore.getState().auth.reset()
  },
})

export type ApiInstance = typeof api
