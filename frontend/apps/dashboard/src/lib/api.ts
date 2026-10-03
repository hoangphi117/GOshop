import { createApiClient } from '@goshop/api-client'
import { useAuthStore } from '@/stores/auth-store'

export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080').replace(/\/+$/, '')
export const API_KEY = import.meta.env.VITE_API_KEY || ''

export const api = createApiClient({
  baseUrl: API_BASE_URL,
  getToken: () => useAuthStore.getState().auth.accessToken,
  getRefreshToken: () => useAuthStore.getState().auth.refreshToken,
  getApiKey: () => API_KEY,
  onTokenRefreshed: (tokens) => {
    useAuthStore.getState().auth.setTokens(tokens.accessToken, tokens.refreshToken)
  },
  onUnauthorized: () => {
    useAuthStore.getState().auth.reset()
  },
})

export type ApiInstance = typeof api

export async function fetchWithAuth(path: string, options: RequestInit = {}): Promise<Response> {
  const baseUrl = API_BASE_URL
  const apiKey = API_KEY
  let token = useAuthStore.getState().auth.accessToken

  const getHeaders = (currToken: string) => {
    const headers = new Headers(options.headers)
    if (currToken) {
      headers.set('Authorization', `Bearer ${currToken}`)
    }
    if (apiKey) {
      headers.set('X-API-KEY', apiKey)
    }
    return headers
  }

  const url = path.startsWith('http') ? path : `${baseUrl}${path.startsWith('/') ? '' : '/'}${path}`

  let res = await fetch(url, {
    ...options,
    headers: getHeaders(token),
  })

  // If 401 Unauthorized (token expired), attempt silent refresh and retry
  if (res.status === 401) {
    const refreshToken = useAuthStore.getState().auth.refreshToken
    if (refreshToken) {
      try {
        const refreshRes = await fetch(`${baseUrl}/api/v1/auth/refresh`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-API-KEY': apiKey,
          },
          body: JSON.stringify({ refresh_token: refreshToken }),
        })

        if (refreshRes.ok) {
          const refreshJson = await refreshRes.json()
          const tokenData = refreshJson?.data
          if (tokenData?.access_token) {
            useAuthStore.getState().auth.setTokens(tokenData.access_token, tokenData.refresh_token)
            token = tokenData.access_token
            res = await fetch(url, {
              ...options,
              headers: getHeaders(token),
            })
          }
        } else {
          useAuthStore.getState().auth.reset()
        }
      } catch {
        useAuthStore.getState().auth.reset()
      }
    }
  }

  return res
}
