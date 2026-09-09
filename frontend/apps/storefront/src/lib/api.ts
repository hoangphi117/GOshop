import { createApiClient } from '@goshop/api-client'
import { useAuthStore } from '@/stores/auth-store'

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

export const api = createApiClient({
  baseUrl: API_BASE_URL,
  getToken: () => useAuthStore.getState().accessToken,
  getRefreshToken: () => useAuthStore.getState().refreshToken,
  getApiKey: () => process.env.NEXT_PUBLIC_API_KEY || 'abcd-1234-1111',
  onTokenRefreshed: (tokens) => {
    useAuthStore.getState().setTokens(tokens.accessToken, tokens.refreshToken)
  },
  onUnauthorized: () => {
    useAuthStore.getState().reset()
  },
})

export type ApiInstance = typeof api
