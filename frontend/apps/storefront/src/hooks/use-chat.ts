import { useState, useEffect, useRef, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { authFetch, API_BASE_URL, API_KEY } from "@/lib/api";
import { useAuthStore } from "@/stores/auth-store";
import { useChatUIStore } from "@/stores/chat-ui-store";
import {
  playNotificationSound,
  showChatWebNotification,
  formatMessageNotificationSnippet,
} from "@/lib/chat-notifications";

export interface MessageItem {
  uuid: string;
  sender_id: number;
  sender_role: "user" | "admin";
  content: string;
  content_type: "text" | "image" | "video" | "order_ref" | string;
  attachment_url?: string;
  attachment_name?: string;
  attachment_size?: number;
  is_read: boolean;
  created_at: string;
}

export interface ConversationData {
  conversation_id: number;
  uuid: string;
  status: string;
  user_id: number;
  admin_id?: number | null;
  user_email: string;
  user_fullname: string;
  user_avatar?: string;
  unread_count: number;
  last_message_content?: string;
  last_message_at?: string | null;
  created_at: string;
  updated_at: string;
  is_online?: boolean;
  is_shop_online?: boolean;
}

export interface SendMessageParams {
  content: string;
  content_type?: string;
  attachment_url?: string;
  attachment_name?: string;
  attachment_size?: number;
  order_uuid?: string;
  product_uuid?: string;
}

export interface OrderItemRef {
  uuid?: string;
  product_uuid?: string;
  product_name?: string;
  product_slug?: string;
  image_url?: string;
  price?: number;
  quantity?: number;
  subtotal?: number;
}

export interface OrderRef {
  uuid: string;
  order_number: string;
  status: string;
  total_amount: number;
  created_at: string;
  items?: OrderItemRef[];
}

export interface ProductRef {
  uuid: string;
  name: string;
  slug: string;
  price: number;
  stock: number;
  image_url?: string;
  category_name?: string;
  status?: string;
}

// Uploads a file to /api/v1/upload?folder=chat, returns the served URL
export async function uploadChatFile(
  file: File,
  accessToken: string | null,
): Promise<{ url: string; name: string; size: number }> {
  const form = new FormData();
  form.append("file", file);
  form.append("folder", "chat");
  const headers: Record<string, string> = { "X-API-KEY": API_KEY };
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;
  const res = await fetch(`${API_BASE_URL}/api/v1/upload`, {
    method: "POST",
    headers,
    body: form,
    credentials: "include",
  });
  if (!res.ok) throw new Error("Upload failed");
  const json = await res.json();
  const fileUrl: string = json.data?.url || json.data?.file_url || "";
  return { url: fileUrl, name: file.name, size: file.size };
}

// Fetch user orders for the order-attach modal (supports search query)
export function useRecentOrders(enabled: boolean, search?: string) {
  return useQuery({
    queryKey: ["chat", "orders", search || ""],
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set("limit", "20");
      params.set("page", "1");
      if (search && search.trim().length >= 2) {
        params.set("search", search.trim());
      }
      const res = await authFetch(
        `${API_BASE_URL}/api/v1/orders?${params.toString()}`,
      );
      if (!res.ok) return [];
      const json = await res.json();
      const items = (json?.data?.data ||
        json?.data?.items ||
        json?.data ||
        []) as OrderRef[];
      return items;
    },
    enabled,
    staleTime: 1000 * 30,
  });
}

// Fetch products for product consultation picker (supports search query)
export function useChatProducts(enabled: boolean, search?: string) {
  return useQuery({
    queryKey: ["chat", "products", search || ""],
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set("limit", "30");
      params.set("page", "1");
      if (search && search.trim().length >= 1) {
        params.set("search", search.trim());
      }
      const res = await authFetch(
        `${API_BASE_URL}/api/v1/products?${params.toString()}`,
      );
      if (!res.ok) return [];
      const json = await res.json();
      const items = (json?.data?.items ||
        json?.data?.data ||
        json?.data ||
        []) as ProductRef[];
      return items;
    },
    enabled,
    staleTime: 1000 * 60,
  });
}

export function useChat() {
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const accessToken = useAuthStore((state) => state.accessToken);
  const isAuthenticated = Boolean(user && accessToken);

  const [isAdminTyping, setIsAdminTyping] = useState(false);
  const adminTypingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(
    null,
  );

  // 1. Fetch active conversation
  const { data: conversation, isLoading: isConversationLoading } = useQuery({
    queryKey: ["chat", "conversation"],
    queryFn: async () => {
      const res = await authFetch("/api/v1/chat/conversation");
      if (!res.ok) throw new Error("Failed to fetch conversation");
      const json = await res.json();
      return json.data as ConversationData;
    },
    enabled: isAuthenticated,
    staleTime: 1000 * 60 * 5,
  });

  // 2. Fetch conversation messages
  const { data: messages = [], isLoading: isMessagesLoading } = useQuery({
    queryKey: ["chat", "messages", conversation?.uuid],
    queryFn: async () => {
      if (!conversation?.uuid) return [];
      const res = await authFetch("/api/v1/chat/messages?limit=50");
      if (!res.ok) throw new Error("Failed to fetch messages");
      const json = await res.json();
      // Return chronologically ordered messages (API returns desc)
      const items = (json.data?.data ||
        json.data?.items ||
        json.data ||
        []) as MessageItem[];
      return [...items].reverse();
    },
    enabled: isAuthenticated && Boolean(conversation?.uuid),
    staleTime: 1000 * 60,
  });

  // 3. Fetch unread count
  const { data: unreadCount = 0 } = useQuery({
    queryKey: ["chat", "unread-count"],
    queryFn: async () => {
      const res = await authFetch("/api/v1/chat/unread-count");
      if (!res.ok) return 0;
      const json = await res.json();
      return (json.data?.unread_count as number) || 0;
    },
    enabled: isAuthenticated,
    refetchInterval: 30000,
  });

  // 4. Fetch shop online status
  const { data: shopStatus } = useQuery({
    queryKey: ["chat", "shop-status"],
    queryFn: async () => {
      const res = await authFetch("/api/v1/chat/shop-status");
      if (!res.ok) return { is_online: false };
      const json = await res.json();
      return (json.data || { is_online: false }) as { is_online: boolean };
    },
    enabled: isAuthenticated,
    refetchInterval: 30000,
  });

  const [isShopOnline, setIsShopOnline] = useState<boolean>(false);

  useEffect(() => {
    if (shopStatus?.is_online !== undefined) {
      setIsShopOnline(shopStatus.is_online);
    } else if (conversation?.is_shop_online !== undefined) {
      setIsShopOnline(conversation.is_shop_online);
    }
  }, [shopStatus, conversation]);

  // 5. Send message mutation
  const sendMessageMutation = useMutation({
    mutationFn: async (params: SendMessageParams) => {
      const body: Record<string, unknown> = {
        content: params.content,
        content_type: params.content_type || "text",
      };
      if (params.attachment_url) body["attachment_url"] = params.attachment_url;
      if (params.attachment_name)
        body["attachment_name"] = params.attachment_name;
      if (params.attachment_size)
        body["attachment_size"] = params.attachment_size;
      if (params.order_uuid) body["order_uuid"] = params.order_uuid;
      if (params.product_uuid) body["product_uuid"] = params.product_uuid;
      const res = await authFetch("/api/v1/chat/messages", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) throw new Error("Failed to send message");
      const json = await res.json();
      return json.data?.message as MessageItem;
    },
    onSuccess: (newMsg) => {
      if (newMsg && conversation?.uuid) {
        queryClient.setQueryData<MessageItem[]>(
          ["chat", "messages", conversation.uuid],
          (old = []) => {
            if (old.some((m) => m.uuid === newMsg.uuid)) return old;
            return [...old, newMsg];
          },
        );
      }
    },
  });

  // 5. Mark as read mutation
  const markReadMutation = useMutation({
    mutationFn: async () => {
      const res = await authFetch("/api/v1/chat/messages/read", {
        method: "PATCH",
      });
      if (!res.ok) throw new Error("Failed to mark messages as read");
      return res.json();
    },
    onSuccess: () => {
      queryClient.setQueryData(["chat", "unread-count"], 0);
    },
  });

  // 5b. Recall message mutation
  const recallMessageMutation = useMutation({
    mutationFn: async (messageUUID: string) => {
      const res = await authFetch(`/api/v1/chat/messages/${messageUUID}/recall`, {
        method: "PATCH",
      });
      if (!res.ok) {
        const json = await res.json().catch(() => ({}));
        throw new Error(json.message || "Failed to recall message");
      }
      return res.json();
    },
    onSuccess: (_, messageUUID) => {
      if (conversation?.uuid) {
        queryClient.setQueryData<MessageItem[]>(
          ["chat", "messages", conversation.uuid],
          (old = []) =>
            old.map((m) =>
              m.uuid === messageUUID
                ? {
                    ...m,
                    content: "This message was unsent",
                    content_type: "recalled",
                    attachment_url: undefined,
                    attachment_name: undefined,
                    attachment_size: undefined,
                  }
                : m,
            ),
        );
      }
    },
  });

  // 6. Real-time WebSocket connection to the chat room
  useEffect(() => {
    if (!isAuthenticated || !accessToken || !conversation?.uuid) {
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      return;
    }

    let isCleanedUp = false;

    const connect = () => {
      if (isCleanedUp) return;

      try {
        const wsBaseUrl = API_BASE_URL.replace(/^http/, "ws");
        const socketUrl = `${wsBaseUrl}/api/v1/ws?token=${encodeURIComponent(accessToken)}&conversation_uuid=${encodeURIComponent(conversation.uuid)}`;

        const ws = new WebSocket(socketUrl);
        wsRef.current = ws;

        ws.onmessage = (event) => {
          try {
            const payload = JSON.parse(event.data);
            if (payload?.type === "chat.message" && payload?.data) {
              const msgData = payload.data;
              const newMsg: MessageItem = {
                uuid: msgData.message_uuid,
                sender_id: msgData.sender_id,
                sender_role: msgData.sender_role,
                content: msgData.content,
                content_type: msgData.content_type || "text",
                attachment_url: msgData.attachment_url,
                attachment_name: msgData.attachment_name,
                attachment_size: msgData.attachment_size,
                is_read: false,
                created_at: msgData.created_at || new Date().toISOString(),
              };

              queryClient.setQueryData<MessageItem[]>(
                ["chat", "messages", conversation.uuid],
                (old = []) => {
                  if (old.some((m) => m.uuid === newMsg.uuid)) return old;
                  return [...old, newMsg];
                },
              );

              // If message came from admin, increment unread count, play chime, and notify
              if (newMsg.sender_role === "admin") {
                queryClient.invalidateQueries({
                  queryKey: ["chat", "unread-count"],
                });

                const isHidden =
                  typeof document !== "undefined" && document.hidden;
                const isWidgetClosed = !useChatUIStore.getState().isOpen;

                // Play audio chime if user is in background tab or widget is minimized
                if (isHidden || isWidgetClosed) {
                  playNotificationSound();
                }

                // Show desktop web notification if user is currently on another tab
                if (isHidden) {
                  showChatWebNotification({
                    title: "GOshop Support",
                    body: formatMessageNotificationSnippet(
                      newMsg.content_type,
                      newMsg.content,
                      newMsg.attachment_name,
                    ),
                    tag: `storefront-chat-${conversation.uuid}`,
                    onClick: () => {
                      useChatUIStore.getState().openChat();
                    },
                  });
                }
              }
            } else if (
              payload?.type === "chat.message_recalled" &&
              payload?.data
            ) {
              const { message_uuid } = payload.data;
              queryClient.setQueryData<MessageItem[]>(
                ["chat", "messages", conversation.uuid],
                (old = []) =>
                  old.map((m) =>
                    m.uuid === message_uuid
                      ? {
                          ...m,
                          content: "This message was unsent",
                          content_type: "recalled",
                          attachment_url: undefined,
                          attachment_name: undefined,
                          attachment_size: undefined,
                        }
                      : m,
                  ),
              );
            } else if (payload?.type === "chat.read") {
              // Mark all messages as read
              queryClient.setQueryData<MessageItem[]>(
                ["chat", "messages", conversation.uuid],
                (old = []) => old.map((m) => ({ ...m, is_read: true })),
              );
            } else if (payload?.type === "chat.typing" && payload?.data) {
              if (payload.data.sender_role === "admin") {
                if (adminTypingTimeoutRef.current) {
                  clearTimeout(adminTypingTimeoutRef.current);
                }

                if (payload.data.is_typing) {
                  setIsAdminTyping(true);
                  // Auto clear typing state after 3.5s in case stop event is lost
                  adminTypingTimeoutRef.current = setTimeout(() => {
                    setIsAdminTyping(false);
                  }, 3500);
                } else {
                  setIsAdminTyping(false);
                }
              }
            } else if (payload?.type === "chat.presence" && payload?.data) {
              if (payload.data.role === "admin") {
                setIsShopOnline(Boolean(payload.data.is_online));
              }
            }
          } catch {
            // Safe ignore non-JSON frame
          }
        };

        ws.onclose = () => {
          if (isCleanedUp) return;
          reconnectTimeoutRef.current = setTimeout(connect, 3000);
        };
      } catch {
        // Fallback on connection error
      }
    };

    connect();

    return () => {
      isCleanedUp = true;
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (adminTypingTimeoutRef.current) {
        clearTimeout(adminTypingTimeoutRef.current);
      }
      if (wsRef.current) {
        const socket = wsRef.current;
        wsRef.current = null;
        socket.onclose = null;
        socket.close(1000, "Normal closure");
      }
    };
  }, [isAuthenticated, accessToken, conversation?.uuid, queryClient]);

  // Send typing status to server via WS and HTTP fallback
  const sendTyping = useCallback(
    (isTyping: boolean) => {
      if (!conversation?.uuid || !isAuthenticated) return;

      // Try WebSocket first
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        try {
          wsRef.current.send(
            JSON.stringify({
              type: "chat.typing",
              data: {
                conversation_uuid: conversation.uuid,
                is_typing: isTyping,
              },
            }),
          );
          return;
        } catch {
          // Fall through to HTTP
        }
      }

      // HTTP fallback
      authFetch("/api/v1/chat/typing", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ is_typing: isTyping }),
      }).catch(() => {});
    },
    [conversation?.uuid, isAuthenticated],
  );

  return {
    isAuthenticated,
    accessToken,
    conversation,
    messages,
    unreadCount,
    isShopOnline,
    isAdminTyping,
    sendTyping,
    isLoading: isConversationLoading || isMessagesLoading,
    sendMessage: sendMessageMutation.mutateAsync,
    isSending: sendMessageMutation.isPending,
    recallMessage: recallMessageMutation.mutateAsync,
    isRecalling: recallMessageMutation.isPending,
    markAsRead: markReadMutation.mutate,
  };
}
