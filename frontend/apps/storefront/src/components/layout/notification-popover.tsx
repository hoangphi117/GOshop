"use client";

import React, { useState, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  Bell,
  CheckCheck,
  Check,
  Trash2,
  Package,
  Truck,
  CheckCircle2,
  CreditCard,
  RotateCcw,
  MessageSquare,
  Tag,
  Sparkles,
  Info,
  X,
  ExternalLink,
  RefreshCw,
  ShieldAlert,
} from "lucide-react";
import { useAuthStore } from "@/stores/auth-store";
import {
  useNotificationStore,
  NotificationItem,
} from "@/stores/notification-store";
import { useMounted } from "@/hooks/use-mounted";
import { formatDistanceToNow } from "@/lib/format";
import { Badge } from "@/components/ui/badge";

interface NotificationPopoverProps {
  isTransparent?: boolean;
}

export function NotificationPopover({
  isTransparent = false,
}: NotificationPopoverProps) {
  const router = useRouter();
  const mounted = useMounted();
  const { user, accessToken } = useAuthStore();
  const {
    notifications,
    unreadCount,
    isLoading,
    isMarkingAllRead,
    fetchNotifications,
    fetchUnreadCount,
    markAsRead,
    markAllAsRead,
    deleteNotification,
    addNotification,
  } = useNotificationStore();

  const [isOpen, setIsOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<"ALL" | "UNREAD">("ALL");
  const popoverRef = useRef<HTMLDivElement>(null);

  const isAuthenticated = Boolean(user && accessToken);

  // 1. Initial load & periodic polling
  useEffect(() => {
    if (!isAuthenticated) return;

    fetchUnreadCount();
    fetchNotifications();

    const interval = setInterval(() => {
      fetchUnreadCount();
    }, 60000);

    return () => clearInterval(interval);
  }, [isAuthenticated, fetchUnreadCount, fetchNotifications]);

  // 2. WebSocket Real-time Connection
  useEffect(() => {
    if (!isAuthenticated || !accessToken) return;

    let ws: WebSocket | null = null;
    let reconnectTimeout: NodeJS.Timeout | null = null;
    let isCleanedUp = false;

    const connectWS = () => {
      if (isCleanedUp) return;

      try {
        const baseUrl =
          process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
        const wsUrl =
          baseUrl.replace(/^http/, "ws") +
          `/api/v1/ws?token=${encodeURIComponent(accessToken)}`;

        ws = new WebSocket(wsUrl);

        ws.onmessage = (event) => {
          try {
            const msg = JSON.parse(event.data);
            if (msg?.type === "notification" && msg?.data) {
              const isAdminNotif =
                Boolean(msg.data.is_admin) ||
                (typeof msg.data.type === "string" &&
                  msg.data.type.startsWith("admin_"));

              if (!isAdminNotif) {
                useNotificationStore.getState().addNotification(msg.data);
              }
            }
          } catch {
            // ignore non-json messages
          }
        };

        ws.onclose = () => {
          if (!isCleanedUp) {
            reconnectTimeout = setTimeout(connectWS, 5000);
          }
        };

        ws.onerror = () => {
          // Error handler to prevent unhandled socket exceptions
        };
      } catch {
        if (!isCleanedUp) {
          reconnectTimeout = setTimeout(connectWS, 5000);
        }
      }
    };

    connectWS();

    return () => {
      isCleanedUp = true;
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
        reconnectTimeout = null;
      }
      if (ws) {
        const socket = ws;
        ws = null;
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
  }, [isAuthenticated, accessToken]);

  // Click outside to close
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  if (!mounted || !isAuthenticated) return null;

  const filteredNotifications =
    activeTab === "UNREAD"
      ? notifications.filter((n) => !n.is_read)
      : notifications;

  const getNotificationIcon = (type: string) => {
    switch (type) {
      case "ORDER_CREATED":
      case "ORDER_UPDATED":
      case "order":
      case "order_status":
        return <Package className="w-4 h-4 text-[#00ADD8]" />;
      case "ORDER_SHIPPED":
        return <Truck className="w-4 h-4 text-emerald-500" />;
      case "ORDER_DELIVERED":
        return <CheckCircle2 className="w-4 h-4 text-emerald-600" />;
      case "PAYMENT_SUCCESS":
      case "payment":
      case "payment_succeeded":
        return <CreditCard className="w-4 h-4 text-emerald-600" />;
      case "PAYMENT_FAILED":
      case "payment_failed":
        return <ShieldAlert className="w-4 h-4 text-red-500" />;
      case "RETURN_REQUESTED":
      case "RETURN_UPDATED":
        return <RotateCcw className="w-4 h-4 text-amber-500" />;
      case "CHAT_MESSAGE":
        return <MessageSquare className="w-4 h-4 text-purple-500" />;
      case "PROMOTION":
        return <Tag className="w-4 h-4 text-[#F26522]" />;
      default:
        return <Info className="w-4 h-4 text-[#007D9C]" />;
    }
  };

  const handleNotificationClick = (item: NotificationItem) => {
    if (!item.is_read) {
      markAsRead(item.uuid);
    }
    const actionUrl =
      item.data?.action_url ||
      (item.data?.order_uuid ? `/account/orders` : undefined);
    if (actionUrl) {
      setIsOpen(false);
      router.push(actionUrl);
    }
  };

  return (
    <div ref={popoverRef} className="relative">
      {/* Trigger Button */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`relative w-9 h-9 rounded-full flex items-center justify-center transition-all cursor-pointer ${
          isTransparent
            ? "text-white/90 hover:text-white hover:bg-white/15"
            : "text-white hover:text-[#007D9C] hover:bg-[#E6F7FB]"
        }`}
        aria-label={`Notifications (${unreadCount} unread)`}
      >
        <Bell className="w-4 h-4" />
        {unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 min-w-[18px] h-[18px] px-1 bg-[#F26522] text-white text-[10px] font-bold rounded-full flex items-center justify-center shadow-xs animate-in zoom-in-50 duration-200">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </button>

      {/* Popover Dropdown */}
      {isOpen && (
        <div className="absolute right-0 top-full mt-2.5 w-80 sm:w-96 bg-white rounded-md shadow-2xl border border-slate-200/90 overflow-hidden z-50 animate-in fade-in slide-in-from-top-2 duration-200">
          {/* Header */}
          <div className="p-4 border-b border-slate-100 flex items-center justify-between bg-slate-50/70">
            <div className="flex items-center gap-2">
              <h3 className="text-sm font-bold text-slate-900 font-heading">
                Notifications
              </h3>
              {unreadCount > 0 && (
                <Badge variant="cyan" className="text-[10px]">
                  {unreadCount} new
                </Badge>
              )}
            </div>
            <div className="flex items-center gap-1">
              {unreadCount > 0 && (
                <button
                  type="button"
                  onClick={() => markAllAsRead()}
                  disabled={isMarkingAllRead}
                  className="inline-flex items-center gap-1 text-[11px] font-semibold text-[#007D9C] hover:text-[#005F73] px-2 py-1 rounded-lg hover:bg-white transition-colors cursor-pointer"
                  title="Mark all as read"
                >
                  <CheckCheck className="w-3.5 h-3.5" />
                  <span>Mark all read</span>
                </button>
              )}
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                className="p-1 text-slate-400 hover:text-slate-600 rounded-lg hover:bg-white transition-colors cursor-pointer"
                aria-label="Close notifications"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Filter Tabs */}
          <div className="flex border-b border-slate-100 bg-slate-50/40 px-3 pt-2 gap-2 text-xs">
            <button
              type="button"
              onClick={() => setActiveTab("ALL")}
              className={`pb-2 px-2 font-bold transition-colors cursor-pointer border-b-2 ${
                activeTab === "ALL"
                  ? "border-[#007D9C] text-[#007D9C]"
                  : "border-transparent text-slate-500 hover:text-slate-800"
              }`}
            >
              All ({notifications.length})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("UNREAD")}
              className={`pb-2 px-2 font-bold transition-colors cursor-pointer border-b-2 ${
                activeTab === "UNREAD"
                  ? "border-[#007D9C] text-[#007D9C]"
                  : "border-transparent text-slate-500 hover:text-slate-800"
              }`}
            >
              Unread ({unreadCount})
            </button>
          </div>

          {/* Notifications List */}
          <div className="max-h-96 overflow-y-auto divide-y divide-slate-100">
            {isLoading && notifications.length === 0 ? (
              <div className="p-8 text-center text-xs text-slate-400">
                <RefreshCw className="w-5 h-5 mx-auto mb-2 animate-spin text-[#00ADD8]" />
                Loading notifications...
              </div>
            ) : filteredNotifications.length === 0 ? (
              <div className="p-8 text-center">
                <div className="w-12 h-12 rounded-full bg-[#E6F7FB] text-[#007D9C] flex items-center justify-center mx-auto mb-2">
                  <Bell className="w-6 h-6 opacity-60" />
                </div>
                <p className="text-xs font-semibold text-slate-700 font-heading">
                  No notifications yet
                </p>
                <p className="text-[11px] text-slate-400 mt-0.5 font-body">
                  You&apos;ll be notified here about order updates and drop
                  releases.
                </p>
              </div>
            ) : (
              filteredNotifications.map((item) => (
                <div
                  key={item.uuid}
                  onClick={() => handleNotificationClick(item)}
                  className={`group p-3.5 flex items-start gap-3 transition-colors cursor-pointer ${
                    item.is_read
                      ? "hover:bg-slate-50 bg-white"
                      : "bg-[#E6F7FB]/40 hover:bg-[#E6F7FB]/70"
                  }`}
                >
                  <div className="w-8 h-8 rounded-xl bg-white border border-slate-200/80 shadow-2xs flex items-center justify-center shrink-0 mt-0.5">
                    {getNotificationIcon(item.type)}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-start justify-between gap-1">
                      <p
                        className={`text-xs ${
                          item.is_read
                            ? "font-semibold text-slate-800"
                            : "font-bold text-slate-900"
                        } font-heading leading-snug`}
                      >
                        {item.title}
                      </p>
                      {!item.is_read && (
                        <span className="w-2 h-2 rounded-full bg-[#F26522] shrink-0 mt-1" />
                      )}
                    </div>
                    <p className="text-[11px] text-slate-600 font-body mt-0.5 line-clamp-2 leading-relaxed">
                      {item.body}
                    </p>
                    <div className="flex items-center justify-between mt-2 text-[10px] text-slate-400 font-body">
                      <span>{formatDistanceToNow(item.created_at)}</span>
                      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        {!item.is_read && (
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              markAsRead(item.uuid);
                            }}
                            className="p-1 hover:text-[#007D9C] hover:bg-slate-100 rounded"
                            title="Mark as read"
                          >
                            <Check className="w-3 h-3" />
                          </button>
                        )}
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            deleteNotification(item.uuid);
                          }}
                          className="p-1 hover:text-red-600 hover:bg-red-50 rounded"
                          title="Delete notification"
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Footer */}
          <div className="p-3 bg-slate-50 border-t border-slate-100 text-center">
            <Link
              href="/account/notifications"
              onClick={() => setIsOpen(false)}
              className="text-xs font-bold text-[#007D9C] hover:text-[#005F73] transition-colors inline-flex items-center gap-1 font-heading"
            >
              <span>View All Notifications</span>
              <ExternalLink className="w-3 h-3" />
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
