import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { directUpload, type UploadFolder } from '@goshop/api-client'
import { api, fetchWithAuth } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import type {
  Product,
  ProductCSVImportResponse,
  ProductFormData,
  ProductListResult,
  ProductSuggestion,
} from '../types'

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
  return useQuery<ProductListResult>({
    queryKey: ['products', params],
    queryFn: async () => {
      const isSearch = Boolean(params.search && params.search.trim().length >= 1)

      if (isSearch) {
        const { data, error } = await api.GET('/api/v1/products/search', {
          params: {
            query: {
              q: params.search!.trim(),
              category_uuid: params.category_uuid || undefined,
              min_price: params.min_price,
              max_price: params.max_price,
              sort: params.sort || undefined,
              page: params.page ?? 1,
              limit: params.limit ?? 10,
            },
          },
        })
        if (error) {
          throw new Error((error as { error?: string })?.error || 'Failed to search products')
        }
        // /products/search answers with a rating-ordered shelf, not the admin
        // list envelope. Normalize it so callers only ever see one shape.
        const items = (data?.items ?? []) as Product[]
        return { data: items, pagination: undefined, source: data?.source }
      }

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
      return {
        data: (data?.data ?? []) as Product[],
        pagination: (data as { pagination?: ProductListResult['pagination'] })?.pagination,
      }
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
      return (data?.data ?? null) as Product | null
    },
    enabled: Boolean(uuid),
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
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
    queryClient.invalidateQueries({ queryKey: ['product'] })
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
      const result = await directUpload(file, folder as UploadFolder, fetchWithAuth)
      return { url: result.url, file_name: result.name, size: result.size }
    },
  })
}

export function useImportProductsCSV() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (file: File) => {
      const { accessToken } = useAuthStore.getState().auth
      if (!accessToken) {
        throw new Error('Authentication required. Please sign in to import products.')
      }

      const formData = new FormData()
      formData.append('file', file)

      const res = await fetchWithAuth('/api/v1/admin/products/import-csv', {
        method: 'POST',
        body: formData,
      })

      const json = await res.json().catch(() => ({}))
      if (!res.ok) {
        throw new Error(json.error || json.detail || 'Failed to upload CSV')
      }

      return json.data as ProductCSVImportResponse
    },
    onSuccess: (data) => {
      if (data && data.success_count > 0) {
        queryClient.invalidateQueries({ queryKey: ['products'] })
      }
    },
  })
}

export function useProductSuggestions(query: string, limit = 8) {
  return useQuery({
    queryKey: ['product-suggestions', query, limit],
    queryFn: async () => {
      if (!query || query.trim().length === 0) return []
      const { data, error } = await api.GET('/api/v1/products/suggest', {
        params: {
          query: {
            q: query.trim(),
            limit,
          },
        },
      })
      if (error) {
        return []
      }
      return ((data as { data?: ProductSuggestion[] })?.data ?? []) as ProductSuggestion[]
    },
    enabled: Boolean(query && query.trim().length >= 1),
    staleTime: 60 * 1000,
  })
}


