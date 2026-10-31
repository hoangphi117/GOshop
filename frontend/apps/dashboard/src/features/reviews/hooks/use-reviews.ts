import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { AdminReviewQueryParams, ReplyReviewPayload, ReviewStatus } from '../types'

export function useAdminReviews(params: AdminReviewQueryParams = {}) {
  return useQuery({
    queryKey: ['adminReviews', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/reviews', {
        params: {
          query: {
            page: params.page ?? 1,
            limit: params.limit ?? 10,
            status: params.status,
            rating: params.rating,
            product_uuid: params.product_uuid,
            search: params.search || undefined,
            order_by: params.order_by,
            sort: params.sort,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch reviews')
      }
      return data
    },
  })
}

export function useReviewReports(reviewUuid: string | null) {
  return useQuery({
    queryKey: ['adminReviewReports', reviewUuid],
    queryFn: async () => {
      if (!reviewUuid) return []
      const { data, error } = await api.GET('/admin/reviews/{uuid}/reports', {
        params: {
          path: { uuid: reviewUuid },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch review reports')
      }
      return data?.data || []
    },
    enabled: Boolean(reviewUuid),
  })
}

export function useReviewMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['adminReviews'] })
    queryClient.invalidateQueries({ queryKey: ['adminReviewReports'] })
  }

  const updateStatus = useMutation({
    mutationFn: async ({
      uuid,
      status,
    }: {
      uuid: string
      status: ReviewStatus
    }) => {
      const { data, error } = await api.PATCH('/admin/reviews/{uuid}/status', {
        params: { path: { uuid } },
        body: { status },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to update review status')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const replyReview = useMutation({
    mutationFn: async ({
      uuid,
      payload,
    }: {
      uuid: string
      payload: ReplyReviewPayload
    }) => {
      const { data, error } = await api.PATCH('/admin/reviews/{uuid}/reply', {
        params: { path: { uuid } },
        body: payload,
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to reply to review')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const dismissReports = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.POST('/admin/reviews/{uuid}/dismiss-reports', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to dismiss reports')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const forceDeleteReview = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/admin/reviews/{uuid}', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to delete review')
      }
      return data
    },
    onSuccess: invalidate,
  })

  return {
    updateStatus,
    replyReview,
    dismissReports,
    forceDeleteReview,
  }
}
