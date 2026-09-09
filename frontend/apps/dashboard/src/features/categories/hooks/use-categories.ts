import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import type { CategoryFormData } from '../types'

export interface CategoryQueryParams {
  page?: number
  limit?: number
  search?: string
  sort?: string
  order_by?: string
}

export function useCategories(params: CategoryQueryParams = {}) {
  return useQuery({
    queryKey: ['categories', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/categories', {
        params: {
          query: {
            page: params.page ?? 1,
            limit: params.limit ?? 10,
            search: params.search || undefined,
            sort: params.sort || undefined,
            order_by: params.order_by || undefined,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch categories')
      }
      return data
    },
  })
}

export function useCategoryMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['categories'] })
  }

  const createCategory = useMutation({
    mutationFn: async (payload: CategoryFormData) => {
      const { data, error } = await api.POST('/admin/categories', {
        body: {
          category_name: payload.category_name,
          description: payload.description,
          status: payload.status,
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to create category')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const updateCategory = useMutation({
    mutationFn: async ({ uuid, payload }: { uuid: string; payload: CategoryFormData }) => {
      const { data, error } = await api.PUT('/admin/categories/{uuid}', {
        params: { path: { uuid } },
        body: {
          category_name: payload.category_name,
          description: payload.description,
          status: payload.status,
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to update category')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const deleteCategory = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/admin/categories/{uuid}/soft-delete', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to delete category')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const restoreCategory = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.PUT('/admin/categories/{uuid}/restore', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to restore category')
      }
      return data
    },
    onSuccess: invalidate,
  })

  const trashCategory = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/admin/categories/{uuid}/trash', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to permanently delete category')
      }
      return data
    },
    onSuccess: invalidate,
  })

  return {
    createCategory,
    updateCategory,
    deleteCategory,
    restoreCategory,
    trashCategory,
  }
}
