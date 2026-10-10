import { createApiClient } from '@goshop/api-client'
import { useAuthStore } from '@/stores/auth-store'

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

export const API_KEY = process.env.NEXT_PUBLIC_API_KEY || ''

export const MEILISEARCH_URL =
  process.env.NEXT_PUBLIC_MEILISEARCH_URL || ''

export const MEILISEARCH_SEARCH_KEY =
  process.env.NEXT_PUBLIC_MEILISEARCH_SEARCH_KEY || ''

export const api = createApiClient({
  baseUrl: API_BASE_URL,
  getToken: () => useAuthStore.getState().accessToken,
  getRefreshToken: () => useAuthStore.getState().refreshToken,
  getApiKey: () => API_KEY,
  onTokenRefreshed: (tokens) => {
    useAuthStore.getState().setTokens(tokens.accessToken, tokens.refreshToken)
  },
  onUnauthorized: () => {
    useAuthStore.getState().reset()
  },
})

export type ApiInstance = typeof api

let refreshPromise: Promise<string | null> | null = null

async function refreshAccessToken(): Promise<string | null> {
  const refreshToken = useAuthStore.getState().refreshToken

  try {
    const apiKey = API_KEY
    const res = await fetch(`${API_BASE_URL}/api/v1/auth/refresh`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-KEY': apiKey,
      },
      credentials: 'include',
      body: JSON.stringify(refreshToken ? { refresh_token: refreshToken } : {}),
    })

    if (!res.ok) {
      useAuthStore.getState().reset()
      return null
    }

    const json = await res.json()
    const tokenData = json?.data
    if (!tokenData?.access_token) {
      useAuthStore.getState().reset()
      return null
    }

    useAuthStore.getState().setTokens(tokenData.access_token, tokenData.refresh_token)
    return tokenData.access_token
  } catch {
    useAuthStore.getState().reset()
    return null
  }
}

export async function authFetch(
  input: string | URL | Request,
  init?: RequestInit
): Promise<Response> {
  const apiKey = API_KEY
  const token = useAuthStore.getState().accessToken

  let targetUrl = input
  if (typeof input === 'string' && input.startsWith('/')) {
    targetUrl = `${API_BASE_URL}${input}`
  }

  const buildHeaders = (authToken?: string) => {
    const headers = new Headers(init?.headers)
    if (authToken) {
      headers.set('Authorization', `Bearer ${authToken}`)
    }
    if (!headers.has('X-API-KEY') && !headers.has('x-api-key')) {
      headers.set('X-API-KEY', apiKey)
    }
    return headers
  }

  let res = await fetch(targetUrl, {
    ...init,
    credentials: 'include',
    headers: buildHeaders(token),
  })

  if (res.status === 401) {
    if (!refreshPromise) {
      refreshPromise = refreshAccessToken().finally(() => {
        refreshPromise = null
      })
    }

    const newToken = await refreshPromise
    if (newToken) {
      res = await fetch(targetUrl, {
        ...init,
        credentials: 'include',
        headers: buildHeaders(newToken),
      })
    }
  }

  return res
}

