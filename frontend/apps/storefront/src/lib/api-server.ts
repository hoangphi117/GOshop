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

export async function getProducts(params?: {
  page?: number
  limit?: number
  category?: string
  search?: string
  sort?: string
}): Promise<{ products: Product[]; pagination?: Pagination }> {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', params.page.toString())
  if (params?.limit) query.set('limit', params.limit.toString())
  if (params?.category) query.set('category', params.category)
  if (params?.search) query.set('search', params.search)
  if (params?.sort) query.set('sort', params.sort)

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

export async function getProductByUuidOrSlug(
  identifier: string
): Promise<Product | null> {
  const res = await fetchServer<ApiResponse<Product>>(`/products/${identifier}`, {
    revalidate: 60,
  })
  return res?.data || null
}
