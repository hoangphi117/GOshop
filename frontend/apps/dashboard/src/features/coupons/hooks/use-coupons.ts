import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { CouponFormData } from '../types'

export interface CouponQueryParams {
  page?: number
  limit?: number
  search?: string
  is_active?: boolean
}

export function useCoupons(params: CouponQueryParams = {}) {
  return useQuery({
    queryKey: ['adminCoupons', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/coupons', {
        params: {
          query: {
            page: params.page ?? 1,
            limit: params.limit ?? 10,
            search: params.search || undefined,
            is_active: params.is_active,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch coupons')
      }
      return data
    },
  })
}

export function useCouponMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['adminCoupons'] })
  }

  const createCoupon = useMutation({
    mutationFn: async (payload: CouponFormData) => {
      const { data, error } = await api.POST('/admin/coupons', {
        body: {
          code: payload.code,
          name: payload.name,
          description: payload.description,
          type: payload.type,
          discount_value: payload.discount_value,
          min_order_amount: payload.min_order_amount,
          max_discount_amount: payload.max_discount_amount,
          usage_limit: payload.usage_limit,
          user_usage_limit: payload.user_usage_limit,
          start_date: payload.start_date,
          end_date: payload.end_date,
          is_active: payload.is_active,
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to create coupon')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const updateCoupon = useMutation({
    mutationFn: async ({
      uuid,
      payload,
    }: {
      uuid: string
      payload: Partial<CouponFormData>
    }) => {
      const { data, error } = await api.PUT('/admin/coupons/{uuid}', {
        params: { path: { uuid } },
        body: {
          name: payload.name,
          description: payload.description,
          discount_value: payload.discount_value,
          min_order_amount: payload.min_order_amount,
          max_discount_amount: payload.max_discount_amount,
          usage_limit: payload.usage_limit,
          user_usage_limit: payload.user_usage_limit,
          end_date: payload.end_date,
          is_active: payload.is_active,
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to update coupon')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const deleteCoupon = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/admin/coupons/{uuid}', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to delete coupon')
      }
      return data
    },
    onSuccess: invalidate,
  })

  return {
    createCoupon,
    updateCoupon,
    deleteCoupon,
  }
}
