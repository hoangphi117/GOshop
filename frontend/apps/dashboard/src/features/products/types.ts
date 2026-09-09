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
