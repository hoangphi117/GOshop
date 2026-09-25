export interface Category {
  category_id?: number
  uuid?: string
  category_name?: string
  slug?: string
  description?: string
  status?: string
  created_at?: string
}

export interface CategoryFormData {
  category_name: string
  description?: string
  status: 1 | 2
}
