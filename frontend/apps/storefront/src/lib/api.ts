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
  // Refresh token lives in an HttpOnly cookie and authFetch owns the refresh,
  // so the client must not retry on its own. A 401 without this check is
  // answered by the middleware's executeRefresh, whose bare /auth/refresh call
  // carries no X-Client-App and reads the legacy shared cookie (usually absent,
  // because refresh cookies are scoped to /api/v1/auth). The second retry then
  // fires with an empty bearer, which is the 401 seen right after the refresh.
  onUnauthorized: () => {
    useAuthStore.getState().reset()
  },
})
// NOTE: no getRefreshToken / onTokenRefreshed hooks on purpose — they would
// re-enable the middleware retry. Ponytail: drop this comment if the middleware
// ever learns X-Client-App and a max one-refresh guard.

export type ApiInstance = typeof api

let refreshPromise: Promise<string | null> | null = null

function needsAuth(init?: RequestInit): boolean {
  // GET/HEAD are public for most storefront endpoints; the 401 retry below
  // still recovers those, so only restore eagerly for mutating calls.
  const method = (init?.method ?? 'GET').toUpperCase()
  return method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS'
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshPromise) return refreshPromise
  refreshPromise = (async () => {
    try {
      const apiKey = API_KEY
      const res = await fetch(`${API_BASE_URL}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-KEY': apiKey,
          'X-Client-App': 'storefront',
        },
        // The refresh token is an HttpOnly cookie; the body stays empty.
        credentials: 'include',
        body: JSON.stringify({}),
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

      useAuthStore.getState().setTokens(tokenData.access_token)
      return tokenData.access_token
    } catch {
      useAuthStore.getState().reset()
      return null
    } finally {
      refreshPromise = null
    }
  })()
  return refreshPromise
}

export async function restoreSession(): Promise<string | null> {
  const token = useAuthStore.getState().accessToken
  if (token) return token
  return refreshAccessToken()
}

export async function authFetch(
  input: string | URL | Request,
  init?: RequestInit
): Promise<Response> {
  const apiKey = API_KEY
  let token = useAuthStore.getState().accessToken

  // Never let the API client answer a 401 by retrying with an empty token: it
  // calls /auth/refresh without X-Client-App, so the backend would rotate the
  // anonymous `refresh_token` cookie instead of this app's. authFetch owns the
  // retry below.
  if (!token && needsAuth(init)) {
    token = (await refreshAccessToken()) ?? ''
  }

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
    // Keeps the storefront's refresh cookie separate from the dashboard's.
    if (!headers.has('X-Client-App')) {
      headers.set('X-Client-App', 'storefront')
    }
    return headers
  }

  let res = await fetch(targetUrl, {
    ...init,
    credentials: 'include',
    headers: buildHeaders(token),
  })

  // A 401 from /auth/* is a real credential failure, not an expired access
  // token; refreshing on it would turn a bad password into a session reset.
  const targetHref = targetUrl instanceof Request ? targetUrl.url : String(targetUrl)
  if (res.status === 401 && !targetHref.includes('/auth/')) {
    const newToken = await refreshAccessToken()
    res = await fetch(targetUrl, {
      ...init,
      credentials: 'include',
      headers: buildHeaders(newToken ?? token),
    })
  }

  return res
}

