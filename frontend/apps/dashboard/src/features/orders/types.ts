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
  shipping_provider?: string
  shipping_code?: string
  expected_delivery?: string
  items?: OrderItem[]
  created_at: string
}

export interface TrackingEvent {
  event_id: number
  status: string
  status_text: string
  note?: string
  source: string
  created_at: string
}

export interface OrderTimeline {
  order_uuid: string
  order_number: string
  events: TrackingEvent[]
}

export interface Province {
  province_id: number
  province_name: string
  code?: string
}

export interface District {
  district_id: number
  province_id: number
  district_name: string
  code?: string
}

export interface Ward {
  ward_code: string
  district_id: number
  ward_name: string
}

export interface CreateShippingInput {
  provider: 'ghn' | 'manual'
  district_id: number
  ward_code: string
  weight: number
  length: number
  width: number
  height: number
  required_note?: 'KHONGCHOXEMHANG' | 'CHOXEMHANGKHONGTHU' | 'CHOTHUHANG'
  note?: string
}

export interface PrintBillResponse {
  order_uuid: string
  order_number: string
  shipping_code: string
  token?: string
  print_a5?: string
  print_80x80?: string
  print_52x70?: string
}

export interface CalculateFeeInput {
  provider: 'ghn' | 'manual'
  district_id: number
  ward_code: string
  weight: number
  length?: number
  width?: number
  height?: number
  insurance_value?: number
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
