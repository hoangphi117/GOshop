import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import type { ProductFormData } from '../types'

export interface ProductQueryParams {
  page?: number
  limit?: number
  search?: string
  category_uuid?: string
  min_price?: number
  max_price?: number
  status?: number
  sort?: string
  order_by?: string
}

export function useProducts(params: ProductQueryParams = {}) {
  return useQuery({
    queryKey: ['products', params],
    queryFn: async () => {
      const { data, error } = await api.GET('/api/v1/products', {
        params: {
          query: {
            page: params.page ?? 1,
            limit: params.limit ?? 10,
            search: params.search || undefined,
            category_uuid: params.category_uuid || undefined,
            min_price: params.min_price,
            max_price: params.max_price,
            status: params.status,
            sort: params.sort || undefined,
            order_by: params.order_by || undefined,
          },
        },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch products')
      }
      return data
    },
  })
}

export function useProduct(uuid?: string) {
  return useQuery({
    queryKey: ['product', uuid],
    queryFn: async () => {
      if (!uuid) return null
      const { data, error } = await api.GET('/api/v1/products/{uuid}', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error((error as { error?: string })?.error || 'Failed to fetch product')
      }
      return data
    },
    enabled: Boolean(uuid),
  })
}

function formatApiError(error: unknown, fallback: string): string {
  if (!error) return fallback
  if (typeof error === 'string') return error
  if (typeof error === 'object') {
    const errObj = error as Record<string, unknown>
    if (typeof errObj.error === 'string') return errObj.error
    if (typeof errObj.message === 'string') return errObj.message
    if (typeof errObj.detail === 'string') return errObj.detail
    if (typeof errObj.error === 'object' && errObj.error !== null) {
      const messages = Object.values(errObj.error as Record<string, unknown>)
        .map((v) => (typeof v === 'string' ? v : JSON.stringify(v)))
        .filter(Boolean)
      if (messages.length > 0) return messages.join(', ')
    }
  }
  return fallback
}

export function useProductMutations() {
  const queryClient = useQueryClient()

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['products'] })
  }

  const createProduct = useMutation({
    mutationFn: async (payload: ProductFormData) => {
      const { data, error } = await api.POST('/api/v1/admin/products', {
        body: {
          name: payload.name,
          category_uuid: payload.category_uuid,
          price: payload.price,
          stock: payload.stock,
          sku: payload.sku,
          image_url: payload.image_url,
          description: payload.description,
          status: payload.status,
        },
      })
      if (error) {
        throw new Error(formatApiError(error, 'Failed to create product'))
      }
      return data
    },
    onSuccess: invalidate,
  })

  const updateProduct = useMutation({
    mutationFn: async ({ uuid, payload }: { uuid: string; payload: ProductFormData }) => {
      const { data, error } = await api.PUT('/api/v1/admin/products/{uuid}', {
        params: { path: { uuid } },
        body: {
          name: payload.name,
          category_uuid: payload.category_uuid,
          price: payload.price,
          stock: payload.stock,
          sku: payload.sku,
          image_url: payload.image_url,
          description: payload.description,
          status: payload.status,
        },
      })
      if (error) {
        throw new Error(formatApiError(error, 'Failed to update product'))
      }
      return data
    },
    onSuccess: invalidate,
  })

  const deleteProduct = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.DELETE('/api/v1/admin/products/{uuid}', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error(formatApiError(error, 'Failed to delete product'))
      }
      return data
    },
    onSuccess: invalidate,
  })

  const restoreProduct = useMutation({
    mutationFn: async (uuid: string) => {
      const { data, error } = await api.PUT('/api/v1/admin/products/{uuid}/restore', {
        params: { path: { uuid } },
      })
      if (error) {
        throw new Error(formatApiError(error, 'Failed to restore product'))
      }
      return data
    },
    onSuccess: invalidate,
  })

  return {
    createProduct,
    updateProduct,
    deleteProduct,
    restoreProduct,
  }
}

export function useUploadImage() {
  return useMutation({
    mutationFn: async ({ file, folder = 'products' }: { file: File; folder?: string }) => {
      const token = useAuthStore.getState().auth.accessToken
      const formData = new FormData()
      formData.append('file', file)
      formData.append('folder', folder)

      const rawBase = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
      const cleanBase = rawBase.replace(/\/+$/, '')
      const uploadUrl = cleanBase.endsWith('/api/v1') ? `${cleanBase}/upload` : `${cleanBase}/api/v1/upload`
      const res = await fetch(uploadUrl, {
        method: 'POST',
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          'X-API-KEY': import.meta.env.VITE_API_KEY || 'abcd-1234-1111',
        },
        body: formData,
      })

      if (!res.ok) {
        const errJson = await res.json().catch(() => ({}))
        throw new Error(errJson.error || errJson.detail || 'Image upload failed')
      }

      const json = await res.json()
      return json.data as { url: string; file_name: string; size: number }
    },
  })
}
