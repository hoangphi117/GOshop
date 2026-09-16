import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type {
  AdminGetReturnsQuery,
  AdminReturnRefundInput,
  AdminReturnSummary,
  AdminUpdateReturnInput,
  OrderReturnDetail,
} from '../types'

export function useAdminReturns(query: AdminGetReturnsQuery = {}) {
  return useQuery<{
    data: AdminReturnSummary[]
    total: number
    page: number
    limit: number
    total_pages: number
  }>({
    queryKey: ['adminReturns', query],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/returns', {
        params: { query },
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to fetch returns'
        )
      }
      return {
        data: (data?.data || []) as unknown as AdminReturnSummary[],
        total: data?.pagination?.total_records || 0,
        page: data?.pagination?.page || 1,
        limit: data?.pagination?.limit || 10,
        total_pages: data?.pagination?.total_pages || 1,
      }
    },
  })
}

export function useOrderReturn(uuid: string | null) {
  return useQuery<OrderReturnDetail | null>({
    queryKey: ['orderReturn', uuid],
    queryFn: async () => {
      if (!uuid) return null
      // GET /admin/returns/{uuid} supports both return UUID and order UUID
      const { data, error } = await api.GET('/admin/returns/{uuid}', {
        params: { path: { uuid } },
      })
      if (error) {
        // Return null on 404 so UI can cleanly show empty state
        return null
      }
      return (data?.data as unknown as OrderReturnDetail) || null
    },
    enabled: Boolean(uuid),
    retry: false,
  })
}

export function useUpdateReturnStatus() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      returnUUID,
      input,
    }: {
      returnUUID: string
      input: AdminUpdateReturnInput
    }) => {
      const { data, error } = await api.PATCH('/admin/returns/{uuid}', {
        params: { path: { uuid: returnUUID } },
        body: input,
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to update return status'
        )
      }
      return data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['adminReturns'] })
      queryClient.invalidateQueries({ queryKey: ['orderReturn'] })
      queryClient.invalidateQueries({ queryKey: ['order'] })
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      queryClient.invalidateQueries({ queryKey: ['orderTimeline'] })
      queryClient.invalidateQueries({ queryKey: ['products'] })
      queryClient.invalidateQueries({ queryKey: ['product'] })
    },
  })
}

export function useProcessReturnRefund() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      returnUUID,
      input,
    }: {
      returnUUID: string
      input: AdminReturnRefundInput
    }) => {
      const { data, error } = await api.POST('/admin/returns/{uuid}/refund', {
        params: { path: { uuid: returnUUID } },
        body: input,
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to process refund'
        )
      }
      return data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['adminReturns'] })
      queryClient.invalidateQueries({ queryKey: ['orderReturn'] })
      queryClient.invalidateQueries({ queryKey: ['order'] })
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      queryClient.invalidateQueries({ queryKey: ['orderTimeline'] })
      queryClient.invalidateQueries({ queryKey: ['products'] })
      queryClient.invalidateQueries({ queryKey: ['product'] })
    },
  })
}
