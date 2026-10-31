"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth-store";
import {
  useNotificationStore,
  NotificationItem,
} from "@/stores/notification-store";
import { useMounted } from "@/hooks/use-mounted";
import { formatDistanceToNow, formatDateTime } from "@/lib/format";
import {
  Bell,
  CheckCheck,
  Check,
  Trash2,
  ExternalLink,
  RefreshCw,
  Loader2,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
} from "lucide-react";
import { toast } from "sonner";

type FilterTab = "ALL" | "UNREAD" | "ORDERS" | "PROMO" | "SECURITY";

export default function NotificationsPage() {
  const router = useRouter();
  const { accessToken } = useAuthStore();
  const mounted = useMounted();

  const {
    notifications,
    unreadCount,
    isLoading: storeLoading,
    isMarkingAllRead,
    fetchNotifications,
    fetchUnreadCount,
    markAsRead,
    markAllAsRead,
    deleteNotification,
  } = useNotificationStore();

  const [activeTab, setActiveTab] = useState<FilterTab>("ALL");
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const limit = 10;

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await Promise.all([fetchNotifications(), fetchUnreadCount()]);
      toast.success("Notifications refreshed");
    } catch {
      toast.error("Failed to refresh notifications");
    } finally {
      setIsRefreshing(false);
    }
  };

  // Filter notifications according to active tab
  const filteredNotifications = notifications.filter((item) => {
    if (activeTab === "UNREAD") return !item.is_read;
    if (activeTab === "SECURITY")
      return (
        item.type === "security_alert" || item.type.includes("security")
      );
    if (activeTab === "ORDERS")
      return (
        item.type.includes("order") ||
        item.type.includes("shipping") ||
        item.type.includes("delivery") ||
        item.type.includes("return")
      );
    if (activeTab === "PROMO")
      return (
        item.type.includes("promo") ||
        item.type.includes("coupon") ||
        item.type.includes("discount") ||
        item.type.includes("drop")
      );
    return true;
  });

  // Pagination calculation
  const totalRecords = filteredNotifications.length;
  const totalPages = Math.max(1, Math.ceil(totalRecords / limit));
  const paginatedItems = filteredNotifications.slice(
    (currentPage - 1) * limit,
    currentPage * limit,
  );

  // Reset page when filter tab changes
  useEffect(() => {
    setCurrentPage(1);
  }, [activeTab]);

  // Clean brand-oriented semantic metadata for notification types
  const getNotificationBadge = (type: string) => {
    if (type === "security_alert" || type.includes("security")) {
      return {
        label: "Account Security",
        badgeClass: "bg-[#FEF2F2] text-[#DC2626] border border-[#FECACA]",
        borderIndicator: "border-l-[#DC2626]",
        dotClass: "bg-[#DC2626]",
      };
    }
    if (type.includes("return")) {
      return {
        label: "Return & Refund",
        badgeClass: "bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/30",
        borderIndicator: "border-l-[#007D9C]",
        dotClass: "bg-[#007D9C]",
      };
    }
    if (
      type.includes("order") ||
      type.includes("shipping") ||
      type.includes("delivery")
    ) {
      return {
        label: "Order & Shipping",
        badgeClass: "bg-[#E6F7FB] text-[#007D9C] border border-[#00ADD8]/30",
        borderIndicator: "border-l-[#007D9C]",
        dotClass: "bg-[#007D9C]",
      };
    }
    if (
      type.includes("promo") ||
      type.includes("coupon") ||
      type.includes("discount") ||
      type.includes("drop")
    ) {
      return {
        label: "Hardware Drop & Promo",
        badgeClass: "bg-[#FFF7ED] text-[#F26522] border border-[#F26522]/30",
        borderIndicator: "border-l-[#F26522]",
        dotClass: "bg-[#F26522]",
      };
    }
    if (type.includes("review")) {
      return {
        label: "Product Review",
        badgeClass: "bg-[#FFFBEB] text-[#D97706] border border-[#FDE68A]",
        borderIndicator: "border-l-[#D97706]",
        dotClass: "bg-[#D97706]",
      };
    }
    return {
      label: "System Notification",
      badgeClass: "bg-slate-100 text-slate-700 border border-[#E2E8F0]",
      borderIndicator: "border-l-slate-400",
      dotClass: "bg-slate-400",
    };
  };

  // Handle action navigation
  const handleActionClick = (item: NotificationItem) => {
    if (!item.is_read) {
      markAsRead(item.uuid);
    }

    const dataUrl = item.data?.url;
    if (typeof dataUrl === "string") {
      router.push(dataUrl);
      return;
    }

    if (item.type === "security_alert" || item.type.includes("security")) {
      router.push("/account/security");
      return;
    }

    if (item.type.includes("return")) {
      router.push("/account/returns");
      return;
    }

    if (
      item.type.includes("order") ||
      item.type.includes("shipping") ||
      item.type.includes("delivery")
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
      router.push("/account/reviews");
      return;
    }
  };

  // Pagination page numbers generator
  const getPaginationPages = () => {
    const pages: (number | string)[] = [];
    const delta = 2;

    for (let i = 1; i <= totalPages; i++) {
      if (
        i === 1 ||
        i === totalPages ||
        (i >= currentPage - delta && i <= currentPage + delta)
      ) {
        pages.push(i);
      } else if (
        (i === currentPage - delta - 1 || i === currentPage + delta + 1) &&
        pages[pages.length - 1] !== "..."
      ) {
        pages.push("...");
      }
    }

    return pages;
  };

  if (!mounted) {
    return (
      <div className="p-10 rounded-md bg-white border border-[#E2E8F0] flex items-center justify-center min-h-[360px]">
        <div className="w-8 h-8 rounded-md border-2 border-[#007D9C] border-t-transparent animate-spin" />
      </div>
    );
  }

  const tabs: { id: FilterTab; label: string; count: number }[] = [
    { id: "ALL", label: "All", count: notifications.length },
    { id: "UNREAD", label: "Unread", count: unreadCount },
    {
      id: "ORDERS",
      label: "Orders & Shipping",
      count: notifications.filter(
        (n) =>
          n.type.includes("order") ||
          n.type.includes("shipping") ||
          n.type.includes("delivery"),
      ).length,
    },
    {
      id: "PROMO",
      label: "Drops & Promos",
      count: notifications.filter(
        (n) =>
          n.type.includes("promo") ||
          n.type.includes("coupon") ||
          n.type.includes("drop"),
      ).length,
    },
    {
      id: "SECURITY",
      label: "Security",
      count: notifications.filter(
        (n) => n.type === "security_alert" || n.type.includes("security"),
      ).length,
    },
  ];

  return (
    <div className="space-y-6 font-body">
      {/* 1. Page Header Card */}
      <div className="bg-white border border-[#E2E8F0] rounded-md p-6 flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="font-bold text-2xl text-[#0F172A] tracking-tight font-heading">
              Notifications
            </h1>
            {unreadCount > 0 && (
              <span className="bg-[#007D9C] text-white text-xs font-bold px-2.5 py-0.5 rounded-md font-heading">
                {unreadCount} Unread
              </span>
            )}
          </div>
          <p className="text-sm text-[#64748B] mt-1">
            Stay updated on order shipments, account security alerts, and promotional announcements.
          </p>
        </div>

        <div className="flex items-center gap-2 self-start md:self-auto">
          <button
            type="button"
            onClick={handleRefresh}
            disabled={isRefreshing || storeLoading}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] text-[#0F172A] text-xs font-semibold disabled:opacity-50 cursor-pointer font-heading"
            title="Refresh notifications"
          >
            <RefreshCw
              className={`w-3.5 h-3.5 ${isRefreshing ? "animate-spin text-[#007D9C]" : "text-slate-500"}`}
            />
            <span>Refresh</span>
          </button>

          {unreadCount > 0 && (
            <button
              type="button"
              onClick={() => markAllAsRead()}
              disabled={isMarkingAllRead}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-md bg-[#007D9C] hover:bg-[#00637C] text-white text-xs font-semibold disabled:opacity-50 cursor-pointer font-heading"
            >
              {isMarkingAllRead ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Marking...</span>
                </>
              ) : (
                <>
                  <CheckCheck className="w-3.5 h-3.5" />
                  <span>Mark All Read</span>
                </>
              )}
            </button>
          )}
        </div>
      </div>

      {/* 2. Filter Tabs Toolbar */}
      <div className="flex flex-wrap gap-2 border-b border-[#E2E8F0] pb-3 text-xs font-semibold font-heading">
        {tabs.map((tab) => {
          const isActive = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`px-3.5 py-2 rounded-md border cursor-pointer flex items-center gap-1.5 ${
                isActive
                  ? "bg-[#E6F7FB] text-[#007D9C] border-[#00ADD8]/40"
                  : "bg-white text-[#64748B] hover:text-[#0F172A] border-[#E2E8F0]"
              }`}
            >
              <span>{tab.label}</span>
              {tab.count > 0 && (
                <span
                  className={`px-1.5 py-0.2 rounded-md text-[10px] font-bold ${
                    isActive
                      ? "bg-[#007D9C] text-white"
                      : "bg-[#F8FAFC] border border-[#E2E8F0] text-slate-600"
                  }`}
                >
                  {tab.count}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* 3. Notification Feed Items */}
      {storeLoading && notifications.length === 0 ? (
        <div className="p-12 rounded-md bg-white border border-[#E2E8F0] text-center space-y-3">
          <Loader2 className="w-8 h-8 text-[#007D9C] animate-spin mx-auto" />
          <p className="text-xs text-[#64748B] font-semibold font-heading">
            Loading notifications...
          </p>
        </div>
      ) : paginatedItems.length > 0 ? (
        <div className="space-y-3">
          {paginatedItems.map((item) => {
            const badgeInfo = getNotificationBadge(item.type);
            const isUnread = !item.is_read;

            const hasAction = Boolean(
              item.data?.url ||
                item.type === "security_alert" ||
                item.type.includes("security") ||
                item.type.includes("order") ||
                item.type.includes("shipping") ||
                item.type.includes("review"),
            );

            return (
              <div
                key={item.uuid}
                className={`bg-white border border-[#E2E8F0] rounded-md p-5 relative overflow-hidden ${
                  isUnread ? `border-l-4 ${badgeInfo.borderIndicator}` : "bg-[#F8FAFC]/60"
                }`}
              >
                <div className="flex flex-col space-y-2">
                  {/* Top Row: Badge, Title, Dot, Timestamp */}
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span
                        className={`px-2 py-0.5 rounded-md text-[11px] font-bold font-heading ${badgeInfo.badgeClass}`}
                      >
                        {badgeInfo.label}
                      </span>
                      {isUnread && (
                        <span
                          className={`w-2 h-2 rounded-full inline-block ${badgeInfo.dotClass}`}
                          title="Unread"
                        />
                      )}
                      <h3
                        className={`text-sm font-heading ${
                          isUnread
                            ? "font-bold text-[#0F172A]"
                            : "font-semibold text-slate-700"
                        }`}
                      >
                        {item.title}
                      </h3>
                    </div>

                    <span
                      className="text-[11px] text-[#64748B] font-mono shrink-0"
                      title={formatDateTime(item.created_at)}
                    >
                      {formatDistanceToNow(item.created_at)}
                    </span>
                  </div>

                  {/* Notification Body */}
                  <div className="text-xs text-[#0F172A] leading-relaxed whitespace-pre-line break-words pt-1">
                    {item.body}
                  </div>

                  {/* Bottom Actions Row */}
                  <div className="pt-3 border-t border-[#E2E8F0] flex flex-wrap items-center justify-between gap-3">
                    {hasAction ? (
                      <button
                        type="button"
                        onClick={() => handleActionClick(item)}
                        className="text-xs font-semibold text-[#007D9C] hover:text-[#00637C] inline-flex items-center gap-1.5 cursor-pointer font-heading"
                      >
                        <span>
                          {item.type === "security_alert" ||
                          item.type.includes("security")
                            ? "Manage Active Sessions"
                            : item.type.includes("order") ||
                                item.type.includes("shipping")
                              ? "View Order Details"
                              : item.type.includes("review")
                                ? "View My Reviews"
                                : "View Details"}
                        </span>
                        <ExternalLink className="w-3.5 h-3.5" />
                      </button>
                    ) : (
                      <div />
                    )}

                    <div className="flex items-center gap-2">
                      {isUnread && (
                        <button
                          type="button"
                          onClick={() => markAsRead(item.uuid)}
                          className="inline-flex items-center gap-1 px-3 py-1.5 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] text-slate-700 text-xs font-semibold cursor-pointer font-heading"
                          title="Mark as read"
                        >
                          <Check className="w-3.5 h-3.5 text-[#007D9C]" />
                          <span>Mark Read</span>
                        </button>
                      )}

                      <button
                        type="button"
                        onClick={() => deleteNotification(item.uuid)}
                        className="inline-flex items-center gap-1 px-3 py-1.5 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#FEF2F2] text-[#DC2626] text-xs font-semibold cursor-pointer font-heading"
                        title="Delete notification"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                        <span>Delete</span>
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}

          {/* Pagination Controls */}
          {totalPages > 1 && (
            <div className="p-4 bg-white border border-[#E2E8F0] rounded-md flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs mt-4">
              <span className="text-[#64748B] font-medium text-center sm:text-left">
                Showing page <strong className="text-[#0F172A]">{currentPage}</strong> of{" "}
                <strong className="text-[#0F172A]">{totalPages}</strong> ({totalRecords} total items)
              </span>

              <div className="flex items-center justify-center gap-1 font-heading">
                {/* First Page */}
                <button
                  type="button"
                  disabled={currentPage <= 1}
                  onClick={() => setCurrentPage(1)}
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center cursor-pointer"
                  title="First Page"
                >
                  <ChevronsLeft className="w-4 h-4" />
                </button>

                {/* Prev Page */}
                <button
                  type="button"
                  disabled={currentPage <= 1}
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center cursor-pointer"
                  title="Previous Page"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>

                {/* Numbered Buttons */}
                {getPaginationPages().map((pageNum, idx) => {
                  if (pageNum === "...") {
                    return (
                      <span
                        key={`dots-${idx}`}
                        className="w-8 h-8 flex items-center justify-center text-slate-400 font-bold select-none"
                      >
                        ...
                      </span>
                    );
                  }

                  const pageNumber = Number(pageNum);
                  const isActive = pageNumber === currentPage;

                  return (
                    <button
                      key={pageNumber}
                      type="button"
                      onClick={() => setCurrentPage(pageNumber)}
                      className={`w-8 h-8 rounded-md text-xs font-bold cursor-pointer ${
                        isActive
                          ? "bg-[#007D9C] text-white"
                          : "border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] text-[#0F172A]"
                      }`}
                    >
                      {pageNumber}
                    </button>
                  );
                })}

                {/* Next Page */}
                <button
                  type="button"
                  disabled={currentPage >= totalPages}
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center cursor-pointer"
                  title="Next Page"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>

                {/* Last Page */}
                <button
                  type="button"
                  disabled={currentPage >= totalPages}
                  onClick={() => setCurrentPage(totalPages)}
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center cursor-pointer"
                  title="Last Page"
                >
                  <ChevronsRight className="w-4 h-4" />
                </button>
              </div>
            </div>
          )}
        </div>
      ) : (
        /* Empty State */
        <div className="p-12 sm:p-16 rounded-md bg-white border border-dashed border-[#CBD5E1] text-center space-y-4">
          <div className="w-14 h-14 rounded-md bg-[#E6F7FB] text-[#007D9C] flex items-center justify-center mx-auto">
            <Bell className="w-7 h-7" />
          </div>
          <div className="space-y-1 max-w-md mx-auto">
            <h3 className="text-base font-bold text-[#0F172A] font-heading">
              {activeTab === "UNREAD"
                ? "No Unread Notifications"
                : activeTab === "SECURITY"
                  ? "No Security Alerts"
                  : activeTab === "ORDERS"
                    ? "No Order Notifications"
                    : activeTab === "PROMO"
                      ? "No Promotion Notifications"
                      : "No Notifications Yet"}
            </h3>
            <p className="text-xs text-[#64748B] leading-relaxed">
              {activeTab === "UNREAD"
                ? "You are all caught up! There are no unread notifications at the moment."
                : "When there are important updates regarding your account, orders, or security, they will appear here."}
            </p>
          </div>
          {activeTab !== "ALL" && (
            <div className="pt-2">
              <button
                type="button"
                onClick={() => setActiveTab("ALL")}
                className="inline-flex items-center gap-1.5 px-4 py-2 rounded-md bg-[#F8FAFC] hover:bg-slate-200 border border-[#E2E8F0] text-[#0F172A] text-xs font-semibold cursor-pointer font-heading"
              >
                <span>View All Notifications</span>
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
