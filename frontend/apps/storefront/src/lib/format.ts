export function formatCurrency(amount: number | string | null | undefined): string {
  if (amount === null || amount === undefined) return '0 ₫'
  const numeric = typeof amount === 'string' ? parseFloat(amount) : amount
  if (isNaN(numeric)) return '0 ₫'

  return new Intl.NumberFormat('vi-VN', {
    style: 'currency',
    currency: 'VND',
    maximumFractionDigits: 0,
  }).format(numeric)
}

export function formatDate(dateString: string | null | undefined): string {
  if (!dateString) return ''
  try {
    const d = new Date(dateString)
    return new Intl.DateTimeFormat('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    }).format(d)
  } catch {
    return dateString
  }
}

export function formatDateTime(dateString: string | null | undefined): string {
  if (!dateString) return ''
  try {
    const d = new Date(dateString)
    return new Intl.DateTimeFormat('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(d)
  } catch {
    return dateString
  }
}

export function formatDistanceToNow(dateString: string | null | undefined): string {
  if (!dateString) return ''
  try {
    const d = new Date(dateString)
    const now = new Date()
    const diffMs = now.getTime() - d.getTime()
    const diffSec = Math.floor(diffMs / 1000)
    if (diffSec < 60) return 'Just now'
    const diffMin = Math.floor(diffSec / 60)
    if (diffMin < 60) return `${diffMin}m ago`
    const diffHours = Math.floor(diffMin / 60)
    if (diffHours < 24) return `${diffHours}h ago`
    const diffDays = Math.floor(diffHours / 24)
    if (diffDays < 30) return `${diffDays}d ago`
    return formatDate(dateString)
  } catch {
    return dateString
  }
}
