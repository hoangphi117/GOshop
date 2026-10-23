export interface StockMovement {
  movement_uuid: string
  product_uuid: string
  change_type: string
  quantity_change: number
  stock_before: number
  stock_after: number
  reference_id?: string
  reference_type?: string
  actor_uuid?: string
  actor_name?: string
  note?: string
  created_at: string
}

export interface StockMovementQueryParams {
  page?: number
  limit?: number
  product_uuid?: string
  change_type?: string
  from_date?: string
  to_date?: string
  search?: string
}

export interface AdjustStockPayload {
  product_uuid: string
  quantity_change: number
  change_type: 'RESTOCK' | 'MANUAL_ADJUSTMENT' | 'DAMAGED_DISCARD' | 'INITIAL_STOCK'
  note?: string
  /** Idempotency reference (INITIAL_STOCK only): same reference never applies twice. */
  reference_id?: string
  /** Absolute stock target; overrides quantity_change. Retry-safe for admin edits. */
  target_quantity?: number
}
