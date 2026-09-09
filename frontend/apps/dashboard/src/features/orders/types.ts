export interface OrderItem {
  uuid?: string
  product_uuid?: string
  product_name?: string
  product_slug?: string
  image_url?: string
  price?: number
  quantity?: number
  subtotal?: number
}

export interface Order {
  uuid: string
  order_number: string
  user_uuid?: string
  user_name?: string
  shipping_name: string
  shipping_phone: string
  shipping_address: string
  coupon_code?: string
  subtotal_amount?: number
  discount_amount?: number
  shipping_fee?: number
  total_amount: number
  refunded_amount?: number
  payment_method: string
  payment_status: string
  note?: string
  status: string
  items?: OrderItem[]
  created_at: string
}

export interface OrderQueryParams {
  page?: number
  limit?: number
  search?: string
  order_by?: 'order_created_at' | 'order_total_amount'
  sort?: 'asc' | 'desc'
  status?: 1 | 2 | 3 | 4 | 5 | 6
}

export const ORDER_STATUS_MAP: Record<number, { label: string; text: string }> = {
  1: { label: 'Pending', text: 'pending' },
  2: { label: 'Confirmed', text: 'confirmed' },
  3: { label: 'Shipped', text: 'shipped' },
  4: { label: 'Delivered', text: 'delivered' },
  5: { label: 'Cancelled', text: 'cancelled' },
  6: { label: 'Returned', text: 'returned' },
}

export const PAYMENT_STATUS_MAP: Record<number, { label: string; text: string }> = {
  1: { label: 'Unpaid', text: 'unpaid' },
  2: { label: 'Paid', text: 'paid' },
  3: { label: 'Partially Refunded', text: 'partially refunded' },
  4: { label: 'Refunded', text: 'refunded' },
}
