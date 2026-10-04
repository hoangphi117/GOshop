export interface Product {
  uuid: string
  name: string
  slug: string
  description?: string
  price: number
  compare_at_price?: number
  stock: number
  sku?: string
  category_uuid?: string
  category_name?: string
  status?: string
  images?: { uuid: string; image_url: string; is_primary?: boolean }[]
  image_url?: string
  variants?: {
    uuid: string
    sku: string
    size?: string
    color?: string
    price: number
    compare_at_price?: number
    stock_quantity: number
  }[]
  created_at?: string
  rating?: number
  reviews_count?: number
}

export interface Category {
  uuid: string
  category_name: string
  slug: string
  description?: string
  image_url?: string
  status?: string
}

export interface Banner {
  uuid: string
  title?: string
  image_url: string
  link_url?: string
  position?: string
  status?: string
}

export interface Pagination {
  page: number
  limit: number
  total_records: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

export interface ApiResponse<T> {
  status: string
  message: string
  data: T
  pagination?: Pagination
}

const RAW_SERVER_API_BASE = (
  process.env.INTERNAL_API_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  'http://localhost:8080'
).replace(/\/$/, '')

const SERVER_API_BASE = RAW_SERVER_API_BASE.endsWith('/api/v1')
  ? RAW_SERVER_API_BASE
  : `${RAW_SERVER_API_BASE}/api/v1`

const API_KEY =
  process.env.API_KEY ||
  process.env.NEXT_PUBLIC_API_KEY ||
  ''

export async function fetchServer<T>(
  path: string,
  init?: RequestInit & { revalidate?: number }
): Promise<T | null> {
  const cleanPath = path.startsWith('/') ? path : `/${path}`
  const url = `${SERVER_API_BASE}${cleanPath}`
  const isDev = process.env.NODE_ENV === 'development'
  const { revalidate = isDev ? 0 : 60, ...customInit } = init || {}

  try {
    const res = await fetch(url, {
      ...customInit,
      headers: {
        'Content-Type': 'application/json',
        'X-API-KEY': API_KEY,
        ...(customInit.headers || {}),
      },
      ...(isDev
        ? { cache: 'no-store' as RequestCache }
        : ({ next: { revalidate } } as any)),
    })

    if (!res.ok) {
      console.error(`[fetchServer] ${res.status} ${res.statusText} for ${url}`)
      return null
    }

    return (await res.json()) as T
  } catch (error) {
    console.error(`[fetchServer] Failed to fetch ${url}:`, error)
    return null
  }
}

export async function getBanners(position?: string): Promise<Banner[]> {
  const query = position ? `?position=${encodeURIComponent(position)}` : ''
  const res = await fetchServer<ApiResponse<Banner[]>>(`/banners${query}`, {
    revalidate: 0,
  })
  return res?.data || []
}

export async function getCategories(): Promise<Category[]> {
  const res = await fetchServer<ApiResponse<Category[]>>('/categories', {
    revalidate: 120,
  })
  return res?.data || []
}

const UUID_REGEX =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

export interface ProductSuggestion {
  uuid: string
  name: string
  slug: string
  price: number
  image_url?: string
  category_name?: string
}

export async function getProducts(params?: {
  page?: number
  limit?: number
  category?: string
  category_uuid?: string
  search?: string
  sort?: string
  order_by?: string
}): Promise<{ products: Product[]; pagination?: Pagination }> {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', params.page.toString())
  if (params?.limit) query.set('limit', params.limit.toString())

  // Resolve category UUID: backend requires 'category_uuid' with valid UUID
  let resolvedCategoryUuid = params?.category_uuid

  if (!resolvedCategoryUuid && params?.category) {
    const rawCategory = params.category.trim()
    if (UUID_REGEX.test(rawCategory)) {
      resolvedCategoryUuid = rawCategory
    } else {
      const categories = await getCategories()
      const matched = categories.find(
        (c) =>
          c.slug.toLowerCase() === rawCategory.toLowerCase() ||
          c.category_name.toLowerCase() === rawCategory.toLowerCase() ||
          c.uuid === rawCategory
      )
      if (matched) {
        resolvedCategoryUuid = matched.uuid
      } else {
        // Provided category does not exist, return empty
        return {
          products: [],
          pagination: undefined,
        }
      }
    }
  }

  if (resolvedCategoryUuid) {
    query.set('category_uuid', resolvedCategoryUuid)
  }

  const isMeiliSearch = Boolean(params?.search && params.search.trim().length >= 1)

  // Use Meilisearch endpoint when search is provided
  if (isMeiliSearch) {
    query.set('q', params!.search!.trim())
    if (params?.sort) {
      const sortVal = params.sort.toLowerCase()
      if (sortVal === 'price_asc') {
        query.set('sort', 'price:asc')
      } else if (sortVal === 'price_desc') {
        query.set('sort', 'price:desc')
      } else if (sortVal === 'newest') {
        query.set('sort', 'created_at:desc')
      } else if (sortVal === 'oldest') {
        query.set('sort', 'created_at:asc')
      }
    }
  } else {
    // Standard PostgreSQL filtering
    if (params?.order_by) {
      query.set('order_by', params.order_by)
    }
    if (params?.sort) {
      const sortVal = params.sort.toLowerCase()
      if (sortVal === 'price_asc') {
        query.set('order_by', 'product_price')
        query.set('sort', 'asc')
      } else if (sortVal === 'price_desc') {
        query.set('order_by', 'product_price')
        query.set('sort', 'desc')
      } else if (sortVal === 'name_asc') {
        query.set('order_by', 'product_name')
        query.set('sort', 'asc')
      } else if (sortVal === 'name_desc') {
        query.set('order_by', 'product_name')
        query.set('sort', 'desc')
      } else if (sortVal === 'newest') {
        query.set('order_by', 'product_created_at')
        query.set('sort', 'desc')
      } else if (sortVal === 'oldest') {
        query.set('order_by', 'product_created_at')
        query.set('sort', 'asc')
      } else if (sortVal === 'asc' || sortVal === 'desc') {
        query.set('sort', sortVal)
      }
    }
  }

  const queryString = query.toString()
  const endpoint = isMeiliSearch ? '/products/search' : '/products'
  const path = `${endpoint}${queryString ? `?${queryString}` : ''}`

  const res = await fetchServer<ApiResponse<Product[]>>(path, {
    revalidate: 15,
  })

  return {
    products: res?.data || [],
    pagination: res?.pagination,
  }
}

export async function getProductSuggestions(q: string, limit = 8): Promise<ProductSuggestion[]> {
  if (!q || q.trim().length === 0) {
    return []
  }
  const query = new URLSearchParams({ q: q.trim(), limit: limit.toString() })
  const res = await fetchServer<ApiResponse<ProductSuggestion[]>>(`/products/suggest?${query.toString()}`, {
    revalidate: 10,
  })
  return res?.data || []
}

export interface Review {
  uuid: string
  product_uuid?: string
  order_uuid?: string
  reviewer_name: string
  rating: number
  title?: string
  comment?: string
  images?: string[]
  video_url?: string
  has_media?: boolean
  can_edit?: boolean
  reply?: string
  reply_at?: string
  helpful_count?: number
  created_at: string
  updated_at?: string
}

export interface ReviewSummary {
  total_reviews: number
  avg_rating: number
  with_media_count: number
  five_star: number
  four_star: number
  three_star: number
  two_star: number
  one_star: number
}

export interface GetProductReviewsParams {
  page?: number
  limit?: number
  rating?: number
  has_media?: boolean
  order_by?: 'created_at' | 'rating' | 'helpful'
  sort?: 'asc' | 'desc'
}

export async function getProductByUuidOrSlug(
  identifier: string
): Promise<Product | null> {
  if (!identifier) return null
  const decoded = decodeURIComponent(identifier).trim()

  // 1. Direct fetch with identifier (Go backend natively handles both UUID and slug)
  const res = await fetchServer<ApiResponse<Product>>(
    `/products/${encodeURIComponent(decoded)}`,
    {
      revalidate: 60,
    }
  )
  if (res?.data) {
    return res.data
  }

  // 2. Fallback: If not found and identifier is a slug, search in products catalog
  if (!UUID_REGEX.test(decoded)) {
    const listRes = await getProducts({ limit: 100 })
    const matched = listRes.products.find(
      (p) =>
        p.slug?.toLowerCase() === decoded.toLowerCase() ||
        p.uuid === decoded
    )
    if (matched) {
      const fallbackRes = await fetchServer<ApiResponse<Product>>(
        `/products/${matched.uuid}`,
        {
          revalidate: 60,
        }
      )
      return fallbackRes?.data || null
    }
  }

  return null
}

export async function getProductReviews(
  productUuid: string,
  params?: GetProductReviewsParams
): Promise<{ reviews: Review[]; pagination?: { page: number; limit: number; total_records: number; total_pages: number } }> {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', String(params.page))
  if (params?.limit) query.set('limit', String(params.limit))
  if (params?.rating) query.set('rating', String(params.rating))
  if (params?.has_media !== undefined) query.set('has_media', String(params.has_media))
  if (params?.order_by) query.set('order_by', params.order_by)
  if (params?.sort) query.set('sort', params.sort)

  const queryString = query.toString()
  const path = `/products/${productUuid}/reviews${queryString ? `?${queryString}` : ''}`

  const res = await fetchServer<ApiResponse<Review[]>>(path, {
    revalidate: 15,
  })

  return {
    reviews: res?.data || [],
    pagination: res?.pagination,
  }
}

export async function getProductReviewSummary(
  productUuid: string
): Promise<ReviewSummary | null> {
  const res = await fetchServer<ApiResponse<ReviewSummary>>(
    `/products/${productUuid}/reviews/summary`,
    {
      revalidate: 15,
    }
  )
  return res?.data || null
}

