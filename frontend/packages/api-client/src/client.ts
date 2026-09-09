import createClient, { type ClientOptions } from 'openapi-fetch'
import type { paths } from './generated/schema'
import { createAuthMiddleware, type AuthMiddlewareOptions } from './middleware'

export interface ApiClientOptions extends AuthMiddlewareOptions {
  baseUrl: string
  fetch?: ClientOptions['fetch']
}

export function createApiClient(options: ApiClientOptions) {
  const { baseUrl, fetch, ...authOptions } = options
  const client = createClient<paths>({
    baseUrl,
    fetch,
  })

  client.use(
    createAuthMiddleware({
      baseUrl,
      fetch,
      ...authOptions,
    })
  )

  return client
}

export type ApiClient = ReturnType<typeof createApiClient>
