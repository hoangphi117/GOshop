import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import type {
  BulkCancelOrdersInput,
  BulkOperationResult,
  BulkUpdateStatusInput,
} from '../types'

type CustomPostFn = <T>(
  url: string,
  options: { body: unknown }
) => Promise<{ data?: { data?: T }; error?: { error?: string } }>

const customPost = api.POST as unknown as CustomPostFn

export function useBulkOrderActions() {
  const queryClient = useQueryClient()

  const bulkUpdateStatus = useMutation({
    mutationFn: async (input: BulkUpdateStatusInput) => {
      const { data, error } = await customPost<BulkOperationResult>(
        '/admin/orders/bulk-status',
        { body: input }
      )
      if (error) {
        throw new Error(
          error.error || 'Failed to update selected orders'
        )
      }
      return data?.data
    },
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['adminOrders'] })
      if (res && res.failed > 0) {
        toast.warning(
          `Updated ${res.succeeded} orders. ${res.failed} orders could not be updated.`
        )
      } else {
        toast.success(`Successfully updated ${res?.succeeded || 0} orders`)
      }
    },
    onError: (err: Error) => {
      toast.error(err.message || 'Bulk status update failed')
    },
  })

  const bulkCancelOrders = useMutation({
    mutationFn: async (input: BulkCancelOrdersInput) => {
      const { data, error } = await customPost<BulkOperationResult>(
        '/admin/orders/bulk-cancel',
        { body: input }
      )
      if (error) {
        throw new Error(
          error.error || 'Failed to cancel selected orders'
        )
      }
      return data?.data
    },
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ['adminOrders'] })
      if (res && res.failed > 0) {
        toast.warning(
          `Cancelled ${res.succeeded} orders. ${res.failed} orders could not be cancelled.`
        )
      } else {
        toast.success(`Successfully cancelled ${res?.succeeded || 0} orders`)
      }
    },
    onError: (err: Error) => {
      toast.error(err.message || 'Bulk cancellation failed')
    },
  })

  return {
    bulkUpdateStatus,
    bulkCancelOrders,
    isProcessing: bulkUpdateStatus.isPending || bulkCancelOrders.isPending,
  }
}
