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

    // Background poll every 60 seconds
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
              addNotification(msg.data);
            }
          } catch {
            // ignore non-json messages
          }
        };

        ws.onerror = () => {
          // close and retry
          if (ws) ws.close();
        };

        ws.onclose = () => {
          if (!isCleanedUp) {
            reconnectTimeout = setTimeout(connectWS, 10000);
          }
        };
      } catch {
        // ignore websocket failure
      }
    };

    connectWS();

    return () => {
      isCleanedUp = true;
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
      if (ws) ws.close();
    };
  }, [isAuthenticated, accessToken, addNotification]);

  // 3. Close popover on Click Outside or ESC
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        popoverRef.current &&
        !popoverRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      document.addEventListener("keydown", handleKeyDown);
    }

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  if (!mounted || !isAuthenticated) return null;

  const handleToggle = () => {
    const nextState = !isOpen;
    setIsOpen(nextState);
    if (nextState) {
      fetchNotifications();
    }
  };

  const handleItemClick = (item: NotificationItem) => {
    if (!item.is_read) {
      markAsRead(item.uuid);
    }
    setIsOpen(false);

    // Navigate to target if applicable
    if (item.data?.url && typeof item.data.url === "string") {
      router.push(item.data.url);
      return;
    }

    if (
      item.type.includes("security") ||
      item.type.includes("session") ||
      item.type.includes("device") ||
      item.type.includes("auth")
    ) {
      router.push(item.data?.url || "/account/security");
      return;
    }

    if (
      item.type.includes("order") ||
      item.type.includes("shipping") ||
      item.type.includes("payment") ||
      item.data?.order_uuid ||
      item.data?.order_number
    ) {
      const orderRef = item.data?.order_number || item.data?.order_uuid;
      if (orderRef) {
        router.push(
          `/account/orders?orderId=${encodeURIComponent(String(orderRef))}`,
        );
      } else {
        router.push("/account/orders");
      }
      return;
    }

    if (item.type.includes("review")) {
      const orderRef = item.data?.order_number || item.data?.order_uuid;
      if (orderRef) {
        router.push(
          `/account/orders?orderId=${encodeURIComponent(String(orderRef))}`,
        );
      } else {
        router.push("/account/orders");
      }
      return;
    }
  };

  const formatRelativeTime = (isoString: string) => {
    try {
      const now = new Date().getTime();
      const date = new Date(isoString).getTime();
      const diffSec = Math.floor((now - date) / 1000);

      if (diffSec < 60) return "Just now";
      if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`;
      if (diffSec < 86400) return `${Math.floor(diffSec / 3600)}h ago`;
      if (diffSec < 604800) return `${Math.floor(diffSec / 86400)}d ago`;

      return new Date(isoString).toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
      });
    } catch {
      return "";
    }
  };

  const getNotificationVisual = (type: string) => {
    const t = (type || "").toLowerCase();
    if (
      t.includes("security") ||
      t.includes("alert") ||
      t.includes("session") ||
      t.includes("device") ||
      t.includes("auth")
    ) {
      return {
        icon: <ShieldAlert className="w-4 h-4 text-rose-600" />,
        bg: "bg-rose-50 border-rose-100",
      };
    }
    if (
      t.includes("shipped") ||
      t.includes("shipping") ||
      t.includes("delivery")
    ) {
      return {
        icon: <Truck className="w-4 h-4 text-indigo-600" />,
        bg: "bg-indigo-50 border-indigo-100",
      };
    }
    if (
      t.includes("delivered") ||
      t.includes("received") ||
      t.includes("success")
    ) {
      return {
        icon: <CheckCircle2 className="w-4 h-4 text-emerald-600" />,
        bg: "bg-emerald-50 border-emerald-100",
      };
    }
    if (t.includes("paid") || t.includes("payment") || t.includes("refund")) {
      return {
        icon: <CreditCard className="w-4 h-4 text-emerald-600" />,
        bg: "bg-emerald-50 border-emerald-100",
      };
    }
    if (t.includes("return")) {
      return {
        icon: <RotateCcw className="w-4 h-4 text-amber-600" />,
        bg: "bg-amber-50 border-amber-100",
      };
    }
    if (t.includes("review")) {
      return {
        icon: <MessageSquare className="w-4 h-4 text-cyan-600" />,
        bg: "bg-cyan-50 border-cyan-100",
      };
    }
    if (t.includes("promo") || t.includes("coupon") || t.includes("discount")) {
      return {
        icon: <Tag className="w-4 h-4 text-violet-600" />,
        bg: "bg-violet-50 border-violet-100",
      };
    }
    if (t.includes("order")) {
      return {
        icon: <Package className="w-4 h-4 text-blue-600" />,
        bg: "bg-blue-50 border-blue-100",
      };
    }
    return {
      icon: <Bell className="w-4 h-4 text-[#2196F3]" />,
      bg: "bg-sky-50 border-sky-100",
    };
  };

  const filteredNotifications = notifications.filter((n) => {
    if (activeTab === "UNREAD") return !n.is_read;
    return true;
  });

  return (
    <div className="relative" ref={popoverRef}>
      {/* Notification Bell Trigger */}
      <button
        type="button"
        onClick={handleToggle}
        aria-label="Notifications"
        className={`relative w-9 h-9 rounded-full flex items-center justify-center transition-colors cursor-pointer ${
          isOpen
            ? "bg-[#E3F2FD] text-[#0D47A1] ring-2 ring-[#2196F3]/40"
            : isTransparent
              ? "text-white hover:bg-white/15"
              : "text-slate-700 hover:bg-[#E3F2FD] hover:text-[#0D47A1]"
        }`}
      >
        <Bell className="w-4 h-4" />
        {unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 min-w-[18px] h-[18px] px-1 bg-rose-500 text-white text-[10px] font-black rounded-full flex items-center justify-center shadow-xs">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </button>

      {/* Floating Popover Card */}
      {isOpen && (
        <div className="absolute right-0 sm:-right-6 top-12 w-[340px] sm:w-[410px] max-w-[calc(100vw-32px)] bg-white rounded-3xl shadow-2xl border border-slate-200/90 z-50 overflow-hidden flex flex-col animate-in fade-in zoom-in-95 duration-150">
          {/* Header */}
          <div className="p-4 bg-slate-50/80 border-b border-slate-200/70 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="w-7 h-7 rounded-xl bg-[#E3F2FD] text-[#0D47A1] flex items-center justify-center">
                <Bell className="w-3.5 h-3.5" />
              </div>
              <div>
                <h3 className="text-sm font-extrabold text-[#0D1B2A] tracking-tight">
                  Notifications
                </h3>
                <p className="text-[11px] text-slate-500 font-medium">
                  {unreadCount > 0
                    ? `${unreadCount} unread update(s)`
                    : "All caught up"}
                </p>
              </div>
            </div>

            <div className="flex items-center gap-1.5">
              {unreadCount > 0 && (
                <button
                  type="button"
                  onClick={markAllAsRead}
                  disabled={isMarkingAllRead}
                  className="px-2.5 py-1.5 rounded-xl bg-white border border-slate-200 hover:bg-slate-100 text-slate-700 text-[11px] font-semibold flex items-center gap-1 transition-colors cursor-pointer disabled:opacity-50"
                  title="Mark all as read"
                >
                  <CheckCheck className="w-3 h-3 text-[#2196F3]" />
                  <span>Mark all as read</span>
                </button>
              )}
              <button
                type="button"
                onClick={() => fetchNotifications()}
                className="p-1.5 rounded-xl text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 transition-colors"
                title="Refresh"
              >
                <RefreshCw
                  className={`w-3.5 h-3.5 ${isLoading ? "animate-spin" : ""}`}
                />
              </button>
              <button
                type="button"
                onClick={() => setIsOpen(false)}
                className="p-1.5 rounded-xl text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 transition-colors"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          {/* Filter Tabs */}
          <div className="px-4 pt-2.5 pb-1 flex items-center gap-1 border-b border-slate-100 bg-white">
            <button
              type="button"
              onClick={() => setActiveTab("ALL")}
              className={`px-3 py-1 rounded-full text-xs font-semibold transition-all cursor-pointer ${
                activeTab === "ALL"
                  ? "bg-[#0D47A1] text-white shadow-2xs"
                  : "text-slate-500 hover:text-slate-800 hover:bg-slate-100"
              }`}
            >
              All ({notifications.length})
            </button>
            <button
              type="button"
              onClick={() => setActiveTab("UNREAD")}
              className={`px-3 py-1 rounded-full text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 ${
                activeTab === "UNREAD"
                  ? "bg-[#0D47A1] text-white shadow-2xs"
                  : "text-slate-500 hover:text-slate-800 hover:bg-slate-100"
              }`}
            >
              <span>Unread</span>
              {unreadCount > 0 && (
                <span
                  className={`px-1.5 py-0.2 rounded-full text-[10px] font-bold ${
                    activeTab === "UNREAD"
                      ? "bg-white text-[#0D47A1]"
                      : "bg-rose-100 text-rose-700"
                  }`}
                >
                  {unreadCount}
                </span>
              )}
            </button>
          </div>

          {/* Scrollable Notification List */}
          <div className="overflow-y-auto max-h-[360px] divide-y divide-slate-100">
            {isLoading && notifications.length === 0 ? (
              <div className="py-12 text-center text-slate-400 space-y-2">
                <RefreshCw className="w-6 h-6 animate-spin mx-auto text-[#2196F3]" />
                <p className="text-xs font-medium">Loading notifications...</p>
              </div>
            ) : filteredNotifications.length === 0 ? (
              <div className="py-12 px-6 text-center space-y-2">
                <div className="w-12 h-12 rounded-full bg-slate-100 text-slate-400 flex items-center justify-center mx-auto">
                  <Sparkles className="w-5 h-5 text-slate-400" />
                </div>
                <p className="text-xs font-bold text-[#0D1B2A]">
                  {activeTab === "UNREAD"
                    ? "No unread notifications"
                    : "No notifications yet"}
                </p>
                <p className="text-[11px] text-slate-400 max-w-[240px] mx-auto">
                  {activeTab === "UNREAD"
                    ? "You have read all received updates."
                    : "Important updates regarding your orders and promos will appear here."}
                </p>
              </div>
            ) : (
              filteredNotifications.map((item) => {
                const visual = getNotificationVisual(item.type);

                return (
                  <div
                    key={item.uuid}
                    onClick={() => handleItemClick(item)}
                    className={`p-3.5 transition-all flex items-start gap-3 cursor-pointer group hover:bg-slate-50 relative ${
                      !item.is_read ? "bg-[#F0F7FF]/70" : "bg-white"
                    }`}
                  >
                    {/* Visual Icon Badge */}
                    <div
                      className={`w-9 h-9 rounded-2xl flex items-center justify-center shrink-0 border ${visual.bg}`}
                    >
                      {visual.icon}
                    </div>

                    {/* Content */}
                    <div className="flex-1 min-w-0 space-y-0.5">
                      <div className="flex items-center justify-between gap-1">
                        <h4
                          className={`text-xs truncate ${
                            !item.is_read
                              ? "font-bold text-[#0D1B2A]"
                              : "font-medium text-slate-700"
                          }`}
                        >
                          {item.title}
                        </h4>
                        <span className="text-[10px] text-slate-400 shrink-0 font-medium">
                          {formatRelativeTime(item.created_at)}
                        </span>
                      </div>

                      <p className="text-[11px] text-slate-600 line-clamp-2 leading-relaxed">
                        {item.body}
                      </p>

                      {/* Attached Info / Tags */}
                      {item.data?.order_number && (
                        <div className="pt-1 flex items-center gap-1.5 text-[10px] text-slate-500 font-mono">
                          <span className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 font-bold">
                            #{item.data.order_number}
                          </span>
                        </div>
                      )}
                    </div>

                    {/* Right Indicators & Quick Actions */}
                    <div className="flex flex-col items-end gap-1.5 shrink-0 pt-0.5">
                      {!item.is_read ? (
                        <span className="w-2 h-2 rounded-full bg-[#2196F3] shadow-xs" />
                      ) : (
                        <span className="w-2 h-2" />
                      )}

                      <div className="opacity-0 group-hover:opacity-100 flex items-center gap-1 transition-opacity">
                        {!item.is_read && (
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              markAsRead(item.uuid);
                            }}
                            className="p-1 rounded-lg hover:bg-blue-100 text-blue-600 transition-colors"
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
                          className="p-1 rounded-lg hover:bg-rose-100 text-rose-500 transition-colors"
                          title="Delete notification"
                        >
                          <Trash2 className="w-3 h-3" />
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </div>

          {/* Footer */}
          <div className="p-3 bg-slate-50/90 border-t border-slate-100 text-center">
            <Link
              href="/account/notifications"
              onClick={() => setIsOpen(false)}
              className="text-xs font-bold text-[#0D47A1] hover:text-[#2196F3] inline-flex items-center gap-1.5 transition-colors"
            >
              <span>View All Notifications</span>
              <ExternalLink className="w-3.5 h-3.5" />
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}
