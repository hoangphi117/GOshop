import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  CalculateFeeInput,
  CreateShippingInput,
  District,
  OrderTimeline,
  PrintBillResponse,
  Province,
  Ward,
} from '../types'

export function useOrderTimeline(uuid: string | null) {
  return useQuery<OrderTimeline | null>({
    queryKey: ['orderTimeline', uuid],
    queryFn: async () => {
      if (!uuid) return null
      const { data, error } = await api.GET('/orders/{uuid}/timeline', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to fetch tracking timeline'
        )
      }
      return (data?.data as unknown as OrderTimeline) || null
    },
    enabled: Boolean(uuid),
  })
}

export function useProvinces() {
  return useQuery<Province[]>({
    queryKey: ['shippingProvinces'],
    queryFn: async () => {
      const { data, error } = await api.GET('/shipping/provinces')
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to fetch provinces'
        )
      }
      return (data?.data as unknown as Province[]) || []
    },
    staleTime: 1000 * 60 * 60, // 1 hour cache
  })
}

export function useDistricts(provinceId: number | null) {
  return useQuery<District[]>({
    queryKey: ['shippingDistricts', provinceId],
    queryFn: async () => {
      if (!provinceId) return []
      const { data, error } = await api.GET('/shipping/districts', {
        params: { query: { province_id: provinceId } },
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to fetch districts'
        )
      }
      return (data?.data as unknown as District[]) || []
    },
    enabled: Boolean(provinceId),
    staleTime: 1000 * 60 * 30,
  })
}

export function useWards(districtId: number | null) {
  return useQuery<Ward[]>({
    queryKey: ['shippingWards', districtId],
    queryFn: async () => {
      if (!districtId) return []
      const { data, error } = await api.GET('/shipping/wards', {
        params: { query: { district_id: districtId } },
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to fetch wards'
        )
      }
      return (data?.data as unknown as Ward[]) || []
    },
    enabled: Boolean(districtId),
    staleTime: 1000 * 60 * 30,
  })
}

export function useCalculateShippingFee() {
  return useMutation({
    mutationFn: async (input: CalculateFeeInput) => {
      const { data, error } = await api.POST('/orders/calculate-fee', {
        body: input,
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to calculate shipping fee'
        )
      }
      return data?.data
    },
  })
}

export function useShippingMutations() {
  const queryClient = useQueryClient()

  const invalidate = (uuid?: string) => {
    queryClient.invalidateQueries({ queryKey: ['adminOrders'] })
    if (uuid) {
      queryClient.invalidateQueries({ queryKey: ['orderDetails', uuid] })
      queryClient.invalidateQueries({ queryKey: ['orderTimeline', uuid] })
    }
  }

  const createShipping = useMutation({
    mutationFn: async ({
      uuid,
      input,
    }: {
      uuid: string
      input: CreateShippingInput
    }) => {
      const { data, error } = await api.POST('/admin/orders/{uuid}/shipping', {
        params: { path: { uuid } },
        body: input,
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to create shipping order'
        )
      }
      return data?.data
    },
    onSuccess: (_, vars) => invalidate(vars.uuid),
  })

  const cancelShipping = useMutation({
    mutationFn: async ({
      uuid,
      reason,
    }: {
      uuid: string
      reason?: string
    }) => {
      const { data, error } = await api.POST(
        '/admin/orders/{uuid}/shipping/cancel',
        {
          params: { path: { uuid } },
          body: { reason: reason || '' },
        }
      )
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to cancel shipping'
        )
      }
      return data?.data
    },
    onSuccess: (_, vars) => invalidate(vars.uuid),
  })

  const syncTracking = useMutation({
    mutationFn: async ({ uuid }: { uuid: string }) => {
      const { data, error } = await api.POST(
        '/admin/orders/{uuid}/sync-tracking',
        {
          params: { path: { uuid } },
        }
      )
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to sync tracking'
        )
      }
      return data?.data
    },
    onSuccess: (_, vars) => invalidate(vars.uuid),
  })

  const fetchPrintBill = async (uuid: string): Promise<PrintBillResponse> => {
    const { data, error } = await api.GET(
      '/admin/orders/{uuid}/shipping/print-bill',
      {
        params: { path: { uuid } },
      }
    )
    if (error) {
      throw new Error(
        (error as { error?: string })?.error || 'Failed to generate print bill URL'
      )
    }
    return data?.data as unknown as PrintBillResponse
  }

  return {
    createShipping,
    cancelShipping,
    syncTracking,
    fetchPrintBill,
  }
}
