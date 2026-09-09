export interface Coupon {
  uuid?: string
  code?: string
  name?: string
  description?: string
  type?: number
  type_label?: string
  discount_value?: number
  min_order_amount?: number
  max_discount_amount?: number
  usage_limit?: number
  used_count?: number
  user_usage_limit?: number
  start_date?: string
  end_date?: string
  is_active?: boolean
  is_expired?: boolean
}

export interface CouponFormData {
  code: string
  name: string
  description?: string
  type: 1 | 2
  discount_value: number
  min_order_amount?: number
  max_discount_amount?: number
  usage_limit?: number
  user_usage_limit?: number
  start_date: string
  end_date: string
  is_active: boolean
}
