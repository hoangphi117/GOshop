import { useState, useEffect, useRef, useCallback } from "react";
import {
  useQuery,
  useMutation,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { directUpload } from "@goshop/api-client";
import { authFetch, API_BASE_URL, API_KEY } from "@/lib/api";
import { useAuthStore } from "@/stores/auth-store";
import { useChatUIStore, resetChatUIForNewSession } from "@/stores/chat-ui-store";
import { useAssistant } from "@/hooks/use-assistant";
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

// Direct-to-storage upload: presign → PUT to bucket → confirm, returns the verified URL.
export async function uploadChatFile(
  file: File,
  accessToken: string | null,
): Promise<{ url: string; name: string; size: number }> {
  const headers: Record<string, string> = { "X-API-KEY": API_KEY };
  if (accessToken) headers["Authorization"] = `Bearer ${accessToken}`;
  const result = await directUpload(file, "chat", (path, init) =>
    fetch(`${API_BASE_URL}${path}`, {
      ...init,
      headers: { ...headers, ...(init?.headers as Record<string, string>) },
      credentials: "include",
    }),
  );
  return { url: result.url, name: result.name, size: result.size };
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

/**
 * Query keys are namespaced by account first. The old flat keys leaked one
 * customer's conversation and messages to whoever signed in next on the same
 * device, because the cached entry never expired before it was read again.
 */
export const chatKeys = {
  all: ["chat"] as const,
  conversation: (account: string) => ["chat", account, "conversation"] as const,
  messages: (account: string, conversationUUID?: string) =>
    ["chat", account, "messages", conversationUUID] as const,
  unreadCount: (account: string) => ["chat", account, "unread-count"] as const,
  shopStatus: (account: string) => ["chat", account, "shop-status"] as const,
};

/**
 * Clears chat state that outlives a logout.
 *
 * Three stores keep customer chat data on the device: the TanStack cache
 * (`chat/conversation`, `chat/messages`, `chat/unread-count` — scoped by user,
 * but the query keys are not), the assistant transcript ("goshop-assistant"),
 * and the widget's queued order/product. Signing out only cleared the auth
 * store, so the next account saw the previous customer's history in both the
 * AI and the human tab.
 *
 * `nextAccount` is the identity about to sign in. The chat queries mount while
 * signed out, so they have already cached an "anonymous" conversation: without
 * evicting that entry the widget under the new account keeps the stale answer
 * for its whole `staleTime` and only a reload refetches.
 */
export function resetChatSession(queryClient: QueryClient, nextAccount?: string) {
  queryClient.removeQueries({ queryKey: ["chat"] });
  queryClient.removeQueries({ queryKey: ["assistant"] });
  useAssistant.getState().reset();
  resetChatUIForNewSession();
  if (nextAccount) {
    queryClient.removeQueries({
      queryKey: chatKeys.conversation(nextAccount),
      exact: true,
    });
  }
}

export const useChat = () => {
  const queryClient = useQueryClient();
  const user = useAuthStore((state) => state.user);
  const accessToken = useAuthStore((state) => state.accessToken);
  const isAuthenticated = Boolean(user && accessToken);

  // Every chat query is scoped by account. Without this the cache entry
  // "chat/conversation" written for account A stays fresh for `staleTime` and
  // is returned verbatim to account B, which never asks the server.
  const accountKey = user?.uuid || user?.email || "anonymous";

  // Watch for identity changes here rather than in an effect: the widget mounts
  // while signed out, the login page re-renders before this hook does, and the
  // stale entry has to be gone before the first query of the new account runs.
  const [observedAccount, setObservedAccount] = useState(accountKey);
  if (observedAccount !== accountKey) {
    setObservedAccount(accountKey);
    resetChatSession(queryClient, accountKey);
  }

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
    queryKey: chatKeys.conversation(accountKey),
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
    queryKey: chatKeys.messages(accountKey, conversation?.uuid),
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
    queryKey: chatKeys.unreadCount(accountKey),
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
    queryKey: chatKeys.shopStatus(accountKey),
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
          chatKeys.messages(accountKey, conversation.uuid),
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
      queryClient.setQueryData(chatKeys.unreadCount(accountKey), 0);
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
          chatKeys.messages(accountKey, conversation.uuid),
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
  const conversationUUID = conversation?.uuid;
  const conversationOwnerID = conversation?.user_id;
  useEffect(() => {
    if (!isAuthenticated || !accessToken || !conversationUUID) {
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
        const socketUrl = `${wsBaseUrl}/api/v1/ws?token=${encodeURIComponent(accessToken)}&conversation_uuid=${encodeURIComponent(conversationUUID)}`;

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
                chatKeys.messages(accountKey, conversationUUID),
                (old = []) => {
                  if (old.some((m) => m.uuid === newMsg.uuid)) return old;
                  return [...old, newMsg];
                },
              );

              // If message came from admin, check if widget is open to mark as read or notify
              if (newMsg.sender_role === "admin") {
                const isHidden =
                  typeof document !== "undefined" && document.hidden;
                const isWidgetOpen = useChatUIStore.getState().isOpen;

                if (isWidgetOpen && !isHidden) {
                  // User is actively looking at the chat, mark read immediately
                  authFetch("/api/v1/chat/messages/read", {
                    method: "PATCH",
                  }).catch(() => {});
                } else {
                  queryClient.invalidateQueries({
                    queryKey: chatKeys.unreadCount(accountKey),
                  });

                  // Play audio chime if user is in background tab or widget is minimized
                  playNotificationSound();

                  // Show desktop web notification if user is currently on another tab
                  if (isHidden) {
                    showChatWebNotification({
                      title: "GOshop Support",
                      body: formatMessageNotificationSnippet(
                        newMsg.content_type,
                        newMsg.content,
                        newMsg.attachment_name,
                      ),
                      tag: `storefront-chat-${conversationUUID}`,
                      onClick: () => {
                        useChatUIStore.getState().openChatWithHuman();
                      },
                    });
                  }
                }
              }
            } else if (
              payload?.type === "chat.message_recalled" &&
              payload?.data
            ) {
              const { message_uuid } = payload.data;
              queryClient.setQueryData<MessageItem[]>(
                chatKeys.messages(accountKey, conversationUUID),
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
                chatKeys.messages(accountKey, conversationUUID),
                (old = []) => old.map((m) => ({ ...m, is_read: true })),
              );
            } else if (payload?.type === "chat.typing" && payload?.data) {
              const typingUserId = payload.data.user_id;
              const typingRole = payload.data.sender_role;
              const isTyping = Boolean(payload.data.is_typing);

              // Never display typing indicator for own user events
              if (conversationOwnerID && typingUserId === conversationOwnerID) {
                return;
              }

              if (typingRole === "admin") {
                if (adminTypingTimeoutRef.current) {
                  clearTimeout(adminTypingTimeoutRef.current);
                }

                if (isTyping) {
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
        socket.onopen = null;
        socket.onmessage = null;
        socket.onerror = null;
        socket.onclose = null;

        if (socket.readyState === WebSocket.OPEN) {
          socket.close(1000, "Normal closure");
        } else if (socket.readyState === WebSocket.CONNECTING) {
          socket.addEventListener(
            "open",
            () => {
              socket.close(1000, "Normal closure");
            },
            { once: true },
          );
        }
      }
    };
  }, [
    isAuthenticated,
    accessToken,
    accountKey,
    conversationUUID,
    conversationOwnerID,
    queryClient,
  ]);

  // Send typing status to server via WS and HTTP fallback
  const sendTyping = useCallback(
    (isTyping: boolean) => {
      if (!conversationUUID || !isAuthenticated) return;

      // Try WebSocket first
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        try {
          wsRef.current.send(
            JSON.stringify({
              type: "chat.typing",
              data: {
                conversation_uuid: conversationUUID,
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
    [conversationUUID, isAuthenticated],
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
