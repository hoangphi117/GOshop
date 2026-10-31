import type {
  OrderStatusDistributionItem,
  OverviewStats,
  PaymentMethodBreakdownItem,
  RevenueChartPoint,
  TopProduct,
} from '../types'

/**
 * Escapes a single CSV cell value according to RFC 4180
 */
function escapeCSV(val: unknown): string {
  if (val === null || val === undefined) return '""'
  const str = String(val)
  if (str.includes(',') || str.includes('"') || str.includes('\n') || str.includes('\r')) {
    return `"${str.replace(/"/g, '""')}"`
  }
  return `"${str}"`
}

/**
 * Converts headers and rows into a CSV string
 */
function toCSV(headers: string[], rows: (string | number | undefined | null)[][]): string {
  const headerLine = headers.map(escapeCSV).join(',')
  const rowLines = rows.map((r) => r.map(escapeCSV).join(','))
  return [headerLine, ...rowLines].join('\r\n')
}

/**
 * Triggers a client-side file download with UTF-8 BOM for Microsoft Excel compatibility
 */
export function downloadCSV(filename: string, csvContent: string): void {
  const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.setAttribute('href', url)
  link.setAttribute('download', filename.endsWith('.csv') ? filename : `${filename}.csv`)
  link.style.visibility = 'hidden'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

/**
 * Export KPI Summary Cards
 */
export function exportOverviewKPI(
  stats: OverviewStats | undefined,
  dateRangeLabel: string
): void {
  if (!stats) return

  const headers = ['Metric', 'Value', 'Comparison vs Previous Period', 'Date Range']
  const rows = [
    ['Total Net Revenue (VND)', stats.total_revenue ?? 0, `${stats.revenue_growth ?? 0}%`, dateRangeLabel],
    ['Total Orders', stats.total_orders ?? 0, `${stats.order_growth ?? 0}%`, dateRangeLabel],
    ['New Customers', stats.new_customers ?? 0, `${stats.customer_growth ?? 0}%`, dateRangeLabel],
    ['Pending Orders', stats.pending_orders ?? 0, 'N/A', dateRangeLabel],
  ]

  const dateSlug = dateRangeLabel.replace(/[^a-zA-Z0-9]/g, '_')
  downloadCSV(`analytics_kpi_summary_${dateSlug}.csv`, toCSV(headers, rows))
}

/**
 * Export Revenue & Daily Orders
 */
export function exportRevenueDaily(
  points: RevenueChartPoint[] | undefined,
  dateRangeLabel: string
): void {
  if (!points || points.length === 0) return

  const headers = ['Date', 'Net Revenue (VND)', 'Total Orders']
  const rows = points.map((p) => [p.date, p.revenue, p.order_count])

  const dateSlug = dateRangeLabel.replace(/[^a-zA-Z0-9]/g, '_')
  downloadCSV(`analytics_daily_revenue_${dateSlug}.csv`, toCSV(headers, rows))
}

/**
 * Export Top Selling Products
 */
export function exportTopProducts(
  products: TopProduct[] | undefined,
  dateRangeLabel: string
): void {
  if (!products || products.length === 0) return

  const headers = [
    'Product Name',
    'Category',
    'Unit Price (VND)',
    'Units Sold',
    'Total Revenue (VND)',
    'Product Slug',
  ]
  const rows = products.map((p) => [
    p.name,
    p.category_name,
    p.price,
    p.total_units_sold,
    p.total_revenue,
    p.slug,
  ])

  const dateSlug = dateRangeLabel.replace(/[^a-zA-Z0-9]/g, '_')
  downloadCSV(`analytics_top_products_${dateSlug}.csv`, toCSV(headers, rows))
}

/**
 * Export Order Status Breakdown
 */
export function exportOrderStatus(
  items: OrderStatusDistributionItem[] | undefined,
  dateRangeLabel: string
): void {
  if (!items || items.length === 0) return

  const headers = ['Status Code', 'Status Name', 'Order Count', 'Total Amount (VND)', 'Share (%)']
  const rows = items.map((item) => [
    item.status,
    item.status_name,
    item.count,
    item.total_amount,
    `${item.percentage}%`,
  ])

  const dateSlug = dateRangeLabel.replace(/[^a-zA-Z0-9]/g, '_')
  downloadCSV(`analytics_order_status_${dateSlug}.csv`, toCSV(headers, rows))
}

/**
 * Export Payment Methods Breakdown
 */
export function exportPaymentMethods(
  items: PaymentMethodBreakdownItem[] | undefined,
  dateRangeLabel: string
): void {
  if (!items || items.length === 0) return

  const headers = ['Method Code', 'Payment Method Name', 'Order Count', 'Net Revenue (VND)', 'Share (%)']
  const rows = items.map((item) => [
    item.payment_method,
    item.method_name,
    item.order_count,
    item.net_revenue,
    `${item.percentage}%`,
  ])

  const dateSlug = dateRangeLabel.replace(/[^a-zA-Z0-9]/g, '_')
  downloadCSV(`analytics_payment_methods_${dateSlug}.csv`, toCSV(headers, rows))
}

/**
 * Export Consolidated Full Analytics Report (All-in-one CSV)
 */
export function exportFullAnalyticsReport(
  data: {
    stats?: OverviewStats
    revenuePoints?: RevenueChartPoint[]
    topProducts?: TopProduct[]
    orderStatus?: OrderStatusDistributionItem[]
    paymentMethods?: PaymentMethodBreakdownItem[]
  },
  dateRangeLabel: string
): void {
  const sections: string[] = []

  // Report Header
  sections.push(['# GOSHOP BUSINESS ANALYTICS REPORT', `Date Range: ${dateRangeLabel}`, `Generated: ${new Date().toLocaleString()}`].map(escapeCSV).join(','))
  sections.push('')

  // 1. KPI Summary
  if (data.stats) {
    sections.push('=== 1. EXECUTIVE KPI SUMMARY ===')
    const kpiHeaders = ['Metric', 'Value', 'Period Growth']
    const kpiRows = [
      ['Total Net Revenue (VND)', data.stats.total_revenue ?? 0, `${data.stats.revenue_growth ?? 0}%`],
      ['Total Orders', data.stats.total_orders ?? 0, `${data.stats.order_growth ?? 0}%`],
      ['New Customers', data.stats.new_customers ?? 0, `${data.stats.customer_growth ?? 0}%`],
      ['Pending Orders', data.stats.pending_orders ?? 0, 'N/A'],
    ]
    sections.push(toCSV(kpiHeaders, kpiRows))
    sections.push('')
  }

  // 2. Order Status Breakdown
  if (data.orderStatus && data.orderStatus.length > 0) {
    sections.push('=== 2. ORDER STATUS DISTRIBUTION ===')
    const statusHeaders = ['Status Name', 'Order Count', 'Total Amount (VND)', 'Percentage']
    const statusRows = data.orderStatus.map((s) => [s.status_name, s.count, s.total_amount, `${s.percentage}%`])
    sections.push(toCSV(statusHeaders, statusRows))
    sections.push('')
  }

  // 3. Payment Method Breakdown
  if (data.paymentMethods && data.paymentMethods.length > 0) {
    sections.push('=== 3. PAYMENT METHODS PERFORMANCE ===')
    const payHeaders = ['Method Name', 'Order Count', 'Net Revenue (VND)', 'Revenue Share']
    const payRows = data.paymentMethods.map((p) => [p.method_name, p.order_count, p.net_revenue, `${p.percentage}%`])
    sections.push(toCSV(payHeaders, payRows))
    sections.push('')
  }

  // 4. Top Selling Products
  if (data.topProducts && data.topProducts.length > 0) {
    sections.push('=== 4. TOP SELLING PRODUCTS ===')
    const prodHeaders = ['Product Name', 'Category', 'Unit Price (VND)', 'Units Sold', 'Total Revenue (VND)']
    const prodRows = data.topProducts.map((p) => [p.name, p.category_name, p.price, p.total_units_sold, p.total_revenue])
    sections.push(toCSV(prodHeaders, prodRows))
    sections.push('')
  }

  // 5. Daily Revenue Breakdown
  if (data.revenuePoints && data.revenuePoints.length > 0) {
    sections.push('=== 5. DAILY REVENUE TIMELINE ===')
    const revHeaders = ['Date', 'Net Revenue (VND)', 'Order Count']
    const revRows = data.revenuePoints.map((r) => [r.date, r.revenue, r.order_count])
    sections.push(toCSV(revHeaders, revRows))
    sections.push('')
  }

  const dateSlug = dateRangeLabel.replace(/[^a-zA-Z0-9]/g, '_')
  downloadCSV(`goshop_analytics_full_report_${dateSlug}.csv`, sections.join('\r\n'))
}
