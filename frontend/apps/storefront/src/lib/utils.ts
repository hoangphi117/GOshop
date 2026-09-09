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
