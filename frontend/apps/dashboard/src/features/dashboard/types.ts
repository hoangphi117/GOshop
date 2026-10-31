import type { components } from '@goshop/api-client'

export type OverviewStats = components['schemas']['dto.OverviewStatsDTO']
export type RevenueChartPoint =
  components['schemas']['dto.RevenueChartPointDTO']
export type RecentOrder = components['schemas']['dto.RecentOrderDTO']
export type TopProduct = components['schemas']['dto.TopProductDTO']

export type OrderStatusDistributionResponse =
  components['schemas']['dto.OrderStatusDistributionResponse']
export type OrderStatusDistributionItem =
  components['schemas']['dto.OrderStatusDistributionItemDTO']

export type LowStockProductResponse =
  components['schemas']['dto.LowStockProductResponse']
export type LowStockProduct = components['schemas']['dto.LowStockProductDTO']

export type PaymentMethodBreakdownResponse =
  components['schemas']['dto.PaymentMethodBreakdownResponse']
export type PaymentMethodBreakdownItem =
  components['schemas']['dto.PaymentMethodBreakdownItemDTO']

export interface OverviewStatsQueryParams {
  days?: number
  from_date?: string
  to_date?: string
}

export interface RevenueChartQueryParams {
  period?: 'daily' | 'monthly'
  days?: number
  months?: number
  from_date?: string
  to_date?: string
}

export interface RecentOrdersQueryParams {
  limit?: number
}

export interface TopProductsQueryParams {
  days?: number
  limit?: number
  from_date?: string
  to_date?: string
}

export interface OrderStatusDistributionQueryParams {
  days?: number
  from_date?: string
  to_date?: string
}

export interface LowStockProductsQueryParams {
  threshold?: number
  limit?: number
}

export interface PaymentMethodBreakdownQueryParams {
  days?: number
  from_date?: string
  to_date?: string
}
