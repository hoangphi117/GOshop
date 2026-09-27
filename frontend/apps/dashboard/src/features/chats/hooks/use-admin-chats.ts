import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { fetchWithAuth } from '@/lib/api'
import type { ConversationItem, ChatMessage, ListConversationsParams } from '../types'

export const adminChatKeys = {
  all: ['admin-chats'] as const,
  conversations: (params?: ListConversationsParams) => ['admin-chats', 'conversations', params] as const,
  conversation: (id?: string) => ['admin-chats', 'conversation', id] as const,
  messages: (id?: string) => ['admin-chats', 'messages', id] as const,
}

export function useAdminConversations(params: ListConversationsParams = {}) {
  return useQuery({
    queryKey: adminChatKeys.conversations(params),
    queryFn: async () => {
      const query = new URLSearchParams()
      if (params.status) query.set('status', params.status)
      if (params.search) query.set('search', params.search)
      if (params.page) query.set('page', String(params.page))
      if (params.limit) query.set('limit', String(params.limit || 50))

      const res = await fetchWithAuth(`/api/v1/admin/chat/conversations?${query.toString()}`)
      if (!res.ok) throw new Error('Failed to fetch conversations')
      const json = await res.json()
      return (json.data?.data || json.data?.items || json.data || []) as ConversationItem[]
    },
    refetchInterval: 30000,
  })
}

export function useAdminConversationDetails(conversationUUID?: string) {
  return useQuery({
    queryKey: adminChatKeys.conversation(conversationUUID),
    queryFn: async () => {
      if (!conversationUUID) return null
      const res = await fetchWithAuth(`/api/v1/admin/chat/conversations/${conversationUUID}`)
      if (!res.ok) throw new Error('Failed to fetch conversation details')
      const json = await res.json()
      return json.data as ConversationItem
    },
    enabled: Boolean(conversationUUID),
  })
}

export function useAdminMessages(conversationUUID?: string) {
  return useQuery({
    queryKey: adminChatKeys.messages(conversationUUID),
    queryFn: async () => {
      if (!conversationUUID) return []
      const res = await fetchWithAuth(`/api/v1/admin/chat/conversations/${conversationUUID}/messages?limit=100`)
      if (!res.ok) throw new Error('Failed to fetch messages')
      const json = await res.json()
      const items = (json.data?.data || json.data?.items || json.data || []) as ChatMessage[]
      return [...items].reverse()
    },
    enabled: Boolean(conversationUUID),
  })
}

export function useAdminSendMessage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({
      conversationUUID,
      content,
    }: {
      conversationUUID: string
      content: string
    }) => {
      const res = await fetchWithAuth(`/api/v1/admin/chat/conversations/${conversationUUID}/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          content,
          content_type: 'text',
        }),
      })
      if (!res.ok) throw new Error('Failed to send admin message')
      const json = await res.json()
      return json.data?.message as ChatMessage
    },
    onSuccess: (newMsg, variables) => {
      if (newMsg) {
        queryClient.setQueryData<ChatMessage[]>(
          adminChatKeys.messages(variables.conversationUUID),
          (old = []) => {
            if (old.some((m) => m.uuid === newMsg.uuid)) return old
            return [...old, newMsg]
          }
        )
      }
      queryClient.invalidateQueries({ queryKey: ['admin-chats', 'conversations'] })
    },
  })
}

export function useAdminMarkRead() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (conversationUUID: string) => {
      const res = await fetchWithAuth(`/api/v1/admin/chat/conversations/${conversationUUID}/read`, {
        method: 'PATCH',
      })
      if (!res.ok) throw new Error('Failed to mark conversation as read')
      return res.json()
    },
    onSuccess: (_, conversationUUID) => {
      queryClient.setQueryData<ConversationItem[]>(
        ['admin-chats', 'conversations'],
        (old = []) =>
          old.map((c) =>
            c.uuid === conversationUUID ? { ...c, unread_count: 0 } : c
          )
      )
      queryClient.invalidateQueries({ queryKey: ['admin-chats', 'conversations'] })
    },
  })
}

export function useAdminCloseConversation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (conversationUUID: string) => {
      const res = await fetchWithAuth(`/api/v1/admin/chat/conversations/${conversationUUID}/close`, {
        method: 'PATCH',
      })
      if (!res.ok) throw new Error('Failed to close conversation')
      return res.json()
    },
    onSuccess: (_, conversationUUID) => {
      queryClient.invalidateQueries({ queryKey: adminChatKeys.conversation(conversationUUID) })
      queryClient.invalidateQueries({ queryKey: ['admin-chats', 'conversations'] })
    },
  })
}

export function useAdminChatUnreadCount() {
  return useQuery({
    queryKey: ['admin-chats', 'unread-count'],
    queryFn: async () => {
      const res = await fetchWithAuth('/api/v1/admin/chat/unread-count')
      if (!res.ok) return 0
      const json = await res.json()
      return (json.data?.unread_count as number) || 0
    },
    refetchInterval: 30000,
  })
}

export function useAdminSendTyping() {
  return useMutation({
    mutationFn: async ({
      conversationUUID,
      isTyping,
    }: {
      conversationUUID: string
      isTyping: boolean
    }) => {
      const res = await fetchWithAuth(
        `/api/v1/admin/chat/conversations/${conversationUUID}/typing`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ is_typing: isTyping }),
        }
      )
      if (!res.ok) throw new Error('Failed to send typing status')
      return res.json()
    },
  })
}
