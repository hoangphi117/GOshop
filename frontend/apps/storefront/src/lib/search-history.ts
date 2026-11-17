import { API_BASE_URL, API_KEY } from './api'

export interface ApiEnvelope<T> {
  status: string
  message: string
  data?: T
}

export const ANON_COOKIE_NAME = 'goshop_anon_id'

function headers(): HeadersInit {
  return {
    'Content-Type': 'application/json',
    ...(API_KEY ? { 'X-API-KEY': API_KEY } : {}),
    'X-Client-App': 'storefront',
  }
}

/**
 * Server-side identity read. The anon cookie lives in the request the
 * storefront is rendering, so a Server Component can forward it to the
 * analytics service and render history on the first paint.
 */
export async function fetchSearchHistory(
  cookieHeader: string | undefined,
  limit = 10
): Promise<string[]> {
  if (!API_BASE_URL) return []
  try {
    const res = await fetch(
      `${API_BASE_URL}/api/v1/search/history?limit=${limit}`,
      {
        headers: { ...headers(), ...(cookieHeader ? { Cookie: cookieHeader } : {}) },
        cache: 'no-store',
      }
    )
    if (!res.ok) return []
    const json = (await res.json()) as ApiEnvelope<{ query: string }[]>
    return (json.data ?? []).map((entry) => entry.query).filter(Boolean)
  } catch {
    return []
  }
}
