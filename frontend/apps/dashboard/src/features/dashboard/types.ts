import type { components } from '@goshop/api-client'

export type OverviewStats = components['schemas']['goshop_internal_dto_v1.OverviewStatsDTO']
export type RevenueChartPoint = components['schemas']['goshop_internal_dto_v1.RevenueChartPointDTO']
export type RecentOrder = components['schemas']['goshop_internal_dto_v1.RecentOrderDTO']
export type TopProduct = components['schemas']['goshop_internal_dto_v1.TopProductDTO']

export type OrderStatusDistributionResponse = components['schemas']['goshop_internal_dto_v1.OrderStatusDistributionResponse']
export type OrderStatusDistributionItem = components['schemas']['goshop_internal_dto_v1.OrderStatusDistributionItemDTO']

export type LowStockProductResponse = components['schemas']['goshop_internal_dto_v1.LowStockProductResponse']
export type LowStockProduct = components['schemas']['goshop_internal_dto_v1.LowStockProductDTO']

export type PaymentMethodBreakdownResponse = components['schemas']['goshop_internal_dto_v1.PaymentMethodBreakdownResponse']
export type PaymentMethodBreakdownItem = components['schemas']['goshop_internal_dto_v1.PaymentMethodBreakdownItemDTO']

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

export type CartFunnelStats = components['schemas']['goshop_internal_dto_v1.CartFunnelDTO']

export interface CartFunnelQueryParams {
  days?: number
  from_date?: string
  to_date?: string
}

export type ReturnAnalyticsResponse = components['schemas']['goshop_internal_dto_v1.ReturnAnalyticsResponse']
export type ReturnReasonBreakdown = components['schemas']['goshop_internal_dto_v1.ReturnReasonBreakdownDTO']
export type TopReturnedProduct = components['schemas']['goshop_internal_dto_v1.TopReturnedProductDTO']

export interface ReturnAnalyticsQueryParams {
  days?: number
  from_date?: string
  to_date?: string
}

export type CategorySalesBreakdownResponse = components['schemas']['goshop_internal_dto_v1.CategorySalesBreakdownResponse']
export type CategorySalesItem = components['schemas']['goshop_internal_dto_v1.CategorySalesItemDTO']

export interface CategorySalesQueryParams {
  days?: number
  from_date?: string
  to_date?: string
}


