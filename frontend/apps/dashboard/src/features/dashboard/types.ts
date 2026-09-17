import type { components } from '@goshop/api-client'

export type OverviewStats = components['schemas']['goshop_internal_dto_v1.OverviewStatsDTO']
export type RevenueChartPoint = components['schemas']['goshop_internal_dto_v1.RevenueChartPointDTO']
export type RecentOrder = components['schemas']['goshop_internal_dto_v1.RecentOrderDTO']
export type TopProduct = components['schemas']['goshop_internal_dto_v1.TopProductDTO']

export interface RevenueChartQueryParams {
  period?: 'daily' | 'monthly'
  days?: number
  months?: number
}

export interface RecentOrdersQueryParams {
  limit?: number
}

export interface TopProductsQueryParams {
  days?: number
  limit?: number
}
