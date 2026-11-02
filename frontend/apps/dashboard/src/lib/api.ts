import { createApiClient } from '@goshop/api-client'
import { useAuthStore } from '@/stores/auth-store'

// Paths in the generated schema are inconsistent: some carry the /api/v1
// prefix, most (e.g. /users/me, /admin/banners) do not. Sending bare paths to
// the origin 404s, so the client base always ends in /api/v1 (openapi-fetch
// joins baseUrl + path). VITE_API_BASE_URL may include the prefix already.
// Default: same origin (relative URLs) — the edge nginx serves the SPA and
// proxies /api, so no host is baked in and CORS never applies. Set
// VITE_API_BASE_URL only when the API lives on a different origin.
export const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '')
  .replace(/\/+$/, '')
  .replace(/\/api\/v1$/, '')
export const API_VERSIONED_BASE = `${API_BASE_URL}/api/v1`
export const API_KEY = import.meta.env.VITE_API_KEY || ''

// openapi-fetch hands us a Request; dedupe the doubled prefix for schema
// paths that already include /api/v1. NOTE: `new Request(url, requestWithBody)`
// turns the body into a ReadableStream (chunked upload) which Chrome refuses
// with a bare "Failed to fetch" on this stack, so body-carrying requests are
// rebuilt with the string body they were created with. Bodyless requests
// (GET/DELETE) can safely use Request-as-init.
const apiFetch: NonNullable<Parameters<typeof createApiClient>[0]['fetch']> = async (
  input: Request,
) => {
  const url = input.url.replace('/api/v1/api/v1/', '/api/v1/')
  if (url === input.url) return fetch(input)
  if (!input.body) return fetch(new Request(url, input))
  const body = await input.clone().text()
  return fetch(url, { method: input.method, headers: input.headers, body })
}

export const api = createApiClient({
  baseUrl: API_VERSIONED_BASE,
  fetch: apiFetch,
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

// Single-flight silent refresh for route guards: when the access token is
// gone but a refresh token survives, try to restore the session before
// sending the user to sign-in. Returns true when a usable access token exists.
let refreshInflight: Promise<boolean> | null = null

export function tryRefreshSession(): Promise<boolean> {
  if (refreshInflight) return refreshInflight
  refreshInflight = (async () => {
    const { auth } = useAuthStore.getState()
    if (auth.accessToken) return true
    if (!auth.refreshToken) return false
    try {
      const res = await fetch(`${API_VERSIONED_BASE}/auth/refresh`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(API_KEY ? { 'X-API-KEY': API_KEY } : {}),
        },
        body: JSON.stringify({ refresh_token: auth.refreshToken }),
      })
      if (!res.ok) {
        auth.reset()
        return false
      }
      const json = await res.json()
      const tokenData = json?.data
      if (!tokenData?.access_token) {
        auth.reset()
        return false
      }
      auth.setTokens(tokenData.access_token, tokenData.refresh_token)
      return true
    } catch {
      auth.reset()
      return false
    } finally {
      refreshInflight = null
    }
  })()
  return refreshInflight
}

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
