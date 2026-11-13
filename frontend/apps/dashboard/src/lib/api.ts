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

// Both apps share one origin, so the refresh cookie name must be per-client or
// logging into one app silently adopts the other app's session.
export const CLIENT_HEADERS: Record<string, string> = { 'X-Client-App': 'dashboard' }

// openapi-fetch hands us a Request; dedupe the doubled prefix for schema
// paths that already include /api/v1. NOTE: `new Request(url, requestWithBody)`
// turns the body into a ReadableStream (chunked upload) which Chrome refuses
// with a bare "Failed to fetch" on this stack, so body-carrying requests are
// rebuilt with the string body they were created with. Bodyless requests
// (GET/DELETE) can safely use Request-as-init.
// `fetch` is also where every API call starts, so a missing access token is
// restored here: the client never retries a 401 on its own (see `api` below).
const apiFetch: NonNullable<Parameters<typeof createApiClient>[0]['fetch']> = async (
  input: Request,
) => {
  const url = input.url.replace('/api/v1/api/v1/', '/api/v1/')
  if (!useAuthStore.getState().auth.accessToken && !url.includes('/auth/')) {
    await tryRefreshSession()
  }
  // The refresh token is an HttpOnly cookie, so every API call must carry
  // cookies: without them a silent refresh after reload has nothing to send.
  if (url === input.url) {
    input.headers.set('X-Client-App', 'dashboard')
    return fetch(new Request(input, { credentials: 'include' }))
  }
  if (!input.body) {
    input.headers.set('X-Client-App', 'dashboard')
    return fetch(new Request(url, input), { credentials: 'include' })
  }
  const body = await input.clone().text()
  const headers = new Headers(input.headers)
  headers.set('X-Client-App', 'dashboard')
  return fetch(url, {
    method: input.method,
    headers,
    body,
    credentials: 'include',
  })
}

// openapi-fetch reports every 401 as an `error` result, so no caller ever
// reacts to an expired token. Through this Proxy the first 401 attempt is
// refreshed explicitly (with this app's cookie) and the call is retried once;
// if it still fails the error result is handed back unchanged.
const rawApi = createApiClient({
  baseUrl: API_VERSIONED_BASE,
  fetch: apiFetch,
  getToken: () => useAuthStore.getState().auth.accessToken,
  getApiKey: () => API_KEY,
  onUnauthorized: () => {
    useAuthStore.getState().auth.resetAccessToken()
  },
})

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'] as const

function isUnauthorized(result: unknown): boolean {
  return (
    typeof result === 'object' &&
    result !== null &&
    'response' in result &&
    (result as { response?: Response }).response?.status === 401
  )
}

export const api = new Proxy(rawApi, {
  get(target, prop, receiver) {
    const value = Reflect.get(target, prop, receiver)
    if (typeof value !== 'function' || !METHODS.includes(prop as (typeof METHODS)[number])) {
      return value
    }
    return async (...args: unknown[]): Promise<unknown> => {
      const call = value as (...a: unknown[]) => Promise<unknown>
      const result = await call(...args)
      if (!isUnauthorized(result) || !(await tryRefreshSession())) {
        return result
      }
      return call(...args)
    }
  },
}) as typeof rawApi

export type Api = typeof api

// Single-flight silent refresh for route guards: when the access token is
// gone or expired but a refresh token survives, try to restore the session
// before sending the user to sign-in. Returns true when a usable access token
// exists. Access tokens live ~15m; treat tokens expiring within 60s as gone
// so navigation triggers a refresh instead of bouncing to sign-in.
const REFRESH_SKEW_MS = 60 * 1000
let refreshInflight: Promise<boolean> | null = null

function isTokenFresh(token: string): boolean {
  if (!token) return false
  const parts = token.split('.')
  if (parts.length !== 3) return true
  try {
    const payload = JSON.parse(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')))
    if (typeof payload.exp !== 'number') return true
    return payload.exp * 1000 - Date.now() > REFRESH_SKEW_MS
  } catch {
    return true
  }
}

export function tryRefreshSession(): Promise<boolean> {
  if (refreshInflight) return refreshInflight
  refreshInflight = (async () => {
    const { auth } = useAuthStore.getState()
    if (isTokenFresh(auth.accessToken)) return true
    // The refresh token is an HttpOnly cookie, so no pre-check on it: just try.
    try {
      const res = await fetch(`${API_VERSIONED_BASE}/auth/refresh`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(API_KEY ? { 'X-API-KEY': API_KEY } : {}),
          'X-Client-App': 'dashboard',
        },
        credentials: 'include',
        body: JSON.stringify({}),
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
      auth.setTokens(tokenData.access_token)
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
  // Restore a missing token before the first attempt; this path is used for
  // uploads and invoices, where a wasted 401 round trip is the slow part.
  if (!useAuthStore.getState().auth.accessToken) {
    await tryRefreshSession()
  }
  let token = useAuthStore.getState().auth.accessToken

  const getHeaders = (currToken: string) => {
    const headers = new Headers(options.headers)
    if (currToken) {
      headers.set('Authorization', `Bearer ${currToken}`)
    }
    if (apiKey) {
      headers.set('X-API-KEY', apiKey)
    }
    // Keeps the dashboard's refresh cookie separate from the storefront's.
    if (!headers.has('X-Client-App')) {
      headers.set('X-Client-App', 'dashboard')
    }
    return headers
  }

  const url = path.startsWith('http') ? path : `${baseUrl}${path.startsWith('/') ? '' : '/'}${path}`

  let res = await fetch(url, {
    ...options,
    headers: getHeaders(token),
  })

  // If 401 Unauthorized (token expired), refresh once and retry. Reuses the
  // same single-flight refresh the route guard uses, which sends this app's
  // X-Client-App so the right refresh cookie is rotated. /auth/* 401s are
  // credential failures, not expiry, and must not reset the session.
  if (res.status === 401 && !path.includes('/auth/')) {
    const refreshed = await tryRefreshSession()
    token = useAuthStore.getState().auth.accessToken
    if (refreshed && token) {
      res = await fetch(url, {
        ...options,
        headers: getHeaders(token),
      })
    }
  }

  return res
}
