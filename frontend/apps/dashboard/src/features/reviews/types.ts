export type ReviewStatus = 1 | 2 | 3 // 1: Visible, 2: Hidden, 3: Reported

export interface AdminReview {
  uuid: string
  product_uuid: string
  product_name: string
  order_uuid: string
  reviewer_name: string
  reviewer_email: string
  rating: number
  title?: string
  comment?: string
  images: string[]
  video_url?: string
  has_media: boolean
  status: ReviewStatus
  reply?: string
  reply_at?: string
  helpful_count: number
  report_count: number
  created_at: string
  updated_at?: string
}

export interface ReviewReport {
  report_id: number
  user_id: number
  user_name: string
  user_email: string
  reason: 'spam' | 'inappropriate' | 'fake' | 'other' | string
  details?: string
  created_at: string
}

export interface AdminReviewQueryParams {
  page?: number
  limit?: number
  status?: ReviewStatus
  rating?: 1 | 2 | 3 | 4 | 5
  product_uuid?: string
  search?: string
  order_by?: 'created_at' | 'rating'
  sort?: 'asc' | 'desc'
}

export interface ReplyReviewPayload {
  reply: string
}
