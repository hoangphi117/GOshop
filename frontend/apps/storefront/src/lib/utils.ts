import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function getImageUrl(url?: string | null): string {
  if (!url) return ''
  if (
    url.startsWith('http://') ||
    url.startsWith('https://') ||
    url.startsWith('data:') ||
    url.startsWith('blob:')
  ) {
    return url
  }
  const apiBase = (
    process.env.NEXT_PUBLIC_API_URL ||
    'http://localhost:8080'
  ).replace(/\/$/, '')
  // Product image_url is already an absolute CDN URL for most of the catalog.
  // Media the app uploaded itself is edge-relative (/goshop-media/...) on
  // purpose, so it survives a domain change; it resolves against the API
  // origin, which serves the bucket in prod. In dev-host the API is the gateway
  // on :8080, so NEXT_PUBLIC_MEDIA_URL points those at MinIO instead of 404ing.
  if (url.startsWith('/goshop-media/')) {
    const mediaBase = (process.env.NEXT_PUBLIC_MEDIA_URL || apiBase).replace(/\/$/, '')
    return `${mediaBase}${url}`
  }
  // Only prefix the API host for a genuine relative path, otherwise a bad base
  // would rewrite every image to a broken api-host URL.
  const cleanPath = url.startsWith('/') ? url : `/${url}`
  return `${apiBase}${cleanPath}`
}

/** True when the client wants a JSON response (Next's client router does). */
export function prefersJson(headers: { get(name: string): string | null }): boolean {
  const rsc = headers.get('rsc')
  if (rsc === '1') return true
  return (headers.get('accept') || '').includes('application/json')
}

export function extractErrorMessage(errData: unknown, fallback = 'An error occurred'): string {
  if (!errData) return fallback
  if (typeof errData === 'string') return errData
  if (typeof errData === 'object' && errData !== null) {
    const obj = errData as Record<string, unknown>
    if (typeof obj.message === 'string' && obj.message.trim()) return obj.message
    if (typeof obj.error === 'string' && obj.error.trim()) return obj.error
    if (typeof obj.detail === 'string' && obj.detail.trim()) return obj.detail
    if (typeof obj.error === 'object' && obj.error !== null) {
      const fieldErrors = Object.values(obj.error as Record<string, unknown>).filter(
        (v): v is string => typeof v === 'string' && v.trim().length > 0
      )
      if (fieldErrors.length > 0) {
        return fieldErrors.join('. ')
      }
    }
  }
  return fallback
}
