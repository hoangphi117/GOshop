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
        : ({ next: { revalidate } } as RequestInit & { next: { revalidate: number } })),
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
  min_price?: number
  max_price?: number
  min_rating?: number
  /**
   * Guest identity, forwarded as the anon cookie. Recommendations uses it to
   * personalise; search does not need it yet, but the same call site already
   * has it, so passing it keeps one identity for every read on the page.
   */
  anonId?: string
}): Promise<{ products: Product[]; pagination?: Pagination }> {
  const query = new URLSearchParams()
  if (params?.page) query.set('page', params.page.toString())
  if (params?.limit) query.set('limit', params.limit.toString())
  if (params?.min_price !== undefined && params.min_price > 0) {
    query.set('min_price', params.min_price.toString())
  }
  if (params?.max_price !== undefined && params.max_price > 0) {
    query.set('max_price', params.max_price.toString())
  }
  if (params?.min_rating !== undefined && params.min_rating > 0) {
    query.set('min_rating', params.min_rating.toString())
  }

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

  // Search is answered by the recommendations service, which fuses the keyword
  // ranking with an embedding ranking. Filters are forwarded too: both halves
  // apply them and the total counts only what survives, so one call site keeps
  // working for both paths.
  if (isMeiliSearch) {
    query.set('q', params!.search!.trim())
  }

  if (!isMeiliSearch) {
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
      } else if (sortVal === 'rating_desc' || sortVal === 'top_rated') {
        query.set('order_by', 'product_avg_rating')
        query.set('sort', 'desc')
      } else if (sortVal === 'rating_asc') {
        query.set('order_by', 'product_avg_rating')
        query.set('sort', 'asc')
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

  // The hybrid route answers `{source, items, total}`, not a paginated list.
  // `total` is the number of products that survived the active check, so the
  // page-number row can render the real last page instead of guessing one page
  // ahead of wherever the visitor currently is.
  if (isMeiliSearch) {
    const res = await fetchServer<ApiResponse<RecommendationShelf>>(path, {
      revalidate: 0,
      headers: params?.anonId ? { Cookie: `goshop_anon_id=${params.anonId}` } : undefined,
    })
    const items = res?.data?.items || []
    const total = res?.data?.total ?? items.length
    return {
      products: items,
      pagination: buildSearchPagination(total, items.length, params?.page ?? 1, params?.limit ?? 20),
    }
  }

  const res = await fetchServer<ApiResponse<Product[]>>(path, {
    revalidate: 15,
  })

  return {
    products: res?.data || [],
    pagination: res?.pagination,
  }
}

/**
 * Maps the hybrid route's `{total, items}` onto the pagination shape the grid
 * already renders. `total` is the true number of matches, so the page-number
 * row can show every page at once instead of revealing one more per click.
 */
function buildSearchPagination(
  total: number,
  count: number,
  page: number,
  limit: number
): Pagination {
  const total_pages = Math.ceil(total / limit)
  return {
    page,
    limit,
    total_pages,
    total_records: total,
    // A page past the end still reports the real last page, so its own link
    // ("Prev") leads somewhere and the row does not collapse to one item.
    has_next: page < total_pages,
    has_prev: page > 1,
  }
}

export interface RecommendationShelf {
  source: string
  items: Product[]
  /** Number of matches across all pages; search only. */
  total?: number
}

/**
 * Server-side personalized shelf. Without a visitor cookie the service returns
 * its best-seller fallback and labels it in `source`, so the client can present
 * it honestly instead of as a personal recommendation.
 */
export async function getRecommendations(
  limit = 12,
  anonId?: string
): Promise<RecommendationShelf | null> {
  // The anon cookie is this app's guest identity and lives on the storefront
  // origin, so it has to be forwarded explicitly or every visitor looks new to
  // the recommendations service.
  const res = await fetchServer<ApiResponse<RecommendationShelf>>(
    `/recommendations/personalized?limit=${limit}`,
    {
      revalidate: 0,
      headers: anonId ? { Cookie: `goshop_anon_id=${anonId}` } : undefined,
    }
  )
  return res?.data ?? null
}

export async function getProductSuggestions(q: string, limit = 8): Promise<ProductSuggestion[]> {  if (!q || q.trim().length === 0) {
    return []
  }

  const meiliHost = process.env.INTERNAL_MEILISEARCH_URL || process.env.NEXT_PUBLIC_MEILISEARCH_URL
  const meiliKey = process.env.MEILISEARCH_SEARCH_KEY || process.env.NEXT_PUBLIC_MEILISEARCH_SEARCH_KEY
  if (meiliHost) {
    try {
      const res = await fetch(`${meiliHost}/indexes/products/search`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(meiliKey ? { Authorization: `Bearer ${meiliKey}` } : {}),
        },
        body: JSON.stringify({
          q: q.trim(),
          limit,
          attributesToRetrieve: [
            'product_id',
            'product_uuid',
            'product_name',
            'product_slug',
            'product_price',
            'product_image_url',
            'category_name',
          ],
        }),
        next: { revalidate: 10 },
      })
      if (res.ok) {
        const data = await res.json()
        type MeiliHit = {
          product_id?: number | string
          product_uuid?: string
          product_name?: string
          product_slug?: string
          product_price?: number
          product_image_url?: string
          category_name?: string
        }
        return (data.hits as MeiliHit[] || []).map((hit) => ({
          uuid: hit.product_uuid || String(hit.product_id),
          name: hit.product_name ?? '',
          slug: hit.product_slug ?? '',
          price: hit.product_price ?? 0,
          image_url: hit.product_image_url,
          category_name: hit.category_name,
        }))
      }
    } catch {
      // Fallback to backend API
    }
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
  with_comment_count?: number
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

