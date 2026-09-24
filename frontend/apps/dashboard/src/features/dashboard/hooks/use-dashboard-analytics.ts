import { useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  LowStockProductResponse,
  LowStockProductsQueryParams,
  OrderStatusDistributionQueryParams,
  OrderStatusDistributionResponse,
  OverviewStats,
  OverviewStatsQueryParams,
  PaymentMethodBreakdownQueryParams,
  PaymentMethodBreakdownResponse,
  RecentOrder,
  RecentOrdersQueryParams,
  RevenueChartPoint,
  RevenueChartQueryParams,
  TopProduct,
  TopProductsQueryParams,
  CartFunnelQueryParams,
  CartFunnelStats,
  ReturnAnalyticsQueryParams,
  ReturnAnalyticsResponse,
} from '../types'


export function useAnalyticsOverview(params: OverviewStatsQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsOverview', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/overview', {
        params: {
          query: {
            days: params.days,
            from_date: params.from_date,
            to_date: params.to_date,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch analytics overview')
      }
      return data?.data as OverviewStats
    },
    staleTime: 1000 * 60 * 5, // 5 minutes
  })
}

export function useRevenueChart(params: RevenueChartQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsRevenueChart', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/revenue-chart', {
        params: {
          query: {
            period: params.period,
            days: params.days,
            months: params.months,
            from_date: params.from_date,
            to_date: params.to_date,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch revenue chart')
      }
      return (data?.data || []) as RevenueChartPoint[]
    },
    staleTime: 1000 * 60 * 5,
  })
}

export function useRecentOrders(params: RecentOrdersQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsRecentOrders', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/recent-orders', {
        params: {
          query: {
            limit: params.limit,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch recent orders')
      }
      return (data?.data || []) as RecentOrder[]
    },
    staleTime: 1000 * 60 * 2, // 2 minutes
  })
}

export function useTopProducts(params: TopProductsQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsTopProducts', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/top-products', {
        params: {
          query: {
            days: params.days,
            limit: params.limit,
            from_date: params.from_date,
            to_date: params.to_date,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch top products')
      }
      return (data?.data || []) as TopProduct[]
    },
    staleTime: 1000 * 60 * 5,
  })
}

export function useOrderStatusDistribution(params: OrderStatusDistributionQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsOrderStatusDistribution', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/order-status-distribution', {
        params: {
          query: {
            days: params.days,
            from_date: params.from_date,
            to_date: params.to_date,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch order status distribution')
      }
      return data?.data as OrderStatusDistributionResponse
    },
    staleTime: 1000 * 60 * 2,
  })
}

export function useLowStockAlerts(params: LowStockProductsQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsLowStockAlerts', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/low-stock-alerts', {
        params: {
          query: {
            threshold: params.threshold,
            limit: params.limit,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch low stock alerts')
      }
      return data?.data as LowStockProductResponse
    },
    staleTime: 1000 * 60 * 2,
  })
}

export function usePaymentMethodBreakdown(params: PaymentMethodBreakdownQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsPaymentMethods', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/payment-methods', {
        params: {
          query: {
            days: params.days,
            from_date: params.from_date,
            to_date: params.to_date,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch payment method breakdown')
      }
      return data?.data as PaymentMethodBreakdownResponse
    },
    staleTime: 1000 * 60 * 5,
  })
}

export function useCartFunnel(params: CartFunnelQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsCartFunnel', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/cart-funnel', {
        params: {
          query: {
            from_date: params.from_date,
            to_date: params.to_date,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch cart funnel')
      }
      return data?.data as CartFunnelStats
    },
    staleTime: 1000 * 60 * 2,
  })
}

export function useReturnAnalytics(params: ReturnAnalyticsQueryParams = {}) {
  return useQuery({
    queryKey: ['analyticsReturns', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/analytics/returns', {
        params: {
          query: {
            days: params.days,
            from_date: params.from_date,
            to_date: params.to_date,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch return analytics')
      }
      return data?.data as ReturnAnalyticsResponse
    },
    staleTime: 1000 * 60 * 2,
  })
}


