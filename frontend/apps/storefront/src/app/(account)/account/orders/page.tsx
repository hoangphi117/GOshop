"use client";

import React, {
  useState,
  useEffect,
  useCallback,
  useRef,
  Suspense,
} from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import { useAuthStore } from "@/stores/auth-store";
import { authFetch } from "@/lib/api";
import { formatCurrency, formatDate, formatDateTime } from "@/lib/format";
import { getImageUrl, extractErrorMessage } from "@/lib/utils";
import { WriteReviewModal } from "@/components/review/write-review-modal";
import { RequestReturnModal } from "@/components/order/request-return-modal";
import { ReturnDetailModal } from "@/components/order/return-detail-modal";
import { OrderTrackingModal } from "@/components/order/order-tracking-modal";
import { useChatUIStore } from "@/stores/chat-ui-store";
import type { OrderRef } from "@/hooks/use-chat";
import {
  ShoppingBag,
  Clock,
  CheckCircle2,
  Package,
  AlertCircle,
  ArrowRight,
  ArrowLeft,
  CreditCard,
  X,
  Truck,
  Eye,
  RefreshCw,
  MapPin,
  FileText,
  Tag,
  Ban,
  RotateCcw,
  AlertTriangle,
  CheckCheck,
  ShieldCheck,
  PackageCheck,
  Star,
  Search,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Download,
  Loader2,
  MessageSquare,
  Copy,
} from "lucide-react";

interface OrderItem {
  uuid?: string;
  order_item_id?: number;
  product_id?: number;
  product_uuid?: string;
  product_name: string;
  image_url?: string;
  price: number;
  quantity: number;
  subtotal?: number;
}

interface TrackingEvent {
  event_id?: number;
  status: string;
  status_text?: string;
  note?: string;
  source?: string;
  created_at: string;
}

interface Order {
  uuid: string;
  order_number: string;
  created_at: string;
  status: string;
  payment_status?: string;
  payment_method?: string;
  total_amount: number;
  subtotal_amount?: number;
  shipping_fee?: number;
  discount_amount?: number;
  coupon_code?: string;
  shipping_name?: string;
  shipping_phone?: string;
  shipping_address?: string;
  shipping_code?: string;
  shipping_provider?: string;
  note?: string;
  items?: OrderItem[];
}

function MyOrdersPageContent() {
  const { accessToken } = useAuthStore();
  const { openChatWithOrder } = useChatUIStore();
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState("ALL");
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [isSearching, setIsSearching] = useState(false);

  const handleChatAboutOrder = (order: Order) => {
    const orderRef: OrderRef = {
      uuid: order.uuid,
      order_number: order.order_number,
      status: order.status,
      total_amount: order.total_amount,
      created_at: order.created_at,
      items: order.items?.map((it) => ({
        uuid: it.uuid,
        product_uuid: it.product_uuid,
        product_name: it.product_name,
        image_url: it.image_url ? getImageUrl(it.image_url) : undefined,
        price: it.price,
        quantity: it.quantity,
        subtotal: it.subtotal,
      })),
    };
    openChatWithOrder(orderRef);
  };

  // Pagination State
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [pagination, setPagination] = useState({
    page: 1,
    limit: 10,
    total_records: 0,
    total_pages: 1,
    has_next: false,
    has_prev: false,
  });

  // 300ms Debounce on search input
  useEffect(() => {
    if (searchInput !== debouncedSearch) {
      setIsSearching(true);
    }
    const timer = setTimeout(() => {
      setDebouncedSearch(searchInput);
      setIsSearching(false);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput, debouncedSearch]);

  // Reset to page 1 on filter, search keyword, or page size change
  useEffect(() => {
    setCurrentPage(1);
  }, [filter, debouncedSearch, pageSize]);

  // Selected Order for in-place Detail View
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null);
  const [timelineEvents, setTimelineEvents] = useState<TrackingEvent[]>([]);
  const [isLoadingDetail, setIsLoadingDetail] = useState(false);

  // Cancel Order Modal State
  const [orderToCancel, setOrderToCancel] = useState<Order | null>(null);
  const [isCancelling, setIsCancelling] = useState(false);

  // Return / Refund Modal State
  const [orderToReturn, setOrderToReturn] = useState<Order | null>(null);
  const [orderToViewReturn, setOrderToViewReturn] = useState<Order | null>(
    null,
  );

  // Live Tracking Modal State
  const [orderToTrack, setOrderToTrack] = useState<Order | null>(null);

  // Confirm Delivery Modal State
  const [orderToConfirmDelivery, setOrderToConfirmDelivery] =
    useState<Order | null>(null);
  const [isConfirmingDelivery, setIsConfirmingDelivery] = useState(false);

  // Invoice Download State
  const [downloadingInvoiceUuid, setDownloadingInvoiceUuid] = useState<
    string | null
  >(null);

  // Write Review Modal State
  const [reviewingItem, setReviewingItem] = useState<{
    orderUuid: string;
    productUuid: string;
    productName: string;
    productImageUrl?: string;
  } | null>(null);

  // Pay Now State
  const [payingOrderUuid, setPayingOrderUuid] = useState<string | null>(null);

  // Sync Tracking State
  const [isSyncingTracking, setIsSyncingTracking] = useState(false);

  const fetchOrders = useCallback(async () => {
    if (!accessToken) return;
    setLoading(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const params = new URLSearchParams();
      params.set("page", String(currentPage));
      params.set("limit", String(pageSize));

      if (debouncedSearch.trim()) {
        params.set("search", debouncedSearch.trim());
      }

      const statusMap: Record<string, number> = {
        PENDING: 1,
        CONFIRMED: 2,
        SHIPPED: 3,
        DELIVERED: 4,
        CANCELLED: 5,
        RETURNED: 6,
      };
      if (filter !== "ALL" && statusMap[filter]) {
        params.set("status", String(statusMap[filter]));
      }

      const res = await authFetch(
        `${baseUrl}/api/v1/orders?${params.toString()}`,
      );
      if (res.ok) {
        const json = await res.json();
        const orderList: Order[] = Array.isArray(json?.data)
          ? json.data
          : Array.isArray(json?.data?.data)
            ? json.data.data
            : Array.isArray(json?.data?.items)
              ? json.data.items
              : [];
        setOrders(orderList);

        const paginationData = json?.pagination || json?.data?.pagination;
        if (paginationData) {
          setPagination(paginationData);
        } else {
          setPagination({
            page: currentPage,
            limit: pageSize,
            total_records: orderList.length,
            total_pages: Math.max(1, Math.ceil(orderList.length / pageSize)),
            has_next: false,
            has_prev: false,
          });
        }
      } else {
        setOrders([]);
      }
    } catch {
      setOrders([]);
    } finally {
      setLoading(false);
    }
  }, [accessToken, currentPage, pageSize, debouncedSearch, filter]);

  useEffect(() => {
    fetchOrders();
  }, [fetchOrders]);

  const router = useRouter();
  const searchParams = useSearchParams();
  const urlOrderId = searchParams.get("orderId") || searchParams.get("order");
  const activeOrderRef = useRef<string | null>(null);

  // In-place switch to order detail with URL synchronization (without reload)
  const handleOpenOrderDetails = useCallback(
    async (order: Order, updateUrl = true) => {
      setSelectedOrder(order);
      setTimelineEvents([]);
      setIsLoadingDetail(true);

      const ref = order.order_number || order.uuid;
      activeOrderRef.current = ref;

      if (updateUrl) {
        router.push(`/account/orders?orderId=${encodeURIComponent(ref)}`, {
          scroll: false,
        });
      }

      try {
        const baseUrl =
          process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

        const [orderRes, timelineRes] = await Promise.allSettled([
          authFetch(`${baseUrl}/api/v1/orders/${order.uuid}`),
          authFetch(`${baseUrl}/api/v1/orders/${order.uuid}/timeline`),
        ]);

        if (orderRes.status === "fulfilled" && orderRes.value.ok) {
          const json = await orderRes.value.json();
          if (json?.data) {
            setSelectedOrder(json.data);
          }
        }

        if (timelineRes.status === "fulfilled" && timelineRes.value.ok) {
          const json = await timelineRes.value.json();
          const events: TrackingEvent[] = Array.isArray(json?.data?.events)
            ? json.data.events
            : Array.isArray(json?.data)
              ? json.data
              : [];
          setTimelineEvents(events);
        }
      } catch {
        // Retain existing summary if network fails
      } finally {
        setIsLoadingDetail(false);
      }
    },
    [router],
  );

  const handleBackToOrdersList = () => {
    setSelectedOrder(null);
    activeOrderRef.current = null;
    router.push("/account/orders", { scroll: false });
  };

  // Sync URL searchParams orderId on mount or external navigation
  useEffect(() => {
    if (!accessToken) return;

    if (!urlOrderId) {
      if (activeOrderRef.current !== null || selectedOrder !== null) {
        activeOrderRef.current = null;
        setSelectedOrder(null);
      }
      return;
    }

    if (
      activeOrderRef.current === urlOrderId ||
      (selectedOrder &&
        (selectedOrder.order_number === urlOrderId ||
          selectedOrder.uuid === urlOrderId))
    ) {
      return;
    }

    const matched = orders.find(
      (o) => o.order_number === urlOrderId || o.uuid === urlOrderId,
    );
    if (matched) {
      handleOpenOrderDetails(matched, false);
      return;
    }

    let isCancelled = false;
    const fetchDirectOrder = async () => {
      setIsLoadingDetail(true);
      try {
        const baseUrl =
          process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
        const isUuid =
          /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
            urlOrderId,
          );
        if (isUuid) {
          const res = await authFetch(`${baseUrl}/api/v1/orders/${urlOrderId}`);
          if (res.ok) {
            const json = await res.json();
            if (json?.data && !isCancelled) {
              handleOpenOrderDetails(json.data, false);
              return;
            }
          }
        } else {
          const res = await authFetch(
            `${baseUrl}/api/v1/orders?search=${encodeURIComponent(urlOrderId)}`,
          );
          if (res.ok) {
            const json = await res.json();
            const orderList: Order[] = Array.isArray(json?.data)
              ? json.data
              : Array.isArray(json?.data?.items)
                ? json.data.items
                : [];
            const found = orderList.find(
              (o) => o.order_number === urlOrderId || o.uuid === urlOrderId,
            );
            if (found && !isCancelled) {
              handleOpenOrderDetails(found, false);
              return;
            }
          }
        }
      } catch {
        // ignore
      } finally {
        if (!isCancelled) {
          setIsLoadingDetail(false);
        }
      }
    };

    fetchDirectOrder();

    return () => {
      isCancelled = true;
    };
  }, [urlOrderId, orders, accessToken, handleOpenOrderDetails, selectedOrder]);

  // Cancel order handler
  const handleOpenCancelConfirmation = (order: Order) => {
    setOrderToCancel(order);
  };

  const handleConfirmCancel = async () => {
    if (!orderToCancel) return;
    setIsCancelling(true);

    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(
        `${baseUrl}/api/v1/orders/${orderToCancel.uuid}/cancel`,
        {
          method: "PATCH",
        },
      );

      if (res.ok) {
        toast.success(
          `Order #${orderToCancel.order_number || orderToCancel.uuid.slice(0, 8)} cancelled successfully.`,
        );
        setOrderToCancel(null);
        if (selectedOrder && selectedOrder.uuid === orderToCancel.uuid) {
          setSelectedOrder({ ...selectedOrder, status: "Cancelled" });
        }
        await fetchOrders();
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(errJson, "Failed to cancel order.");
        toast.error(msg);
      }
    } catch {
      toast.error("Network error occurred while cancelling order.");
    } finally {
      setIsCancelling(false);
    }
  };

  // Return / refund handlers
  const handleOpenReturnModal = (order: Order) => {
    setOrderToReturn(order);
  };

  const handleOpenReturnDetails = (order: Order) => {
    setOrderToViewReturn(order);
  };

  const handleOpenTrackingModal = (order: Order) => {
    setOrderToTrack(order);
  };

  // Confirm delivery handlers
  const handleOpenConfirmDelivery = (order: Order) => {
    setOrderToConfirmDelivery(order);
  };

  const handleConfirmDelivery = async () => {
    if (!orderToConfirmDelivery) return;
    setIsConfirmingDelivery(true);

    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(
        `${baseUrl}/api/v1/orders/${orderToConfirmDelivery.uuid}/confirm-delivery`,
        {
          method: "POST",
        },
      );

      if (res.ok) {
        toast.success(
          `Order #${orderToConfirmDelivery.order_number || orderToConfirmDelivery.uuid.slice(0, 8).toUpperCase()} delivery confirmed! Thank you for shopping with GOshop.`,
        );
        setOrderToConfirmDelivery(null);
        if (
          selectedOrder &&
          selectedOrder.uuid === orderToConfirmDelivery.uuid
        ) {
          setSelectedOrder({ ...selectedOrder, status: "Delivered" });
        }
        await fetchOrders();
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(errJson, "Failed to confirm delivery.");
        toast.error(msg);
      }
    } catch {
      toast.error("Network error while confirming order delivery.");
    } finally {
      setIsConfirmingDelivery(false);
    }
  };

  // Invoice download handler
  const handleDownloadInvoice = async (order: Order) => {
    if (downloadingInvoiceUuid) return;
    setDownloadingInvoiceUuid(order.uuid);

    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(
        `${baseUrl}/api/v1/orders/${order.uuid}/invoice`,
      );

      if (res.ok) {
        const blob = await res.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `Invoice-${order.order_number || order.uuid.slice(0, 8).toUpperCase()}.pdf`;
        document.body.appendChild(a);
        a.click();
        window.URL.revokeObjectURL(url);
        document.body.removeChild(a);
        toast.success("Invoice PDF downloaded successfully.");
      } else {
        toast.error(
          "Invoice is currently unavailable. Please try again later.",
        );
      }
    } catch {
      toast.error("Network error while downloading invoice PDF.");
    } finally {
      setDownloadingInvoiceUuid(null);
    }
  };

  // Review modal handler
  const handleOpenReviewItem = (
    order: Order,
    item: { product_uuid?: string; product_name: string; image_url?: string },
  ) => {
    if (!item.product_uuid) {
      toast.error("Product information is missing for review.");
      return;
    }
    setReviewingItem({
      orderUuid: order.uuid,
      productUuid: item.product_uuid,
      productName: item.product_name,
      productImageUrl: item.image_url ? getImageUrl(item.image_url) : undefined,
    });
  };

  // MoMo Pay Now handler
  const handlePayNow = async (order: Order) => {
    setPayingOrderUuid(order.uuid);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(
        `${baseUrl}/api/v1/orders/${order.uuid}/pay`,
        {
          method: "POST",
        },
      );
      if (res.ok) {
        const json = await res.json();
        const payUrl = json?.data?.pay_url || json?.data?.payment_url;
        if (payUrl) {
          window.location.href = payUrl;
        } else {
          toast.error(
            "Unable to initialize MoMo payment. Please contact support.",
          );
        }
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(
          errJson,
          "Payment initialization failed.",
        );
        toast.error(msg);
      }
    } catch {
      toast.error("Network error occurred during payment processing.");
    } finally {
      setPayingOrderUuid(null);
    }
  };

  // Sync tracking handler
  const handleSyncTracking = async (order: Order) => {
    setIsSyncingTracking(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(
        `${baseUrl}/api/v1/orders/${order.uuid}/sync-ghn-tracking`,
        {
          method: "POST",
        },
      );

      if (res.ok) {
        const syncJson = await res.json();
        toast.success("Carrier tracking status updated from GHN Express.");
        const syncEvents: TrackingEvent[] = Array.isArray(
          syncJson?.data?.events,
        )
          ? syncJson.data.events
          : [];

        if (syncEvents.length > 0) {
          setTimelineEvents(syncEvents);
        }

        const [orderRes, timelineRes] = await Promise.all([
          authFetch(`${baseUrl}/api/v1/orders/${order.uuid}`),
          authFetch(`${baseUrl}/api/v1/orders/${order.uuid}/timeline`),
        ]);
        if (orderRes.ok) {
          const ordJson = await orderRes.json();
          if (ordJson?.data) setSelectedOrder(ordJson.data);
        }
        if (timelineRes.ok) {
          const tlJson = await timelineRes.json();
          const events: TrackingEvent[] = Array.isArray(tlJson?.data?.events)
            ? tlJson.data.events
            : Array.isArray(tlJson?.data?.timeline)
              ? tlJson.data.timeline
              : Array.isArray(tlJson?.data)
                ? tlJson.data
                : [];
          if (events.length > 0) {
            setTimelineEvents(events);
          }
        }
        await fetchOrders();
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(
          errJson,
          "Failed to sync carrier tracking.",
        );
        toast.error(msg);
      }
    } catch {
      toast.error("Network error while synchronizing tracking details.");
    } finally {
      setIsSyncingTracking(false);
    }
  };

  const generatePaginationPages = (current: number, total: number) => {
    if (total <= 7) {
      return Array.from({ length: total }, (_, i) => i + 1);
    }
    if (current <= 4) {
      return [1, 2, 3, 4, 5, "...", total];
    }
    if (current >= total - 3) {
      return [1, "...", total - 4, total - 3, total - 2, total - 1, total];
    }
    return [1, "...", current - 1, current, current + 1, "...", total];
  };

  // Order Fulfillment Status Badge (Flat, MASTER.md compliant, rounded-md)
  const getOrderStatusBadge = (status: string) => {
    const s = (status || "").toUpperCase();
    if (s.includes("DELIVER") || s.includes("COMPLETE")) {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#ECFDF5] text-[#059669] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#A7F3D0]">
          <CheckCircle2 className="w-3.5 h-3.5" />
          {status}
        </span>
      );
    }
    if (s.includes("SHIP") || s.includes("TRANSIT")) {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#E6F7FB] text-[#007D9C] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#00ADD8]/40">
          <Truck className="w-3.5 h-3.5" />
          {status}
        </span>
      );
    }
    if (s.includes("CONFIRM")) {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#E6F7FB] text-[#007D9C] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#00ADD8]/30">
          <CheckCircle2 className="w-3.5 h-3.5" />
          {status}
        </span>
      );
    }
    if (s.includes("PENDING")) {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#FFFBEB] text-[#D97706] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#FDE68A]">
          <Clock className="w-3.5 h-3.5" />
          {status}
        </span>
      );
    }
    if (s.includes("CANCEL")) {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#FEF2F2] text-[#DC2626] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#FECACA]">
          <AlertCircle className="w-3.5 h-3.5" />
          {status}
        </span>
      );
    }
    if (s.includes("RETURN")) {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#F5F3FF] text-[#7C3AED] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#DDD6FE]">
          <RotateCcw className="w-3.5 h-3.5" />
          {status}
        </span>
      );
    }
    return (
      <span className="px-2.5 py-1 rounded-md bg-slate-100 text-slate-700 text-xs font-semibold inline-flex items-center gap-1.5 border border-[#E2E8F0]">
        <Package className="w-3.5 h-3.5" />
        {status || "Processing"}
      </span>
    );
  };

  // Payment Status Badge (Flat, MASTER.md compliant, rounded-md)
  const getPaymentStatusBadge = (paymentStatus?: string) => {
    const ps = (paymentStatus || "").toUpperCase();
    if (ps === "PAID") {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#ECFDF5] text-[#059669] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#A7F3D0]">
          <CheckCircle2 className="w-3.5 h-3.5 text-[#059669]" />
          Paid
        </span>
      );
    }
    if (ps === "UNPAID") {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#FFFBEB] text-[#D97706] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#FDE68A]">
          <Clock className="w-3.5 h-3.5 text-[#D97706]" />
          Unpaid
        </span>
      );
    }
    if (ps.includes("REFUND")) {
      return (
        <span className="px-2.5 py-1 rounded-md bg-[#F5F3FF] text-[#7C3AED] text-xs font-semibold inline-flex items-center gap-1.5 border border-[#DDD6FE]">
          <RefreshCw className="w-3.5 h-3.5 text-[#7C3AED]" />
          {paymentStatus}
        </span>
      );
    }
    return (
      <span className="px-2.5 py-1 rounded-md bg-slate-100 text-slate-700 text-xs font-semibold inline-flex items-center gap-1.5 border border-[#E2E8F0]">
        <CreditCard className="w-3.5 h-3.5 text-slate-500" />
        {paymentStatus || "Pending"}
      </span>
    );
  };

  const checkIsCOD = (order: Order) => {
    const pm = (order.payment_method || "").toUpperCase();
    return pm.includes("COD") || pm.includes("CASH");
  };

  const isShippingEvent = (ev: TrackingEvent): boolean => {
    const s = (ev.status || "").toLowerCase();
    const st = (ev.status_text || "").toLowerCase();
    return (
      s.includes("ship") ||
      s.includes("transit") ||
      s.includes("pick") ||
      s.includes("transport") ||
      s.includes("delivering") ||
      st.includes("ship") ||
      st.includes("transit") ||
      st.includes("delivering") ||
      st.includes("out for delivery") ||
      st.includes("vận chuyển") ||
      st.includes("đang giao")
    );
  };

  const getActiveStepIndex = (
    order: Order,
    events: TrackingEvent[],
  ): number => {
    const s = (order.status || "").toUpperCase();
    const ps = (order.payment_status || "").toUpperCase();
    const isCOD = checkIsCOD(order);

    if (s.includes("CANCEL")) return -1;

    const hasReturnEvent = events.some((e) =>
      (e.status || "").toLowerCase().startsWith("return"),
    );
    if (s === "RETURNED" || hasReturnEvent) {
      return 4;
    }

    if (s === "DELIVERED" || s === "COMPLETED" || s === "COMPLETE") {
      return 4;
    }

    const hasDeliveredEvent = events.some((e) => {
      const es = (e.status || "").toLowerCase();
      const est = (e.status_text || "").toLowerCase();
      if (
        es.includes("delivering") ||
        est.includes("out for delivery") ||
        est.includes("đang giao")
      ) {
        return false;
      }
      return (
        es.includes("delivered") ||
        es.includes("complete") ||
        es.includes("received") ||
        est.includes("delivered") ||
        est.includes("đã giao") ||
        est.includes("đã nhận")
      );
    });
    if (hasDeliveredEvent) {
      return 4;
    }

    if (
      s === "SHIPPED" ||
      s === "IN_TRANSIT" ||
      s === "INTRANSIT" ||
      events.some(isShippingEvent)
    ) {
      return 2;
    }

    if (s.includes("CONFIRM")) {
      return 1;
    }

    if (!isCOD && ps === "PAID") {
      return 1;
    }

    return 0;
  };

  const getStepTimestamp = (
    stepIdx: number,
    order: Order,
    events: TrackingEvent[],
  ): string | null => {
    if (stepIdx === 0) {
      const ev = events.find((e) => {
        const s = (e.status || "").toLowerCase();
        return s === "pending" || s.includes("placed") || s.includes("create");
      });
      return ev?.created_at || order.created_at || null;
    }

    if (stepIdx === 1) {
      const isCOD = checkIsCOD(order);
      if (isCOD) {
        const ev = events.find((e) =>
          (e.status || "").toLowerCase().includes("confirm"),
        );
        return ev?.created_at || null;
      } else {
        const ev = events.find((e) => {
          const s = (e.status || "").toLowerCase();
          const st = (e.status_text || "").toLowerCase();
          const n = (e.note || "").toLowerCase();
          return (
            s.includes("paid") ||
            s.includes("payment") ||
            st.includes("paid") ||
            st.includes("payment") ||
            n.includes("paid") ||
            n.includes("thanh toán")
          );
        });
        if (ev?.created_at) return ev.created_at;
        if ((order.payment_status || "").toUpperCase() === "PAID") {
          const confirmEv = events.find((e) =>
            (e.status || "").toLowerCase().includes("confirm"),
          );
          return confirmEv?.created_at || order.created_at || null;
        }
        return null;
      }
    }

    if (stepIdx === 2) {
      const ev = events.find((e) => {
        const s = (e.status || "").toLowerCase();
        const st = (e.status_text || "").toLowerCase();
        return (
          s.includes("ship") ||
          s.includes("transit") ||
          s.includes("pick") ||
          s.includes("delivering") ||
          st.includes("out for delivery") ||
          st.includes("đang giao")
        );
      });
      return ev?.created_at || null;
    }

    if (stepIdx === 3 || stepIdx === 4) {
      const deliveredEv = events.find((e) => {
        const es = (e.status || "").toLowerCase();
        const est = (e.status_text || "").toLowerCase();
        if (
          es.includes("delivering") ||
          est.includes("out for delivery") ||
          est.includes("đang giao")
        ) {
          return false;
        }
        return (
          es.includes("delivered") ||
          es.includes("complete") ||
          es.includes("received") ||
          est.includes("delivered") ||
          est.includes("đã giao") ||
          est.includes("đã nhận")
        );
      });
      const isDeliveredStatus =
        (order.status || "").toUpperCase() === "DELIVERED" ||
        (order.status || "").toUpperCase() === "RETURNED";
      return (
        deliveredEv?.created_at ||
        (isDeliveredStatus
          ? ((order as unknown as Record<string, unknown>)
              .order_delivered_at as string)
          : null) ||
        null
      );
    }

    return null;
  };

  const filterTabs = [
    { key: "ALL", label: "All Orders" },
    { key: "PENDING", label: "Pending Payment" },
    { key: "CONFIRMED", label: "Confirmed" },
    { key: "SHIPPED", label: "In Transit" },
    { key: "DELIVERED", label: "Delivered" },
    { key: "CANCELLED", label: "Cancelled" },
    { key: "RETURNED", label: "Returned" },
  ];

  if (loading) {
    return (
      <div className="p-10 rounded-md bg-white border border-[#E2E8F0] flex items-center justify-center min-h-[360px]">
        <div className="w-8 h-8 rounded-md border-2 border-[#007D9C] border-t-transparent animate-spin" />
      </div>
    );
  }

  // ==========================================
  // VIEW 1: ORDER DETAIL VIEW (MASTER.md COMPLIANT)
  // ==========================================
  if (selectedOrder) {
    const isPending = selectedOrder.status?.toUpperCase() === "PENDING";
    const isConfirmed = selectedOrder.status?.toUpperCase() === "CONFIRMED";
    const isShipped =
      selectedOrder.status?.toUpperCase() === "SHIPPED" ||
      selectedOrder.status?.toUpperCase() === "IN_TRANSIT" ||
      selectedOrder.status?.toUpperCase() === "INTRANSIT";
    const isDelivered = selectedOrder.status?.toUpperCase() === "DELIVERED";
    const isPaid = selectedOrder.payment_status?.toUpperCase() === "PAID";
    const activeStepIndex = getActiveStepIndex(selectedOrder, timelineEvents);
    const isCancelled = activeStepIndex === -1;
    const isCOD = checkIsCOD(selectedOrder);
    const isOrderExpired =
      Boolean(selectedOrder.created_at) &&
      Date.now() - new Date(selectedOrder.created_at).getTime() >
        15 * 60 * 1000;
    const isUnpaidOnline =
      isPending &&
      selectedOrder.payment_status?.toUpperCase() !== "PAID" &&
      !isCOD &&
      !isOrderExpired;

    const returnApprovedEvent = timelineEvents.find(
      (e) => (e.status || "").toLowerCase() === "return_approved",
    );
    const returnRequestedEvent = timelineEvents.find(
      (e) => (e.status || "").toLowerCase() === "return_requested",
    );
    const returnRejectedEvent = timelineEvents.find(
      (e) => (e.status || "").toLowerCase() === "return_rejected",
    );
    const returnCancelledEvent = timelineEvents.find(
      (e) => (e.status || "").toLowerCase() === "return_cancelled",
    );
    const isOrderReturned =
      (selectedOrder.status || "").toUpperCase() === "RETURNED" ||
      timelineEvents.some((e) => (e.status || "").toLowerCase() === "returned");

    const isReturnApproved = Boolean(
      returnApprovedEvent &&
      !isOrderReturned &&
      !returnRejectedEvent &&
      !returnCancelledEvent,
    );
    const isReturnRequested = Boolean(
      returnRequestedEvent &&
      !returnApprovedEvent &&
      !isOrderReturned &&
      !returnRejectedEvent &&
      !returnCancelledEvent,
    );
    const hasReturnRequest = Boolean(
      returnRequestedEvent ||
      returnApprovedEvent ||
      isOrderReturned ||
      timelineEvents.some((e) =>
        (e.status || "").toLowerCase().startsWith("return"),
      ),
    );

    let finalStepLabel = "Completed";
    let finalStepIcon = CheckCircle2;
    if (isOrderReturned) {
      finalStepLabel = "Returned & Refunded";
      finalStepIcon = RotateCcw;
    } else if (isReturnApproved) {
      finalStepLabel = "Return Approved";
      finalStepIcon = ShieldCheck;
    } else if (isReturnRequested) {
      finalStepLabel = "Return Requested";
      finalStepIcon = Clock;
    } else if (returnRejectedEvent) {
      finalStepLabel = "Return Rejected";
      finalStepIcon = Ban;
    }

    const currentSteps = [
      { key: "placed", label: "Order Placed", icon: FileText },
      {
        key: "payment",
        label: isCOD ? "Payment Info Confirmed" : "Order Paid",
        icon: isCOD ? CheckCircle2 : CreditCard,
      },
      { key: "shipped", label: "Order Shipped Out", icon: Truck },
      { key: "received", label: "Order Received", icon: PackageCheck },
      { key: "completed", label: finalStepLabel, icon: finalStepIcon },
    ];

    const copyTrackingCode = () => {
      if (selectedOrder.shipping_code) {
        navigator.clipboard.writeText(selectedOrder.shipping_code);
        toast.success("Tracking number copied to clipboard!");
      }
    };

    return (
      <div className="space-y-6">
        {/* Top Control Bar */}
        <div className="p-4 bg-white border border-[#E2E8F0] rounded-md flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <button
            type="button"
            onClick={handleBackToOrdersList}
            className="inline-flex items-center gap-2 text-xs font-semibold font-heading text-[#64748B] hover:text-[#007D9C] cursor-pointer"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>BACK TO MY ORDERS</span>
          </button>

          <div className="flex items-center flex-wrap gap-3 text-xs">
            <span className="text-[#64748B] font-mono">ORDER ID:</span>
            <span className="font-bold text-[#0F172A] font-heading">
              #
              {selectedOrder.order_number ||
                selectedOrder.uuid.slice(0, 8).toUpperCase()}
            </span>
            <span className="text-[#E2E8F0]">|</span>
            {getPaymentStatusBadge(selectedOrder.payment_status)}
            {getOrderStatusBadge(selectedOrder.status)}

            <button
              type="button"
              onClick={() => handleChatAboutOrder(selectedOrder)}
              className="ml-auto sm:ml-2 px-3 py-1.5 rounded-md bg-[#007D9C] hover:bg-[#007D9C]/90 text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
              title="Chat with support about this order"
            >
              <MessageSquare className="w-3.5 h-3.5 text-[#00ADD8]" />
              <span>Chat About Order</span>
            </button>
          </div>
        </div>

        {/* Return Approved Banner */}
        {isReturnApproved && (
          <div className="p-4 rounded-md bg-[#E6F7FB] border border-[#00ADD8]/40 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
            <div className="flex items-start gap-3">
              <div className="w-8 h-8 rounded-md bg-[#007D9C] text-white flex items-center justify-center shrink-0 mt-0.5">
                <RotateCcw className="w-4 h-4" />
              </div>
              <div className="space-y-0.5">
                <h3 className="font-bold text-[#007D9C] text-sm font-heading">
                  Return & Refund Request Approved
                </h3>
                <p className="text-[#0F172A]">
                  Your return request has been approved. Please pack the item
                  and provide return shipping tracking.
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={() => handleOpenReturnDetails(selectedOrder)}
              className="px-3.5 py-2 rounded-md bg-[#007D9C] hover:bg-[#00637C] text-white text-xs font-semibold flex items-center gap-1.5 shrink-0 self-start sm:self-auto cursor-pointer font-heading"
            >
              <Truck className="w-3.5 h-3.5" />
              <span>Update Return Shipment</span>
            </button>
          </div>
        )}

        {/* 5-STAGE PROCESS BAR (Retained & Styled with MASTER.md) */}
        <div className="p-6 bg-white border border-[#E2E8F0] rounded-md">
          {isCancelled ? (
            <div className="p-4 rounded-md bg-[#FEF2F2] border border-[#FECACA] flex items-start gap-3 text-xs">
              <div className="w-8 h-8 rounded-md bg-[#EF4444] text-white flex items-center justify-center shrink-0 mt-0.5">
                <Ban className="w-4 h-4" />
              </div>
              <div className="space-y-1">
                <h3 className="font-bold text-[#DC2626] text-sm font-heading">
                  Order Cancelled
                </h3>
                <p className="text-slate-700">
                  This order has been cancelled. Any pre-authorized charges will
                  be refunded according to your payment provider’s policy.
                </p>
                <p className="text-[11px] text-[#DC2626] font-medium pt-1">
                  Updated: {formatDateTime(selectedOrder.created_at)}
                </p>
              </div>
            </div>
          ) : (
            <div className="overflow-x-auto pb-2">
              <div className="min-w-[560px] sm:min-w-0">
                <div className="relative flex items-center justify-between">
                  {/* Inactive Track */}
                  <div className="absolute top-5 sm:top-6 -translate-y-1/2 left-[10%] right-[10%] h-1 bg-[#E2E8F0] z-0" />

                  {/* Active Track */}
                  <div
                    className="absolute top-5 sm:top-6 -translate-y-1/2 left-[10%] h-1 bg-[#007D9C] z-0"
                    style={{
                      width: `${
                        (Math.min(activeStepIndex, currentSteps.length - 1) /
                          (currentSteps.length - 1)) *
                        80
                      }%`,
                    }}
                  />

                  {/* Step Nodes */}
                  {currentSteps.map((step, idx) => {
                    const isCompleted = idx < activeStepIndex;
                    const isCurrent = idx === activeStepIndex;
                    const isReached = idx <= activeStepIndex;
                    const timestamp = getStepTimestamp(
                      idx,
                      selectedOrder,
                      timelineEvents,
                    );
                    const Icon = step.icon;

                    return (
                      <div
                        key={step.key}
                        className="flex flex-col items-center text-center relative z-10 flex-1 px-1"
                      >
                        {/* Step Indicator (rounded-md) */}
                        <div
                          className={`w-10 h-10 sm:w-11 sm:h-11 rounded-md flex items-center justify-center border ${
                            isCompleted
                              ? "bg-[#007D9C] text-white border-[#007D9C]"
                              : isCurrent
                                ? "bg-[#00ADD8] text-white border-[#00ADD8] ring-2 ring-[#00ADD8]/30"
                                : "bg-white border-[#E2E8F0] text-[#94A3B8]"
                          }`}
                        >
                          {isCompleted ? (
                            <CheckCheck className="w-5 h-5" />
                          ) : (
                            <Icon className="w-5 h-5" />
                          )}
                        </div>

                        {/* Step Label */}
                        <span
                          className={`mt-2.5 text-xs font-semibold font-heading leading-tight ${
                            isReached ? "text-[#007D9C]" : "text-[#94A3B8]"
                          }`}
                        >
                          {step.label}
                        </span>

                        {/* Timestamp */}
                        {timestamp && (
                          <span className="text-[10px] text-[#64748B] mt-0.5 leading-tight font-mono">
                            {formatDateTime(timestamp)}
                          </span>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Delivery & Tracking Details Card */}
        <div className="grid grid-cols-1 md:grid-cols-12 gap-6">
          {/* Shipping Address */}
          <div className="md:col-span-6 bg-white border border-[#E2E8F0] rounded-md p-5 space-y-3">
            <div className="flex items-center gap-2 text-xs font-bold text-[#0F172A] font-heading">
              <MapPin className="w-4 h-4 text-[#007D9C]" />
              <span>SHIPPING DESTINATION</span>
            </div>
            <div className="text-xs space-y-1 text-slate-700">
              <p className="font-bold text-[#0F172A] text-sm font-heading">
                {selectedOrder.shipping_name || "Alex Nguyen"}
              </p>
              {selectedOrder.shipping_phone && (
                <p className="text-[#64748B]">
                  Phone: {selectedOrder.shipping_phone}
                </p>
              )}
              <p className="leading-relaxed text-slate-600">
                {selectedOrder.shipping_address ||
                  "Standard delivery address on file"}
              </p>
            </div>
          </div>

          {/* Courier & Tracking */}
          <div className="md:col-span-6 bg-white border border-[#E2E8F0] rounded-md p-5 space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-xs font-bold text-[#0F172A] font-heading">
                <Truck className="w-4 h-4 text-[#007D9C]" />
                <span>COURIER & TRACKING</span>
              </div>
              <button
                type="button"
                onClick={() => handleSyncTracking(selectedOrder)}
                disabled={isSyncingTracking}
                className="text-xs font-semibold text-[#007D9C] hover:text-[#00637C] flex items-center gap-1 cursor-pointer disabled:opacity-50 font-heading"
              >
                <RefreshCw
                  className={`w-3.5 h-3.5 ${isSyncingTracking ? "animate-spin" : ""}`}
                />
                <span>Sync Status</span>
              </button>
            </div>

            <div className="text-xs space-y-2">
              <div className="flex items-center justify-between bg-[#F8FAFC] border border-[#E2E8F0] rounded-md p-2.5">
                <div>
                  <span className="text-[#64748B] block text-[11px]">
                    Carrier
                  </span>
                  <span className="font-bold text-[#0F172A] font-heading">
                    {selectedOrder.shipping_provider || "GHN Express"}
                  </span>
                </div>
                {selectedOrder.shipping_code ? (
                  <div className="text-right">
                    <span className="text-[#64748B] block text-[11px]">
                      Tracking No.
                    </span>
                    <div className="flex items-center gap-1.5">
                      <span className="font-mono font-bold text-[#007D9C]">
                        {selectedOrder.shipping_code}
                      </span>
                      <button
                        type="button"
                        onClick={copyTrackingCode}
                        className="text-[#64748B] hover:text-[#007D9C] cursor-pointer"
                        title="Copy tracking code"
                      >
                        <Copy className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ) : (
                  <span className="text-[11px] text-[#64748B]">
                    Tracking pending dispatch
                  </span>
                )}
              </div>

              {(isShipped || isDelivered) && (
                <button
                  type="button"
                  onClick={() => handleOpenTrackingModal(selectedOrder)}
                  className="w-full py-2 rounded-md bg-[#E6F7FB] hover:bg-[#d0f1f9] text-[#007D9C] text-xs font-semibold flex items-center justify-center gap-1.5 border border-[#00ADD8]/30 cursor-pointer font-heading"
                >
                  <Truck className="w-3.5 h-3.5" />
                  <span>View Full Tracking Timeline</span>
                </button>
              )}
            </div>
          </div>
        </div>

        {/* Order Items Table */}
        <div className="bg-white border border-[#E2E8F0] rounded-md overflow-hidden">
          <div className="p-4 bg-[#F8FAFC] border-b border-[#E2E8F0] flex items-center justify-between">
            <h3 className="text-xs font-bold text-[#0F172A] font-heading uppercase tracking-wide">
              Items In Order ({selectedOrder.items?.length || 0})
            </h3>
            <span className="text-xs text-[#64748B]">
              100% Genuine Tech Gear
            </span>
          </div>

          <div className="divide-y divide-[#E2E8F0]">
            {selectedOrder.items && selectedOrder.items.length > 0 ? (
              selectedOrder.items.map((item, idx) => (
                <div
                  key={item.uuid || `item-${idx}`}
                  className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4"
                >
                  <div className="flex items-center gap-4 min-w-0">
                    <div className="w-16 h-16 rounded-md border border-[#E2E8F0] bg-[#F8FAFC] overflow-hidden shrink-0 flex items-center justify-center">
                      {item.image_url ? (
                        <img
                          src={getImageUrl(item.image_url)}
                          alt={item.product_name}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <Package className="w-6 h-6 text-slate-400" />
                      )}
                    </div>
                    <div className="min-w-0">
                      <h4 className="font-bold text-sm text-[#0F172A] truncate font-heading">
                        {item.product_name}
                      </h4>
                      <div className="flex items-center gap-3 text-xs text-[#64748B] mt-1">
                        <span>
                          Qty:{" "}
                          <strong className="text-[#0F172A]">
                            {item.quantity}
                          </strong>
                        </span>
                        <span>•</span>
                        <span>Unit: {formatCurrency(item.price)}</span>
                      </div>
                    </div>
                  </div>

                  <div className="flex sm:flex-col items-center sm:items-end justify-between sm:justify-center gap-2 shrink-0 pt-2 sm:pt-0 border-t sm:border-t-0 border-[#E2E8F0]">
                    <div className="text-right">
                      <span className="text-xs text-[#64748B] sm:hidden mr-2">
                        Subtotal:
                      </span>
                      <span className="text-sm font-bold text-[#007D9C] font-heading">
                        {formatCurrency(
                          item.subtotal || item.price * item.quantity,
                        )}
                      </span>
                    </div>

                    {isDelivered && (
                      <button
                        type="button"
                        onClick={() =>
                          handleOpenReviewItem(selectedOrder, item)
                        }
                        className="px-3 py-1.5 rounded-md bg-[#FFFBEB] hover:bg-[#FEF3C7] text-[#D97706] border border-[#FDE68A] text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
                      >
                        <Star className="w-3.5 h-3.5 fill-[#D97706] text-[#D97706]" />
                        <span>Review Product</span>
                      </button>
                    )}
                  </div>
                </div>
              ))
            ) : (
              <div className="p-8 text-center text-xs text-[#64748B]">
                No items found for this order.
              </div>
            )}
          </div>

          {/* Financial Breakdown */}
          <div className="p-5 bg-[#F8FAFC] border-t border-[#E2E8F0]">
            <div className="max-w-md ml-auto space-y-2 text-xs">
              <div className="flex justify-between text-slate-600">
                <span>Merchandise Subtotal</span>
                <span className="font-semibold text-[#0F172A]">
                  {formatCurrency(
                    selectedOrder.subtotal_amount || selectedOrder.total_amount,
                  )}
                </span>
              </div>

              {selectedOrder.discount_amount !== undefined &&
                selectedOrder.discount_amount > 0 && (
                  <div className="flex justify-between text-[#059669] font-medium">
                    <span className="flex items-center gap-1">
                      <Tag className="w-3.5 h-3.5" />
                      Voucher Discount{" "}
                      {selectedOrder.coupon_code
                        ? `(${selectedOrder.coupon_code})`
                        : ""}
                    </span>
                    <span>
                      -{formatCurrency(selectedOrder.discount_amount)}
                    </span>
                  </div>
                )}

              {selectedOrder.shipping_fee !== undefined && (
                <div className="flex justify-between text-slate-600">
                  <span>Shipping Fee (GHN Express)</span>
                  <span className="font-semibold text-[#0F172A]">
                    {selectedOrder.shipping_fee === 0
                      ? "FREE"
                      : formatCurrency(selectedOrder.shipping_fee)}
                  </span>
                </div>
              )}

              <div className="pt-3 border-t border-[#E2E8F0] flex justify-between items-baseline">
                <div>
                  <span className="text-sm font-bold text-[#0F172A] font-heading block">
                    Order Total
                  </span>
                  <span className="text-[11px] text-[#64748B]">
                    Payment via{" "}
                    {selectedOrder.payment_method || "Online Payment"}
                  </span>
                </div>
                <span className="text-xl font-bold text-[#007D9C] font-heading">
                  {formatCurrency(selectedOrder.total_amount)}
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Action Controls Bar */}
        <div className="p-4 bg-white border border-[#E2E8F0] rounded-md flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <button
            type="button"
            onClick={handleBackToOrdersList}
            className="inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] text-[#0F172A] text-xs font-semibold cursor-pointer font-heading"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to Orders List</span>
          </button>

          <div className="flex items-center gap-2.5 flex-wrap justify-end">
            {(isPaid || isDelivered) && (
              <button
                type="button"
                disabled={downloadingInvoiceUuid === selectedOrder.uuid}
                onClick={() => handleDownloadInvoice(selectedOrder)}
                className="px-4 py-2.5 rounded-md bg-white hover:bg-[#F8FAFC] text-[#0F172A] border border-[#E2E8F0] text-xs font-semibold flex items-center gap-1.5 cursor-pointer disabled:opacity-50 font-heading"
              >
                {downloadingInvoiceUuid === selectedOrder.uuid ? (
                  <RefreshCw className="w-3.5 h-3.5 animate-spin text-[#007D9C]" />
                ) : (
                  <Download className="w-3.5 h-3.5" />
                )}
                <span>Invoice PDF</span>
              </button>
            )}

            {isUnpaidOnline && (
              <button
                type="button"
                disabled={payingOrderUuid === selectedOrder.uuid}
                onClick={() => handlePayNow(selectedOrder)}
                className="px-5 py-2.5 rounded-md bg-[#F26522] hover:bg-[#D85314] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer disabled:opacity-50 font-heading"
              >
                {payingOrderUuid === selectedOrder.uuid ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <CreditCard className="w-4 h-4" />
                )}
                <span>Pay Now (MoMo)</span>
              </button>
            )}

            {isShipped && (
              <button
                type="button"
                onClick={() => handleOpenConfirmDelivery(selectedOrder)}
                className="px-5 py-2.5 rounded-md bg-[#059669] hover:bg-[#047857] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
              >
                <PackageCheck className="w-4 h-4" />
                <span>Confirm Delivery</span>
              </button>
            )}

            {isPending && (
              <button
                type="button"
                onClick={() => handleOpenCancelConfirmation(selectedOrder)}
                className="px-5 py-2.5 rounded-md bg-[#FEF2F2] hover:bg-[#FEE2E2] text-[#DC2626] border border-[#FECACA] text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
              >
                <Ban className="w-4 h-4" />
                <span>Cancel Order</span>
              </button>
            )}

            {hasReturnRequest && (
              <button
                type="button"
                onClick={() => handleOpenReturnDetails(selectedOrder)}
                className="px-5 py-2.5 rounded-md bg-[#007D9C] hover:bg-[#00637C] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
              >
                <Truck className="w-4 h-4" />
                <span>Update Return Shipment</span>
              </button>
            )}

            {isDelivered && !hasReturnRequest && (
              <button
                type="button"
                onClick={() => handleOpenReturnModal(selectedOrder)}
                className="px-5 py-2.5 rounded-md bg-[#FFFBEB] hover:bg-[#FEF3C7] text-[#D97706] border border-[#FDE68A] text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
              >
                <RotateCcw className="w-4 h-4" />
                <span>Return / Refund</span>
              </button>
            )}

            <Link
              href="/products"
              className="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-md bg-[#007D9C] hover:bg-[#00637C] text-white text-xs font-semibold font-heading"
            >
              <span>Explore Products</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
          </div>
        </div>

        {renderActionModals()}
      </div>
    );
  }

  // ==========================================
  // VIEW 2: ORDER HISTORY LIST VIEW (WITHOUT PROCESS BAR)
  // ==========================================
  return (
    <div className="space-y-6 font-body">
      {/* Page Header */}
      <div className="bg-white border border-[#E2E8F0] rounded-md p-6">
        <h1 className="font-bold text-2xl text-[#0F172A] tracking-tight font-heading">
          My Orders
        </h1>
        <p className="text-sm text-[#64748B] mt-1">
          Track, manage, and view detailed invoices for your hardware &
          merchandise orders.
        </p>
      </div>

      {/* Filter Tabs Bar */}
      <div className="flex flex-wrap gap-2 border-b border-[#E2E8F0] pb-3 text-xs font-semibold font-heading">
        {filterTabs.map((tab) => {
          const isActive = filter === tab.key;
          return (
            <button
              key={tab.key}
              type="button"
              onClick={() => setFilter(tab.key)}
              className={`px-3.5 py-2 rounded-md border cursor-pointer ${
                isActive
                  ? "bg-[#E6F7FB] text-[#007D9C] border-[#00ADD8]/40"
                  : "bg-white text-[#64748B] hover:text-[#0F172A] border-[#E2E8F0]"
              }`}
            >
              {tab.label}
              {tab.key === "ALL" && pagination.total_records > 0
                ? ` (${pagination.total_records})`
                : ""}
            </button>
          );
        })}
      </div>

      {/* Search & Filter Toolbar */}
      <div className="bg-white border border-[#E2E8F0] rounded-md p-4 flex flex-wrap items-center justify-between gap-4">
        <div className="relative flex-1 min-w-[240px]">
          <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-[#94A3B8]">
            {isSearching ? (
              <RefreshCw className="w-4 h-4 text-[#007D9C] animate-spin" />
            ) : (
              <Search className="w-4 h-4" />
            )}
          </div>
          <input
            type="text"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search by Order ID or Product name..."
            className="w-full pl-9 pr-8 py-2 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] text-xs text-[#0F172A] placeholder-[#94A3B8] focus:outline-none focus:border-[#007D9C]"
          />
          {searchInput && (
            <button
              type="button"
              onClick={() => setSearchInput("")}
              className="absolute inset-y-0 right-0 pr-2.5 flex items-center text-[#94A3B8] hover:text-[#0F172A] cursor-pointer"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>

        <div className="flex items-center gap-3">
          {(filter !== "ALL" || searchInput.trim() !== "") && (
            <button
              type="button"
              onClick={() => {
                setFilter("ALL");
                setSearchInput("");
              }}
              className="px-3 py-2 rounded-md bg-[#F8FAFC] hover:bg-slate-200 border border-[#E2E8F0] text-xs font-semibold text-[#64748B] cursor-pointer font-heading"
            >
              Clear Filters
            </button>
          )}
        </div>
      </div>

      {/* Orders List Content */}
      {orders.length === 0 ? (
        <div className="p-12 text-center border border-dashed border-[#CBD5E1] bg-white rounded-md space-y-3">
          <div className="w-14 h-14 rounded-md bg-[#E6F7FB] text-[#007D9C] flex items-center justify-center mx-auto">
            <ShoppingBag className="w-7 h-7" />
          </div>
          <h3 className="text-sm font-bold text-[#0F172A] font-heading">
            No orders found
          </h3>
          <p className="text-xs text-[#64748B] max-w-sm mx-auto">
            {debouncedSearch.trim()
              ? `No orders matching "${debouncedSearch.trim()}".`
              : "You haven't placed any orders matching this filter."}
          </p>
          <div className="pt-2">
            <Link
              href="/products"
              className="inline-flex items-center gap-1.5 px-4 py-2 rounded-md bg-[#007D9C] text-white text-xs font-semibold hover:bg-[#00637C] font-heading"
            >
              <span>Explore Gear Catalog</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          </div>
        </div>
      ) : (
        <div className="space-y-4">
          {orders.map((order) => {
            const isPending = order.status?.toUpperCase() === "PENDING";
            const isShipped =
              order.status?.toUpperCase() === "SHIPPED" ||
              order.status?.toUpperCase() === "IN_TRANSIT" ||
              order.status?.toUpperCase() === "INTRANSIT";
            const isDelivered = order.status?.toUpperCase() === "DELIVERED";
            const isPaid = order.payment_status?.toUpperCase() === "PAID";
            const isOrderExpired =
              Boolean(order.created_at) &&
              Date.now() - new Date(order.created_at).getTime() >
                15 * 60 * 1000;
            const isUnpaidOnline =
              isPending &&
              order.payment_status?.toUpperCase() !== "PAID" &&
              !checkIsCOD(order) &&
              !isOrderExpired;

            return (
              <div
                key={order.uuid}
                onClick={() => handleOpenOrderDetails(order)}
                className="bg-white border border-[#E2E8F0] hover:border-[#007D9C]/50 rounded-md overflow-hidden cursor-pointer"
              >
                {/* Header Row: Order ID, Date, Badges */}
                <div className="p-4 bg-[#F8FAFC] border-b border-[#E2E8F0] flex flex-wrap items-center justify-between gap-4">
                  <div className="flex flex-wrap items-center gap-4 text-xs">
                    <div>
                      <span className="text-[#64748B] block text-[11px]">
                        Order ID
                      </span>
                      <span className="font-bold text-[#0F172A] font-mono text-sm">
                        #
                        {order.order_number ||
                          order.uuid.slice(0, 8).toUpperCase()}
                      </span>
                    </div>
                    <div className="h-6 w-px bg-[#E2E8F0]" />
                    <div>
                      <span className="text-[#64748B] block text-[11px]">
                        Placed On
                      </span>
                      <span className="font-semibold text-[#0F172A] font-heading">
                        {formatDate(order.created_at)}
                      </span>
                    </div>
                    {order.shipping_address && (
                      <>
                        <div className="h-6 w-px bg-[#E2E8F0] hidden sm:block" />
                        <div className="hidden sm:block">
                          <span className="text-[#64748B] block text-[11px]">
                            Destination
                          </span>
                          <span className="text-slate-700 truncate max-w-xs block">
                            {order.shipping_address}
                          </span>
                        </div>
                      </>
                    )}
                  </div>

                  <div className="flex items-center gap-2 flex-wrap">
                    {getPaymentStatusBadge(order.payment_status)}
                    {getOrderStatusBadge(order.status)}
                  </div>
                </div>

                {/* Items Summary Row */}
                <div className="p-5">
                  <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
                    <div className="flex items-center gap-4 min-w-0">
                      <div className="w-16 h-16 rounded-md border border-[#E2E8F0] bg-[#F8FAFC] overflow-hidden shrink-0 flex items-center justify-center">
                        {order.items && order.items[0]?.image_url ? (
                          <img
                            src={getImageUrl(order.items[0].image_url)}
                            alt={order.items[0].product_name}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <Package className="w-6 h-6 text-slate-400" />
                        )}
                      </div>
                      <div className="min-w-0">
                        <h4 className="font-bold text-sm text-[#0F172A] truncate font-heading">
                          {order.items && order.items.length > 0
                            ? order.items[0].product_name
                            : "Standard Hardware Shipment"}
                        </h4>
                        <p className="text-xs text-[#64748B] mt-0.5">
                          {order.items && order.items.length > 1
                            ? `+ ${order.items.length - 1} other item(s) in package`
                            : `Quantity: ${order.items?.[0]?.quantity || 1}`}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center md:flex-col md:items-end justify-between md:justify-center gap-1 shrink-0 pt-2 md:pt-0 border-t md:border-t-0 border-[#E2E8F0]">
                      <span className="text-xs text-[#64748B]">
                        Total Amount:
                      </span>
                      <span className="text-base font-bold text-[#007D9C] font-heading">
                        {formatCurrency(order.total_amount)}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Footer Action Row */}
                <div className="p-3.5 bg-[#F8FAFC] border-t border-[#E2E8F0] flex flex-wrap items-center justify-between gap-3">
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleChatAboutOrder(order);
                      }}
                      className="px-3 py-1.5 rounded-md bg-white hover:bg-slate-100 border border-[#E2E8F0] text-slate-700 text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
                      title="Chat about order"
                    >
                      <MessageSquare className="w-3.5 h-3.5 text-slate-600" />
                      <span>Chat</span>
                    </button>

                    {(isPaid || isDelivered) && (
                      <button
                        type="button"
                        disabled={downloadingInvoiceUuid === order.uuid}
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDownloadInvoice(order);
                        }}
                        className="px-3 py-1.5 rounded-md bg-white hover:bg-slate-100 border border-[#E2E8F0] text-slate-700 text-xs font-semibold flex items-center gap-1.5 cursor-pointer disabled:opacity-50 font-heading"
                      >
                        {downloadingInvoiceUuid === order.uuid ? (
                          <RefreshCw className="w-3.5 h-3.5 animate-spin text-[#007D9C]" />
                        ) : (
                          <Download className="w-3.5 h-3.5" />
                        )}
                        <span>Invoice</span>
                      </button>
                    )}
                  </div>

                  <div className="flex items-center gap-2 flex-wrap">
                    {isUnpaidOnline && (
                      <button
                        type="button"
                        disabled={payingOrderUuid === order.uuid}
                        onClick={(e) => {
                          e.stopPropagation();
                          handlePayNow(order);
                        }}
                        className="px-3.5 py-1.5 rounded-md bg-[#F26522] hover:bg-[#D85314] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer disabled:opacity-50 font-heading"
                      >
                        {payingOrderUuid === order.uuid ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <CreditCard className="w-3.5 h-3.5" />
                        )}
                        <span>Pay Now</span>
                      </button>
                    )}

                    {isShipped && (
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOpenConfirmDelivery(order);
                        }}
                        className="px-3.5 py-1.5 rounded-md bg-[#059669] hover:bg-[#047857] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
                      >
                        <PackageCheck className="w-3.5 h-3.5" />
                        <span>Confirm Delivery</span>
                      </button>
                    )}

                    {(isShipped || isDelivered) && (
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOpenTrackingModal(order);
                        }}
                        className="px-3.5 py-1.5 rounded-md bg-white hover:bg-[#E6F7FB] text-[#007D9C] border border-[#007D9C] text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
                      >
                        <Truck className="w-3.5 h-3.5" />
                        <span>Track</span>
                      </button>
                    )}

                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleOpenOrderDetails(order);
                      }}
                      className="px-4 py-1.5 rounded-md bg-[#007D9C] hover:bg-[#00637C] text-white text-xs font-semibold flex items-center gap-1.5 cursor-pointer font-heading"
                    >
                      <span>Order Details</span>
                      <Eye className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Pagination Controls */}
      {orders.length > 0 && (
        <div className="p-4 bg-white border border-[#E2E8F0] rounded-md flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-3 text-xs text-[#64748B] font-medium flex-wrap justify-center sm:justify-start">
            <span>
              Showing{" "}
              <strong className="text-[#0F172A]">
                {Math.min(
                  (pagination.page - 1) * pagination.limit + 1,
                  pagination.total_records || orders.length,
                )}
              </strong>{" "}
              to{" "}
              <strong className="text-[#0F172A]">
                {Math.min(
                  pagination.page * pagination.limit,
                  pagination.total_records || orders.length,
                )}
              </strong>{" "}
              of{" "}
              <strong className="text-[#0F172A]">
                {pagination.total_records || orders.length}
              </strong>{" "}
              orders
            </span>

            <div className="flex items-center gap-1.5 pl-2 border-l border-[#E2E8F0]">
              <span className="text-[#64748B]">Show:</span>
              <select
                value={pageSize}
                onChange={(e) => {
                  setPageSize(Number(e.target.value));
                  setCurrentPage(1);
                }}
                className="px-2 py-1 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] text-xs font-semibold text-[#0F172A] focus:outline-none focus:border-[#007D9C] cursor-pointer"
              >
                <option value={10}>10 / page</option>
                <option value={20}>20 / page</option>
                <option value={50}>50 / page</option>
                <option value={100}>100 / page</option>
              </select>
            </div>
          </div>

          <div className="flex items-center gap-1.5 flex-wrap justify-center font-heading">
            <button
              type="button"
              disabled={currentPage <= 1 || loading}
              onClick={() => setCurrentPage(1)}
              className="h-8 w-8 rounded-md border border-[#E2E8F0] bg-white text-slate-600 hover:bg-[#F8FAFC] disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center cursor-pointer"
              title="First Page"
            >
              <ChevronsLeft className="w-4 h-4" />
            </button>

            <button
              type="button"
              disabled={currentPage <= 1 || loading}
              onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
              className="h-8 w-8 rounded-md border border-[#E2E8F0] bg-white text-slate-600 hover:bg-[#F8FAFC] disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center cursor-pointer"
              title="Previous Page"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>

            {generatePaginationPages(currentPage, pagination.total_pages).map(
              (p, idx) => {
                if (p === "...") {
                  return (
                    <span
                      key={`ellipsis-${idx}`}
                      className="w-8 h-8 flex items-center justify-center text-[#94A3B8] text-xs font-semibold select-none"
                    >
                      ...
                    </span>
                  );
                }

                const pageNum = Number(p);
                const isActive = pageNum === currentPage;

                return (
                  <button
                    key={`page-${pageNum}`}
                    type="button"
                    disabled={loading}
                    onClick={() => setCurrentPage(pageNum)}
                    className={`h-8 min-w-8 px-2.5 rounded-md text-xs font-bold flex items-center justify-center cursor-pointer ${
                      isActive
                        ? "bg-[#007D9C] text-white"
                        : "border border-[#E2E8F0] bg-white text-[#0F172A] hover:bg-[#F8FAFC]"
                    }`}
                  >
                    {pageNum}
                  </button>
                );
              },
            )}

            <button
              type="button"
              disabled={currentPage >= pagination.total_pages || loading}
              onClick={() =>
                setCurrentPage((p) => Math.min(pagination.total_pages, p + 1))
              }
              className="h-8 w-8 rounded-md border border-[#E2E8F0] bg-white text-slate-600 hover:bg-[#F8FAFC] disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center cursor-pointer"
              title="Next Page"
            >
              <ChevronRight className="w-4 h-4" />
            </button>

            <button
              type="button"
              disabled={currentPage >= pagination.total_pages || loading}
              onClick={() => setCurrentPage(pagination.total_pages)}
              className="h-8 w-8 rounded-md border border-[#E2E8F0] bg-white text-slate-600 hover:bg-[#F8FAFC] disabled:opacity-35 disabled:cursor-not-allowed flex items-center justify-center cursor-pointer"
              title="Last Page"
            >
              <ChevronsRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {renderActionModals()}
    </div>
  );

  // ==========================================
  // ACTION MODALS
  // ==========================================
  function renderActionModals() {
    return (
      <>
        {/* Cancel Confirmation Dialog */}
        {orderToCancel && (
          <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/60 p-4 overflow-y-auto">
            <div className="w-full max-w-md bg-white rounded-md p-6 space-y-4 border border-[#E2E8F0]">
              <div className="w-12 h-12 rounded-md bg-[#FEF2F2] text-[#DC2626] flex items-center justify-center mx-auto">
                <AlertTriangle className="w-6 h-6" />
              </div>

              <div className="text-center space-y-1">
                <h3 className="text-base font-bold text-[#0F172A] font-heading">
                  Cancel Order Confirmation
                </h3>
                <p className="text-xs text-[#64748B]">
                  Are you sure you want to cancel order{" "}
                  <strong className="text-[#0F172A]">
                    #
                    {orderToCancel.order_number ||
                      orderToCancel.uuid.slice(0, 8).toUpperCase()}
                  </strong>
                  ? This action cannot be undone.
                </p>
              </div>

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  disabled={isCancelling}
                  onClick={() => setOrderToCancel(null)}
                  className="flex-1 py-2.5 rounded-md bg-[#F8FAFC] hover:bg-slate-200 border border-[#E2E8F0] text-[#0F172A] text-xs font-semibold cursor-pointer font-heading"
                >
                  Keep Order
                </button>
                <button
                  type="button"
                  disabled={isCancelling}
                  onClick={handleConfirmCancel}
                  className="flex-1 py-2.5 rounded-md bg-[#DC2626] hover:bg-red-700 text-white text-xs font-semibold flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50 font-heading"
                >
                  {isCancelling ? (
                    <>
                      <div className="w-3.5 h-3.5 rounded-md border-2 border-white border-t-transparent animate-spin" />
                      <span>Cancelling...</span>
                    </>
                  ) : (
                    <>
                      <Ban className="w-3.5 h-3.5" />
                      <span>Yes, Cancel</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Return / Refund Request Modal */}
        {orderToReturn && (
          <RequestReturnModal
            isOpen={!!orderToReturn}
            onClose={() => setOrderToReturn(null)}
            orderUuid={orderToReturn.uuid}
            orderNumber={orderToReturn.order_number}
            paymentMethod={orderToReturn.payment_method}
            totalAmount={orderToReturn.total_amount}
            onSuccess={() => {
              setOrderToReturn(null);
              fetchOrders();
              if (selectedOrder && selectedOrder.uuid === orderToReturn.uuid) {
                handleOpenOrderDetails(orderToReturn, false);
              }
            }}
          />
        )}

        {/* Return Details Modal */}
        {orderToViewReturn && (
          <ReturnDetailModal
            isOpen={!!orderToViewReturn}
            onClose={() => setOrderToViewReturn(null)}
            orderUuid={orderToViewReturn.uuid}
            onUpdated={() => {
              fetchOrders();
              if (
                selectedOrder &&
                selectedOrder.uuid === orderToViewReturn.uuid
              ) {
                handleOpenOrderDetails(orderToViewReturn, false);
              }
            }}
          />
        )}

        {/* Order Tracking Modal */}
        {orderToTrack && (
          <OrderTrackingModal
            isOpen={!!orderToTrack}
            onClose={() => setOrderToTrack(null)}
            orderUuid={orderToTrack.uuid}
            orderNumber={orderToTrack.order_number}
            orderStatus={orderToTrack.status}
            onStatusUpdated={() => {
              fetchOrders();
              if (selectedOrder && selectedOrder.uuid === orderToTrack.uuid) {
                handleOpenOrderDetails(orderToTrack, false);
              }
            }}
          />
        )}

        {/* Confirm Delivery Dialog */}
        {orderToConfirmDelivery && (
          <div className="fixed inset-0 z-60 flex items-center justify-center bg-black/60 p-4 overflow-y-auto">
            <div className="w-full max-w-md bg-white rounded-md p-6 space-y-4 border border-[#E2E8F0]">
              <div className="w-12 h-12 rounded-md bg-[#ECFDF5] text-[#059669] flex items-center justify-center mx-auto">
                <PackageCheck className="w-6 h-6" />
              </div>

              <div className="text-center space-y-1">
                <h3 className="text-base font-bold text-[#0F172A] font-heading">
                  Confirm Order Delivery
                </h3>
                <p className="text-xs text-[#64748B] leading-relaxed">
                  Have you received all items for order{" "}
                  <strong className="text-[#0F172A]">
                    #
                    {orderToConfirmDelivery.order_number ||
                      orderToConfirmDelivery.uuid.slice(0, 8).toUpperCase()}
                  </strong>{" "}
                  in good condition?
                </p>
              </div>

              <div className="p-3 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] text-[11px] text-slate-600 flex items-start gap-2">
                <ShieldCheck className="w-4 h-4 text-[#059669] shrink-0 mt-0.5" />
                <span>
                  Confirming delivery completes this order and unlocks product
                  reviews and official sales invoices.
                </span>
              </div>

              <div className="flex gap-2 pt-2">
                <button
                  type="button"
                  disabled={isConfirmingDelivery}
                  onClick={() => setOrderToConfirmDelivery(null)}
                  className="flex-1 py-2.5 rounded-md bg-[#F8FAFC] hover:bg-slate-200 border border-[#E2E8F0] text-[#0F172A] text-xs font-semibold cursor-pointer font-heading"
                >
                  Not Yet
                </button>
                <button
                  type="button"
                  disabled={isConfirmingDelivery}
                  onClick={handleConfirmDelivery}
                  className="flex-1 py-2.5 rounded-md bg-[#059669] hover:bg-[#047857] text-white text-xs font-semibold flex items-center justify-center gap-1.5 cursor-pointer disabled:opacity-50 font-heading"
                >
                  {isConfirmingDelivery ? (
                    <>
                      <div className="w-3.5 h-3.5 rounded-md border-2 border-white border-t-transparent animate-spin" />
                      <span>Confirming...</span>
                    </>
                  ) : (
                    <>
                      <PackageCheck className="w-3.5 h-3.5" />
                      <span>Yes, I Received It</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Write Review Modal */}
        {reviewingItem && (
          <WriteReviewModal
            isOpen={!!reviewingItem}
            onClose={() => setReviewingItem(null)}
            orderUuid={reviewingItem.orderUuid}
            productUuid={reviewingItem.productUuid}
            productName={reviewingItem.productName}
            productImageUrl={reviewingItem.productImageUrl}
            onSuccess={() => {
              setReviewingItem(null);
              fetchOrders();
            }}
          />
        )}
      </>
    );
  }
}

export default function MyOrdersPage() {
  return (
    <Suspense
      fallback={
        <div className="p-12 text-center text-[#64748B] space-y-2 bg-white border border-[#E2E8F0] rounded-md">
          <RefreshCw className="w-6 h-6 animate-spin mx-auto text-[#007D9C]" />
          <p className="text-xs font-semibold font-heading">
            Loading orders...
          </p>
        </div>
      }
    >
      <MyOrdersPageContent />
    </Suspense>
  );
}
