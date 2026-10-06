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
  const baseUrl = (
    process.env.NEXT_PUBLIC_API_URL ||
    'http://localhost:8080'
  ).replace(/\/$/, '')
  const cleanPath = url.startsWith('/') ? url : `/${url}`
  return `${baseUrl}${cleanPath}`
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
