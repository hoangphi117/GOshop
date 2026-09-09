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

const SERVER_API_BASE =
  process.env.INTERNAL_API_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  'http://localhost:8080'

const API_KEY = process.env.API_KEY || 'abcd-1234-1111'

export async function fetchServer<T>(
  path: string,
  init?: RequestInit & { revalidate?: number }
): Promise<T | null> {
  const url = `${SERVER_API_BASE}${path.startsWith('/') ? path : `/${path}`}`
  const { revalidate = 60, ...customInit } = init || {}

  try {
    const res = await fetch(url, {
      ...customInit,
      headers: {
        'Content-Type': 'application/json',
        'X-API-KEY': API_KEY,
        ...(customInit.headers || {}),
      },
      next: {
        revalidate,
      },
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

export async function getBanners(): Promise<Banner[]> {
  const res = await fetchServer<ApiResponse<Banner[]>>('/banners', {
    revalidate: 60,
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

  // Backend requires search keyword to be at least 3 characters
  if (params?.search && params.search.trim().length >= 3) {
    query.set('search', params.search.trim())
  }

  // Handle sorting
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

  const queryString = query.toString()
  const path = `/products${queryString ? `?${queryString}` : ''}`

  const res = await fetchServer<ApiResponse<Product[]>>(path, {
    revalidate: 30,
  })

  return {
    products: res?.data || [],
    pagination: res?.pagination,
  }
}

export interface Review {
  uuid: string
  user_name?: string
  rating: number
  comment?: string
  created_at: string
}

export interface ReviewSummary {
  average_rating: number
  total_reviews: number
  rating_breakdown?: Record<string, number>
}

export async function getProductByUuidOrSlug(
  identifier: string
): Promise<Product | null> {
  let targetUuid = identifier

  // If identifier is a slug, resolve it to UUID
  if (!UUID_REGEX.test(identifier)) {
    const listRes = await getProducts({ limit: 100 })
    const matched = listRes.products.find(
      (p) =>
        p.slug?.toLowerCase() === identifier.toLowerCase() ||
        p.uuid === identifier
    )
    if (matched) {
      targetUuid = matched.uuid
    } else {
      return null
    }
  }

  const res = await fetchServer<ApiResponse<Product>>(
    `/products/${targetUuid}`,
    {
      revalidate: 60,
    }
  )
  return res?.data || null
}

export async function getProductReviews(productUuid: string): Promise<Review[]> {
  const res = await fetchServer<ApiResponse<Review[]>>(
    `/products/${productUuid}/reviews`,
    {
      revalidate: 60,
    }
  )
  return res?.data || []
}

export async function getProductReviewSummary(
  productUuid: string
): Promise<ReviewSummary | null> {
  const res = await fetchServer<ApiResponse<ReviewSummary>>(
    `/products/${productUuid}/reviews/summary`,
    {
      revalidate: 60,
    }
  )
  return res?.data || null
}
