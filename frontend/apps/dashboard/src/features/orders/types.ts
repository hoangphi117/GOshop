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

export const ORDER_STATUS_MAP: Record<number, { label: string; text: string }> =
  {
    1: { label: 'Pending', text: 'pending' },
    2: { label: 'Confirmed', text: 'confirmed' },
    3: { label: 'Shipped', text: 'shipped' },
    4: { label: 'Delivered', text: 'delivered' },
    5: { label: 'Cancelled', text: 'cancelled' },
    6: { label: 'Returned', text: 'returned' },
  }

export const PAYMENT_STATUS_MAP: Record<
  number,
  { label: string; text: string }
> = {
  1: { label: 'Unpaid', text: 'unpaid' },
  2: { label: 'Paid', text: 'paid' },
  3: { label: 'Partially Refunded', text: 'partially refunded' },
  4: { label: 'Refunded', text: 'refunded' },
}

export const RETURN_STATUS_MAP: Record<
  number,
  {
    label: string
    variant: 'default' | 'secondary' | 'destructive' | 'outline'
  }
> = {
  1: { label: 'Pending Approval', variant: 'secondary' },
  2: { label: 'Approved (Awaiting Return)', variant: 'outline' },
  3: { label: 'Rejected', variant: 'destructive' },
  4: { label: 'Completed', variant: 'default' },
  5: { label: 'Cancelled', variant: 'secondary' },
}

export interface ReturnCustomer {
  user_uuid: string
  fullname: string
  email: string
}

export interface ReturnOrderInfo {
  order_uuid: string
  order_number: string
  total_amount: number
  refunded_amount: number
  payment_method: string
  payment_status: string
  order_status: string
  shipping_name: string
  shipping_phone: string
  shipping_address: string
  created_at: string
}

export interface OrderReturnDetail {
  return_uuid: string
  reason: string
  images: string[]
  video_url?: string | null
  status: number
  status_name: string
  status_text: string
  admin_note?: string | null
  shipping_provider?: string | null
  shipping_code?: string | null
  shipping_note?: string | null
  bank_name?: string | null
  bank_account?: string | null
  bank_account_name?: string | null
  created_at: string
  updated_at: string
  customer?: ReturnCustomer
  order?: ReturnOrderInfo
  items?: OrderItem[]
  events?: TrackingEvent[]
}

export interface AdminUpdateReturnInput {
  status: 2 | 3 | 4
  admin_note?: string
}

export interface AdminReturnRefundInput {
  amount?: number
  reason?: string
  bank_name?: string
  bank_account?: string
  bank_account_name?: string
  bank_txn_ref?: string
  accounting_note?: string
}

export interface AdminReturnSummary {
  return_uuid: string
  order_uuid: string
  order_number: string
  customer_name?: string
  customer_phone?: string
  customer_email?: string
  user_fullname?: string
  user_email?: string
  total_amount: number
  payment_method?: string
  payment_status?: string
  reason: string
  images: string[]
  video_url?: string | null
  status: number
  status_name?: string
  status_text: string
  admin_note?: string | null
  shipping_provider?: string | null
  shipping_code?: string | null
  shipping_note?: string | null
  created_at: string
  updated_at: string
}

export interface AdminGetReturnsQuery {
  page?: number
  limit?: number
  status?: 1 | 2 | 3 | 4 | 5
  search?: string
}

export interface BulkItemError {
  order_uuid: string
  error: string
}

export interface BulkOperationResult {
  total: number
  succeeded: number
  failed: number
  errors?: BulkItemError[]
}

export interface BulkUpdateStatusInput {
  order_uuids: string[]
  status: 1 | 2 | 3 | 4 | 5 | 6
}

export interface BulkCancelOrdersInput {
  order_uuids: string[]
  reason?: string
}

