import type { Middleware, ClientOptions } from 'openapi-fetch'

export interface AuthTokens {
  accessToken: string
  refreshToken?: string
  expiresIn?: number
}

export interface AuthMiddlewareOptions {
  baseUrl?: string
  getToken?: () => string | null | undefined
  getRefreshToken?: () => string | null | undefined
  getApiKey?: () => string | null | undefined
  onTokenRefreshed?: (tokens: AuthTokens) => void
  onUnauthorized?: () => void
  refreshEndpoint?: string
  fetch?: ClientOptions['fetch']
}

export function createAuthMiddleware(options: AuthMiddlewareOptions = {}): Middleware {
  const cleanBase = (options.baseUrl || '').replace(/\/+$/, '')
  const hasV1InBase = cleanBase.endsWith('/api/v1')
  const defaultRefresh = hasV1InBase ? '/auth/refresh' : '/api/v1/auth/refresh'
  const refreshEndpoint = options.refreshEndpoint || defaultRefresh
  // Terminal 401s: the refresh call can succeed while the retried request
  // still comes back 401 (wrong scope, wrong app's cookie, deleted user). Only
  // callers that set onUnauthorized/listen for the event would stop, so
  // track idempotency here and never refresh a request a second time.
  const refreshedIds = new Set<string>()
  const requestClones = new Map<string, Request>()
  let refreshPromise: Promise<AuthTokens | null> | null = null

  const isAuthEndpoint = (url: string, schemaPath?: string) => {
    return (
      url.includes(refreshEndpoint) ||
      url.includes('/auth/login') ||
      url.includes('/auth/logout') ||
      url.includes('/auth/register') ||
      (schemaPath !== undefined &&
        (schemaPath.includes('/auth/refresh') ||
          schemaPath.includes('/auth/login') ||
          schemaPath.includes('/auth/logout') ||
          schemaPath.includes('/auth/register')))
    )
  }

  const executeRefresh = async (): Promise<AuthTokens | null> => {
    const refreshToken = options.getRefreshToken?.()

    try {
      const fetchFn = options.fetch || globalThis.fetch
      const baseUrl = options.baseUrl ? options.baseUrl.replace(/\/+$/, '') : ''
      const url = `${baseUrl}${refreshEndpoint}`

      const headers = new Headers({
        'Content-Type': 'application/json',
      })
      const apiKey = options.getApiKey?.()
      if (apiKey) {
        headers.set('X-API-KEY', apiKey)
      }

      const refreshRequest = new Request(url, {
        method: 'POST',
        headers,
        credentials: 'include',
        body: JSON.stringify(refreshToken ? { refresh_token: refreshToken } : {}),
      })

      const res = await fetchFn(refreshRequest)

      if (!res.ok) {
        return null
      }

      const json = await res.json()
      const tokenData = json?.data
      if (!tokenData?.access_token) {
        return null
      }

      const newTokens: AuthTokens = {
        accessToken: tokenData.access_token,
        refreshToken: tokenData.refresh_token,
        expiresIn: tokenData.expires_in,
      }

      options.onTokenRefreshed?.(newTokens)
      return newTokens
    } catch {
      return null
    }
  }

  return {
    async onRequest({ request, schemaPath, id }) {
      const apiKey = options.getApiKey?.()
      if (apiKey) {
        request.headers.set('X-API-KEY', apiKey)
      }

      // Auth endpoints carry credentials in the body; attaching a stale
      // Authorization from a previous session is never wanted. Also skip the
      // clone for them (their 401 must not trigger refresh, see onResponse).
      if (!isAuthEndpoint(request.url, schemaPath)) {
        const token = options.getToken?.()
        if (token) {
          request.headers.set('Authorization', `Bearer ${token}`)
        }
        try {
          requestClones.set(id, request.clone())
        } catch {
          // If request body stream cannot be cloned, proceed without clone
        }
      }

      return request
    },

    async onResponse({ request, response, schemaPath, id }) {
      const requestClone = requestClones.get(id)
      requestClones.delete(id)

      if (response.status === 401) {
        // If 401 is from an auth endpoint, do not attempt refresh
        if (isAuthEndpoint(request.url, schemaPath)) {
          if (options.onUnauthorized) {
            options.onUnauthorized()
          } else if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('goshop:unauthorized'))
          }
          return response
        }

        // Retried once already and still unauthorized: refreshing again would
        // spin (refresh 200 -> retry 401 -> refresh ...). Give up instead.
        if (refreshedIds.has(id)) {
          refreshedIds.delete(id)
          if (options.onUnauthorized) {
            options.onUnauthorized()
          } else if (typeof window !== 'undefined') {
            window.dispatchEvent(new CustomEvent('goshop:unauthorized'))
          }
          return response
        }

        // Share the refresh promise among concurrent 401s
        if (!refreshPromise) {
          refreshPromise = executeRefresh().finally(() => {
            refreshPromise = null
          })
        }

        const newTokens = await refreshPromise

        if (newTokens?.accessToken) {
          // Bounded: the retried request gets at most one more 401 check.
          refreshedIds.add(id)
          setTimeout(() => refreshedIds.delete(id), 30_000)
          // Retry original request with the new access token
          try {
            const baseReq = requestClone || request
            const retryHeaders = new Headers(baseReq.headers)
            retryHeaders.set('Authorization', `Bearer ${newTokens.accessToken}`)

            // Rebuild with a string body: passing a body-carrying Request as
            // init turns the body into a ReadableStream, which some Chrome
            // builds refuse with a bare "Failed to fetch".
            const body = baseReq.body ? await baseReq.clone().text() : undefined
            const retryRequest = new Request(baseReq.url, {
              method: baseReq.method,
              headers: retryHeaders,
              body,
              credentials: 'include',
            })

            const fetchFn = options.fetch || globalThis.fetch
            const retryResponse = await fetchFn(retryRequest)
            return retryResponse
          } catch {
            // If retry throws, fall through to onUnauthorized
          }
        }

        // If refresh failed, clear session
        if (options.onUnauthorized) {
          options.onUnauthorized()
        } else if (typeof window !== 'undefined') {
          window.dispatchEvent(new CustomEvent('goshop:unauthorized'))
        }
      }

      return response
    },
  }
}
