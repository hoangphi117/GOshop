export interface ProductVariant {
  uuid: string
  sku: string
  title: string
  price: number
  compare_at_price?: number
  image_url?: string
  attributes: Record<string, string>
  position: number
  status: 'Active' | 'Inactive' | string
  created_at: string
  updated_at: string
}

export interface CreateVariantInput {
  sku: string
  title: string
  price: number
  compare_at_price?: number
  image_url?: string
  attributes: Record<string, string>
  position?: number
  status?: 1 | 2
}

export interface UpdateVariantInput {
  sku?: string
  title?: string
  price?: number
  compare_at_price?: number
  image_url?: string
  attributes?: Record<string, string>
  position?: number
  status?: 1 | 2
}

export interface Product {
  uuid?: string
  name?: string
  slug?: string
  category_uuid?: string
  category_name?: string
  price?: number
  stock?: number
  sku?: string
  image_url?: string
  description?: string
  specifications?: Record<string, string>
  status?: string
  created_at?: string
  rating?: number
  reviews_count?: number
  variants?: ProductVariant[]
}

export interface ProductListResult {
  data: Product[]
  pagination?: {
    page?: number
    limit?: number
    total_records?: number
    total_pages?: number
    has_next?: boolean
    has_prev?: boolean
  }
  /** Set only by the search endpoint: hybrid | keyword | semantic. */
  source?: string
}

export interface ProductFormData {
  name: string
  category_uuid: string
  price: number
  stock: number
  sku?: string
  image_url?: string
  description?: string
  specifications?: Record<string, string>
  status: 1 | 2
  variants?: CreateVariantInput[]
}

export interface CSVRowError {
  row: number
  field?: string
  reason: string
}

export interface ProductCSVImportResponse {
  success_count: number
  error_count: number
  errors: CSVRowError[]
}

export interface ProductSuggestion {
  uuid: string
  name: string
  slug: string
  price: number
  image_url?: string
  category_name?: string
}


