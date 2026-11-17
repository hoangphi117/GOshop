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
  status?: string
  created_at?: string
  rating?: number
  reviews_count?: number
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
  status: 1 | 2
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


