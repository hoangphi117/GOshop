import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type { AdjustStockPayload, StockMovement, StockMovementQueryParams } from '../types'

type CustomGetFn = <T>(
  url: string,
  options?: { params?: { query?: Record<string, unknown> } }
) => Promise<{ data?: { data?: T; pagination?: { total_pages?: number; total_items?: number } }; error?: { error?: string } }>

type CustomPostFn = <T>(
  url: string,
  options: { body: unknown }
) => Promise<{ data?: { data?: T }; error?: { error?: string } }>

const customGet = api.GET as unknown as CustomGetFn
const customPost = api.POST as unknown as CustomPostFn

export function useStockMovements(params: StockMovementQueryParams = {}) {
  return useQuery({
    queryKey: ['stockMovements', params],
    queryFn: async () => {
      const { data, error } = await customGet<StockMovement[]>(
        '/admin/inventory/movements',
        {
          params: {
            query: {
              page: params.page ?? 1,
              limit: params.limit ?? 10,
              product_uuid: params.product_uuid || undefined,
              change_type: params.change_type || undefined,
              from_date: params.from_date || undefined,
              to_date: params.to_date || undefined,
              search: params.search || undefined,
            },
          },
        }
      )
      if (error) {
        throw new Error(error.error || 'Failed to fetch stock movements')
      }
      return data
    },
  })
}

export function useAdjustStock() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (payload: AdjustStockPayload) => {
      const { data, error } = await customPost(
        '/admin/inventory/adjust',
        { body: payload }
      )
      if (error) {
        throw new Error(error.error || 'Failed to adjust stock')
      }
      return data?.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['stockMovements'] })
      queryClient.invalidateQueries({ queryKey: ['products'] })
      queryClient.invalidateQueries({ queryKey: ['adminProducts'] })
      toast.success('Stock adjusted successfully')
    },
    onError: (err: Error) => {
      toast.error(err.message || 'Failed to adjust stock')
    },
  })
}
