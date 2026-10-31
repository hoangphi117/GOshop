import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'

export interface AdminUserQueryParams {
  page?: number
  limit?: number
  search?: string
  sort?: 'asc' | 'desc'
  order_by?: 'user_id' | 'user_created_at'
}

export function useAdminUsers(params: AdminUserQueryParams = {}) {
  return useQuery({
    queryKey: ['adminUsers', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/admin/users', {
        params: {
          query: {
            page: params.page ?? 1,
            limit: params.limit ?? 10,
            search: params.search || undefined,
            sort: params.sort,
            order_by: params.order_by,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch users')
      }
      return data
    },
  })
}

export function useUserMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['adminUsers'] })
  }

  const softDeleteUser = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/admin/users/{uuid}/soft-delete', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to soft delete user')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const restoreUser = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.PUT('/admin/users/{uuid}/restore', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to restore user')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const trashUser = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/admin/users/{uuid}/trash', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to permanently delete user')
      }
      return data
    },
    onSuccess: invalidate,
  })

  return {
    softDeleteUser,
    restoreUser,
    trashUser,
  }
}
