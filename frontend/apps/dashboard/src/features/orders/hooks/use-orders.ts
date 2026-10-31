import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, fetchWithAuth } from '@/lib/api'
import type { OrderQueryParams } from '../types'

export function useOrders(params: OrderQueryParams = {}) {
  return useQuery({
    queryKey: ['adminOrders', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/orders', {
        params: {
          query: {
            page: params.page ?? 1,
            limit: params.limit ?? 10,
            search: params.search || undefined,
            order_by: params.order_by,
            sort: params.sort,
            status: params.status,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch orders')
      }
      return data
    },
  })
}

export function useOrderDetails(uuid: string | null) {
  return useQuery({
    queryKey: ['orderDetails', uuid],
    queryFn: async () => {
      if (!uuid) return null
      const { data, error } = await api.GET('/orders/{uuid}', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to fetch order details'
        )
      }
      return data?.data
    },
    enabled: Boolean(uuid),
  })
}

export function useOrderMutations() {
  const queryClient = useQueryClient()

  const invalidate = (uuid?: string) => {
    queryClient.invalidateQueries({ queryKey: ['adminOrders'] })
    if (uuid) {
      queryClient.invalidateQueries({ queryKey: ['orderDetails', uuid] })
    }
  }

  const updateOrderStatus = useMutation({
    mutationFn: async ({
      uuid,
      status,
    }: {
      uuid: string
      status: 1 | 2 | 3 | 4 | 5 | 6
    }) => {
      const { data, error } = await api.PATCH('/admin/orders/{uuid}', {
        params: { path: { uuid } },
        body: { status },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to update order status')
      }
      return data
    },
    onSuccess: (_, vars) => invalidate(vars.uuid),
  })

  const updateOrderPaymentStatus = useMutation({
    mutationFn: async ({
      uuid,
      payment_status,
      note,
    }: {
      uuid: string
      payment_status: 2 | 3 | 4
      note?: string
    }) => {
      const { data, error } = await api.PATCH(
        '/admin/orders/{uuid}/payment-status',
        {
          params: { path: { uuid } },
          body: {
            payment_status,
            note,
          },
        }
      )
      if (error) {
        throw new Error(
          (error as { error?: string })?.error || 'Failed to update payment status'
        )
      }
      return data
    },
    onSuccess: (_, vars) => invalidate(vars.uuid),
  })

  return {
    updateOrderStatus,
    updateOrderPaymentStatus,
  }
}

export async function downloadOrderInvoice(uuid: string, orderNumber?: string): Promise<void> {
  const res = await fetchWithAuth(`/api/v1/orders/${uuid}/invoice?action=download`)
  if (!res.ok) {
    let errMsg = 'Failed to download invoice'
    try {
      const errJson = await res.json()
      if (errJson?.error) errMsg = errJson.error
    } catch {
      // Non-JSON error
    }
    throw new Error(errMsg)
  }
  const blob = await res.blob()
  const blobUrl = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = blobUrl
  a.download = `invoice-${orderNumber || uuid}.pdf`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(blobUrl)
}
