import { useRef, useCallback, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { fetchWithAuth } from '@/lib/api'
import { useChatTypingStore } from '../stores/chat-typing-store'
import type {
  ConversationItem,
  ChatMessage,
  ListConversationsParams,
} from '../types'
import {
  playNotificationSound,
  showChatWebNotification,
  formatAdminNotificationSnippet,
} from '../lib/chat-notifications'

const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

export interface SendMessagePayload {
  conversationUUID: string
  content: string
  content_type?: string
  attachment_url?: string
  attachment_name?: string
  attachment_size?: number
  order_uuid?: string
  product_uuid?: string
}

export async function uploadChatFile(
  file: File,
  getToken: () => string | null
): Promise<{ url: string; name: string; size: number }> {
  const form = new FormData()
  form.append('file', file)
  form.append('folder', 'chat')
  const token = getToken()
  const apiKey = import.meta.env.VITE_API_KEY || 'abcd-1234-1111'
  const headers: Record<string, string> = { 'X-API-KEY': apiKey }
  if (token) headers['Authorization'] = `Bearer ${token}`
  const res = await fetch(`${API_BASE_URL}/api/v1/upload`, {
    method: 'POST',
    headers,
    body: form,
    credentials: 'include',
  })
  if (!res.ok) throw new Error('Upload failed')
  const json = await res.json()
  const fileUrl: string = json.data?.url || json.data?.file_url || ''
  return { url: fileUrl, name: file.name, size: file.size }
}

export const adminChatKeys = {
  all: ['admin-chats'] as const,
  conversations: (params?: ListConversationsParams) =>
    ['admin-chats', 'conversations', params] as const,
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

      const res = await fetchWithAuth(
        `/api/v1/admin/chat/conversations?${query.toString()}`
      )
      if (!res.ok) throw new Error('Failed to fetch conversations')
      const json = await res.json()
      return (json.data?.data ||
        json.data?.items ||
        json.data ||
        []) as ConversationItem[]
    },
    refetchInterval: 5000,
  })
}

export function useAdminConversationDetails(conversationUUID?: string) {
  return useQuery({
    queryKey: adminChatKeys.conversation(conversationUUID),
    queryFn: async () => {
      if (!conversationUUID) return null
      const res = await fetchWithAuth(
        `/api/v1/admin/chat/conversations/${conversationUUID}`
      )
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
      const res = await fetchWithAuth(
        `/api/v1/admin/chat/conversations/${conversationUUID}/messages?limit=100`
      )
      if (!res.ok) throw new Error('Failed to fetch messages')
      const json = await res.json()
      const items = (json.data?.data ||
        json.data?.items ||
        json.data ||
        []) as ChatMessage[]
      return [...items].reverse()
    },
    enabled: Boolean(conversationUUID),
    refetchInterval: 3000,
  })
}

export function useAdminSendMessage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (payload: SendMessagePayload) => {
      const {
        conversationUUID,
        content,
        content_type,
        attachment_url,
        attachment_name,
        attachment_size,
        order_uuid,
        product_uuid,
      } = payload
      const body: Record<string, unknown> = {
        content,
        content_type: content_type || 'text',
      }
      if (attachment_url) body['attachment_url'] = attachment_url
      if (attachment_name) body['attachment_name'] = attachment_name
      if (attachment_size) body['attachment_size'] = attachment_size
      if (order_uuid) body['order_uuid'] = order_uuid
      if (product_uuid) body['product_uuid'] = product_uuid
      const res = await fetchWithAuth(
        `/api/v1/admin/chat/conversations/${conversationUUID}/messages`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        }
      )
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
      queryClient.invalidateQueries({
        queryKey: ['admin-chats', 'conversations'],
      })
    },
  })
}

export function useAdminMarkRead() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (conversationUUID: string) => {
      const res = await fetchWithAuth(
        `/api/v1/admin/chat/conversations/${conversationUUID}/read`,
        {
          method: 'PATCH',
        }
      )
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
      queryClient.invalidateQueries({
        queryKey: ['admin-chats', 'conversations'],
      })
    },
  })
}

export function useAdminCloseConversation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (conversationUUID: string) => {
      const res = await fetchWithAuth(
        `/api/v1/admin/chat/conversations/${conversationUUID}/close`,
        {
          method: 'PATCH',
        }
      )
      if (!res.ok) throw new Error('Failed to close conversation')
      return res.json()
    },
    onSuccess: (_, conversationUUID) => {
      queryClient.invalidateQueries({
        queryKey: adminChatKeys.conversation(conversationUUID),
      })
      queryClient.invalidateQueries({
        queryKey: ['admin-chats', 'conversations'],
      })
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

/**
 * Dedicated real-time WebSocket connection hook for the active Admin Chat conversation.
 * Provides instant 0ms message arrival, immediate read trigger, and WebSocket typing dispatch.
 */
export function useAdminChatRoom(conversationUUID?: string | null) {
  const queryClient = useQueryClient()
  const accessToken = useAuthStore((state) => state.auth.accessToken)
  const isAuthenticated = Boolean(useAuthStore((state) => state.auth.user))

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (!isAuthenticated || !accessToken || !conversationUUID) {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      return
    }

    let isCleanedUp = false

    const connect = () => {
      if (isCleanedUp) return

      try {
        const rawBaseUrl =
          import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'
        const wsBaseUrl = rawBaseUrl.replace(/^http/, 'ws')
        const socketUrl = `${wsBaseUrl}/api/v1/ws?token=${encodeURIComponent(
          accessToken
        )}&conversation_uuid=${encodeURIComponent(conversationUUID)}`

        const ws = new WebSocket(socketUrl)
        wsRef.current = ws

        ws.onmessage = (event) => {
          try {
            const payload = JSON.parse(event.data)
            if (payload?.type === 'chat.message' && payload?.data) {
              const msgData = payload.data
              const newMsg: ChatMessage = {
                uuid: msgData.message_uuid,
                sender_id: msgData.sender_id,
                sender_role: msgData.sender_role,
                content: msgData.content,
                content_type: msgData.content_type || 'text',
                attachment_url: msgData.attachment_url,
                attachment_name: msgData.attachment_name,
                attachment_size: msgData.attachment_size,
                is_read: false,
                created_at: msgData.created_at || new Date().toISOString(),
              }

              const targetConvUUID =
                msgData.conversation_uuid || conversationUUID

              // 1. Immediately insert message into query cache with zero delay
              queryClient.setQueryData<ChatMessage[]>(
                adminChatKeys.messages(targetConvUUID),
                (old = []) => {
                  if (old.some((m) => m.uuid === newMsg.uuid)) return old
                  return [...old, newMsg]
                }
              )

              // 2. Immediately update sidebar conversation preview
              queryClient.setQueriesData<ConversationItem[]>(
                { queryKey: ['admin-chats', 'conversations'] },
                (old) => {
                  if (!old || !Array.isArray(old)) return old
                  return old.map((c) => {
                    if (c.uuid === targetConvUUID) {
                      return {
                        ...c,
                        last_message_content: msgData.content,
                        last_message_at:
                          msgData.created_at || new Date().toISOString(),
                        unread_count:
                          msgData.sender_role === 'user' &&
                          targetConvUUID !== conversationUUID
                            ? (c.unread_count || 0) + 1
                            : 0,
                      }
                    }
                    return c
                  })
                }
              )

              // 3. Mark read immediately if admin is currently viewing this active conversation
              if (
                msgData.sender_role === 'user' &&
                targetConvUUID === conversationUUID &&
                document.visibilityState === 'visible'
              ) {
                fetchWithAuth(
                  `/api/v1/admin/chat/conversations/${conversationUUID}/read`,
                  {
                    method: 'PATCH',
                  }
                ).catch(() => {})
              }

              // 4. Audio chime & Web notification for incoming customer messages
              if (msgData.sender_role === 'user') {
                const isHidden =
                  typeof document !== 'undefined' && document.hidden
                const isDifferentConv = targetConvUUID !== conversationUUID

                // Play audio chime if admin is in background tab or viewing a different conversation
                if (isHidden || isDifferentConv) {
                  playNotificationSound()
                }

                // Show desktop web notification if admin tab is in background
                if (isHidden) {
                  const convs = queryClient.getQueryData<ConversationItem[]>([
                    'admin-chats',
                    'conversations',
                  ])
                  const conv = convs?.find((c) => c.uuid === targetConvUUID)
                  const senderName =
                    conv?.user_fullname || conv?.user_email || 'Customer'

                  showChatWebNotification({
                    title: `New message from ${senderName}`,
                    body: formatAdminNotificationSnippet(
                      msgData.content_type || 'text',
                      msgData.content,
                      msgData.attachment_name
                    ),
                    tag: `admin-chat-${targetConvUUID}`,
                    onClick: () => {
                      window.focus()
                    },
                  })
                }
              }
            } else if (payload?.type === 'chat.read') {
              const readConvUUID =
                payload?.data?.conversation_uuid || conversationUUID
              queryClient.setQueryData<ChatMessage[]>(
                adminChatKeys.messages(readConvUUID),
                (old = []) => old.map((m) => ({ ...m, is_read: true }))
              )
            } else if (payload?.type === 'chat.typing' && payload?.data) {
              const { conversation_uuid, sender_role, is_typing } = payload.data
              if (sender_role === 'user' && conversation_uuid) {
                useChatTypingStore
                  .getState()
                  .setTyping(conversation_uuid, Boolean(is_typing))
              }
            } else if (payload?.type === 'chat.presence' && payload?.data) {
              const { conversation_uuid, user_id, role, is_online } =
                payload.data
              if (role === 'user') {
                queryClient.setQueriesData<ConversationItem[]>(
                  { queryKey: ['admin-chats', 'conversations'] },
                  (old) => {
                    if (!old || !Array.isArray(old)) return old
                    return old.map((c) => {
                      if (
                        (conversation_uuid && c.uuid === conversation_uuid) ||
                        (user_id && c.user_id === user_id)
                      ) {
                        return { ...c, is_online: Boolean(is_online) }
                      }
                      return c
                    })
                  }
                )

                if (conversationUUID) {
                  queryClient.setQueryData<ConversationItem>(
                    adminChatKeys.conversation(conversationUUID),
                    (old) => {
                      if (!old) return old
                      if (
                        (conversation_uuid && old.uuid === conversation_uuid) ||
                        (user_id && old.user_id === user_id)
                      ) {
                        return { ...old, is_online: Boolean(is_online) }
                      }
                      return old
                    }
                  )
                }
              }
            }
          } catch {
            // Safe ignore non-JSON frame
          }
        }

        ws.onclose = () => {
          if (isCleanedUp) return
          reconnectTimeoutRef.current = setTimeout(connect, 3000)
        }
      } catch {
        // Fallback on connection error
      }
    }

    connect()

    return () => {
      isCleanedUp = true
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (wsRef.current) {
        const socket = wsRef.current
        wsRef.current = null
        socket.onclose = null
        socket.close(1000, 'Normal closure')
      }
    }
  }, [isAuthenticated, accessToken, conversationUUID, queryClient])

  // Send typing status to server via WebSocket first, then fallback to HTTP
  const sendTyping = useCallback(
    (isTyping: boolean) => {
      if (!conversationUUID || !isAuthenticated) return

      // Try WebSocket first with 0ms overhead
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        try {
          wsRef.current.send(
            JSON.stringify({
              type: 'chat.typing',
              data: {
                conversation_uuid: conversationUUID,
                is_typing: isTyping,
              },
            })
          )
          return
        } catch {
          // Fall through to HTTP
        }
      }

      // HTTP fallback
      fetchWithAuth(
        `/api/v1/admin/chat/conversations/${conversationUUID}/typing`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ is_typing: isTyping }),
        }
      ).catch(() => {})
    },
    [conversationUUID, isAuthenticated]
  )

  return { sendTyping }
}
