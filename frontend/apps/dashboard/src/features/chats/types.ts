export interface ChatMessage {
  uuid: string
  sender_id: number
  sender_role: 'user' | 'admin'
  content: string
  content_type: 'text' | 'image' | 'video' | 'order_ref' | string
  attachment_url?: string
  attachment_name?: string
  attachment_size?: number
  is_read: boolean
  created_at: string
}

export interface ConversationItem {
  conversation_id: number
  uuid: string
  status: 'open' | 'closed' | 'pending'
  user_id: number
  admin_id?: number | null
  user_email: string
  user_fullname: string
  user_avatar?: string
  unread_count: number
  last_message_content?: string
  last_message_at?: string | null
  created_at: string
  updated_at: string
}

export interface ListConversationsParams {
  status?: string
  search?: string
  page?: number
  limit?: number
}
