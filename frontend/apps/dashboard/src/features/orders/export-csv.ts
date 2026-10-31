import type { Order } from './types'

function escapeCSV(val: string | number | undefined | null): string {
  if (val === undefined || val === null) return '""'
  const str = String(val).replace(/"/g, '""')
  return `"${str}"`
}

function escapePhone(phone?: string | null): string {
  if (!phone) return '""'
  const clean = phone.trim()
  return `="${clean}"`
}

/**
 * Exports an array of orders to a downloadable CSV file.
 * Includes UTF-8 BOM (\uFEFF) for native Excel compatibility with accented characters.
 */
export function exportOrdersToCSV(orders: Order[], filename?: string): void {
  if (!orders || orders.length === 0) return

  const headers = [
    'Order Number',
    'Customer Name',
    'Phone',
    'Address',
    'Total Amount (VND)',
    'Payment Method',
    'Payment Status',
    'Order Status',
    'Created At',
  ]

  const rows = orders.map((order) => [
    escapeCSV(order.order_number),
    escapeCSV(order.shipping_name || order.user_name || 'N/A'),
    escapePhone(order.shipping_phone),
    escapeCSV(order.shipping_address || 'N/A'),
    escapeCSV(order.total_amount ?? 0),
    escapeCSV(order.payment_method || 'COD'),
    escapeCSV(order.payment_status || 'Pending'),
    escapeCSV(order.status || 'Pending'),
    escapeCSV(
      order.created_at
        ? new Date(order.created_at).toLocaleString('en-US')
        : 'N/A'
    ),
  ])

  const csvContent =
    '\uFEFF' + [headers.join(','), ...rows.map((r) => r.join(','))].join('\r\n')
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)

  const dateStr = new Date().toISOString().slice(0, 10)
  const a = document.createElement('a')
  a.href = url
  a.download = filename || `orders-export-${dateStr}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
