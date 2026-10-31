"use client";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import Link from "next/link";
import { authFetch } from "@/lib/api";
import { getImageUrl, extractErrorMessage } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";
import { useMounted } from "@/hooks/use-mounted";
import { WriteReviewModal } from "@/components/review/write-review-modal";
import {
  Star,
  MessageSquare,
  ShoppingBag,
  ExternalLink,
  Edit3,
  Trash2,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Loader2,
  AlertTriangle,
  CheckCircle2,
  X,
  PlusCircle,
  Camera,
} from "lucide-react";
import { toast } from "sonner";

export interface MyReview {
  uuid: string;
  product_uuid: string;
  order_uuid?: string;
  product_name: string;
  product_slug: string;
  product_image_url?: string;
  rating: number;
  title?: string;
  comment?: string;
  images: string[];
  video_url?: string;
  has_media: boolean;
  can_edit: boolean;
  created_at: string;
}

export interface PendingReviewItem {
  orderUuid: string;
  orderNumber: string;
  orderDate: string;
  productUuid: string;
  productName: string;
  productSlug?: string;
  productImageUrl?: string;
}

interface OrderItemApi {
  product_uuid?: string;
  product_name: string;
  image_url?: string;
}

interface OrderApi {
  uuid: string;
  order_number: string;
  created_at: string;
  status: string | number;
  items?: OrderItemApi[];
}

export default function MyReviewsPage() {
  const { accessToken } = useAuthStore();
  const mounted = useMounted();

  const [reviews, setReviews] = useState<MyReview[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [totalRecords, setTotalRecords] = useState(0);
  const limit = 10;

  // Pending Reviews from delivered orders
  const [pendingItems, setPendingItems] = useState<PendingReviewItem[]>([]);
  const [loadingOrders, setLoadingOrders] = useState(false);

  // Active sub-tab state ('reviewed' | 'pending')
  const [activeTab, setActiveTab] = useState<"reviewed" | "pending">(
    "reviewed",
  );

  // Write Review Modal State (from To Review tab)
  const [writingItem, setWritingItem] = useState<PendingReviewItem | null>(
    null,
  );

  // Edit Modal State
  const [editingReview, setEditingReview] = useState<MyReview | null>(null);
  const [editRating, setEditRating] = useState(5);
  const [editHoverRating, setEditHoverRating] = useState(0);
  const [editTitle, setEditTitle] = useState("");
  const [editComment, setEditComment] = useState("");
  const [isUpdating, setIsUpdating] = useState(false);

  // Delete Confirmation State
  const [deletingReview, setDeletingReview] = useState<MyReview | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  // Lightbox Preview State
  const [previewMediaUrl, setPreviewMediaUrl] = useState<string | null>(null);

  const fetchMyReviews = useCallback(
    async (page: number) => {
      if (!accessToken) return;
      setLoading(true);
      try {
        const baseUrl =
          process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
        const res = await authFetch(
          `${baseUrl}/api/v1/reviews/me?page=${page}&limit=${limit}`,
        );
        if (res.ok) {
          const json = await res.json();
          const data: MyReview[] = json?.data || [];
          setReviews(data);

          const pagination = json?.pagination;
          if (pagination) {
            setTotalPages(pagination.total_pages || 1);
            setTotalRecords(pagination.total_records || data.length);
          } else {
            setTotalRecords(data.length);
            setTotalPages(Math.ceil(data.length / limit) || 1);
          }
        } else {
          const errJson = await res.json().catch(() => null);
          toast.error(
            extractErrorMessage(errJson, "Failed to fetch your reviews."),
          );
        }
      } catch {
        toast.error("Network error loading your reviews.");
      } finally {
        setLoading(false);
      }
    },
    [accessToken, limit],
  );

  const fetchDeliveredOrders = useCallback(async () => {
    if (!accessToken) return;
    setLoadingOrders(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      // Fetch delivered orders
      const res = await authFetch(`${baseUrl}/api/v1/orders?limit=50`);
      if (res.ok) {
        const json = await res.json();
        const orders: OrderApi[] = json?.data || [];

        // Filter delivered / completed orders
        const delivered = orders.filter((o) => {
          const statusStr = String(o.status).toUpperCase();
          return (
            statusStr === "DELIVERED" ||
            statusStr === "4" ||
            statusStr === "COMPLETED"
          );
        });

        // Collect all items from delivered orders
        const items: PendingReviewItem[] = [];
        delivered.forEach((order) => {
          (order.items || []).forEach((item) => {
            if (item.product_uuid) {
              items.push({
                orderUuid: order.uuid,
                orderNumber: order.order_number,
                orderDate: order.created_at,
                productUuid: item.product_uuid,
                productName: item.product_name,
                productImageUrl: item.image_url,
              });
            }
          });
        });

        setPendingItems(items);
      }
    } catch {
      // Non-blocking error
    } finally {
      setLoadingOrders(false);
    }
  }, [accessToken]);

  useEffect(() => {
    if (mounted && accessToken) {
      fetchMyReviews(currentPage);
      fetchDeliveredOrders();
    }
  }, [mounted, accessToken, currentPage, fetchMyReviews, fetchDeliveredOrders]);

  // Filter out pending items that already have a review.
  // Backend allows one review per (order, product) pair, so dedupe on the pair,
  // not product alone — the same product can sit in two delivered orders.
  const unreviewedPendingItems = useMemo(() => {
    const reviewedKeys = new Set(
      reviews.map((r) => `${r.order_uuid || ""}:${r.product_uuid}`),
    );
    return pendingItems.filter(
      (item) => !reviewedKeys.has(`${item.orderUuid}:${item.productUuid}`),
    );
  }, [pendingItems, reviews]);

  // Real Calculated Metrics
  const avgRatingNumber = useMemo(() => {
    if (reviews.length === 0) return 0;
    const sum = reviews.reduce((acc, r) => acc + (r.rating || 0), 0);
    return Number((sum / reviews.length).toFixed(1));
  }, [reviews]);

  // Open Edit Modal
  const handleOpenEdit = (rev: MyReview) => {
    if (!rev.can_edit) {
      toast.error(
        "This review cannot be edited because the 30-day editing window has expired.",
      );
      return;
    }
    setEditingReview(rev);
    setEditRating(rev.rating);
    setEditTitle(rev.title || "");
    setEditComment(rev.comment || "");
  };

  // Submit Edit
  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingReview) return;

    setIsUpdating(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const payload: { rating: number; title?: string; comment?: string } = {
        rating: editRating,
      };
      if (editTitle.trim()) payload.title = editTitle.trim();
      if (editComment.trim()) payload.comment = editComment.trim();

      const res = await authFetch(
        `${baseUrl}/api/v1/reviews/${editingReview.uuid}`,
        {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(payload),
        },
      );

      if (res.ok) {
        toast.success("Your review has been updated successfully!");
        setEditingReview(null);
        fetchMyReviews(currentPage);
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(errJson, "Failed to update review.");
        toast.error(msg);
      }
    } catch {
      toast.error("Network error while saving review.");
    } finally {
      setIsUpdating(false);
    }
  };

  // Submit Delete
  const handleConfirmDelete = async () => {
    if (!deletingReview) return;

    setIsDeleting(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(
        `${baseUrl}/api/v1/reviews/${deletingReview.uuid}`,
        {
          method: "DELETE",
        },
      );

      if (res.ok) {
        toast.success("Review deleted successfully.");
        setDeletingReview(null);
        if (reviews.length === 1 && currentPage > 1) {
          setCurrentPage((p) => p - 1);
        } else {
          fetchMyReviews(currentPage);
        }
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(errJson, "Failed to delete review.");
        toast.error(msg);
      }
    } catch {
      toast.error("Network error while deleting review.");
    } finally {
      setIsDeleting(false);
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
      <div
        className="p-8 rounded-md bg-white border border-[#E2E8F0] shadow-sm flex items-center justify-center min-h-[300px]"
        style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
      >
        <Loader2 className="w-8 h-8 text-[#007D9C] animate-spin" />
      </div>
    );
  }

  return (
    <div
      className="space-y-6"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      {/* Top Header & Write Review Action */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-[#E2E8F0]">
        <div>
          <h1
            className="text-2xl font-bold text-[#0F172A]"
            style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
          >
            My Reviews
          </h1>
          <p className="text-sm text-[#64748B]">
            Manage your product reviews and feedback for purchased gaming gears
          </p>
        </div>
        {unreviewedPendingItems.length > 0 ? (
          <button
            type="button"
            onClick={() => setActiveTab("pending")}
            className="inline-flex items-center justify-center gap-2 bg-[#007D9C] text-white px-4 py-2 rounded-md text-sm font-semibold hover:bg-[#00657E] transition-colors cursor-pointer"
          >
            <PlusCircle className="w-4 h-4" />
            <span>To Review ({unreviewedPendingItems.length})</span>
          </button>
        ) : (
          <Link
            href="/account/orders"
            className="inline-flex items-center justify-center gap-2 bg-[#007D9C] text-white px-4 py-2 rounded-md text-sm font-semibold hover:bg-[#00657E] transition-colors cursor-pointer"
          >
            <ShoppingBag className="w-4 h-4" />
            <span>View Orders</span>
          </Link>
        )}
      </div>

      {/* Rating Summary Bar / Overview Box */}
      <div className="bg-white border border-[#E2E8F0] rounded-md p-5 grid grid-cols-1 md:grid-cols-3 divide-y md:divide-y-0 md:divide-x divide-[#E2E8F0]">
        {/* Metric 1: Avg Rating Given */}
        <div className="p-3 text-center md:text-left">
          <div className="flex items-center justify-center md:justify-start gap-3">
            <span
              className="text-3xl font-bold text-[#0F172A]"
              style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
            >
              {avgRatingNumber > 0 ? avgRatingNumber.toFixed(1) : "—"}
            </span>
            <div className="flex text-[#F59E0B]">
              {[1, 2, 3, 4, 5].map((s) => (
                <Star
                  key={s}
                  className={`w-4 h-4 ${
                    s <= Math.round(avgRatingNumber)
                      ? "fill-amber-400 text-amber-400"
                      : "text-[#E2E8F0]"
                  }`}
                />
              ))}
            </div>
          </div>
          <p className="text-sm font-bold text-[#0F172A] mt-1">
            Average Rating Given
          </p>
        </div>

        {/* Metric 2: Total Written */}
        <div className="p-3 text-center md:text-left md:pl-6">
          <div
            className="text-3xl font-bold text-[#0F172A]"
            style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
          >
            {totalRecords}
          </div>
          <p className="text-sm font-bold text-[#0F172A] mt-1">
            Total Reviews Written
          </p>
        </div>
      </div>

      {/* Sub-tabs Bar */}
      <div className="border-b border-[#E2E8F0] flex gap-6">
        <button
          type="button"
          onClick={() => setActiveTab("reviewed")}
          className={`pb-3 px-1 flex items-center gap-2 text-sm font-bold transition-colors cursor-pointer border-b-2 ${
            activeTab === "reviewed"
              ? "border-[#007D9C] text-[#007D9C]"
              : "border-transparent text-[#64748B] hover:text-[#0F172A]"
          }`}
        >
          <span>Reviewed ({totalRecords})</span>
        </button>
        <button
          type="button"
          onClick={() => setActiveTab("pending")}
          className={`pb-3 px-1 flex items-center gap-2 text-sm font-semibold transition-colors cursor-pointer border-b-2 ${
            activeTab === "pending"
              ? "border-[#007D9C] text-[#007D9C]"
              : "border-transparent text-[#64748B] hover:text-[#0F172A]"
          }`}
        >
          <span>To Review</span>
          {unreviewedPendingItems.length > 0 && (
            <span className="bg-[#F26522] text-white text-[11px] px-2 py-0.5 rounded-md font-bold">
              {unreviewedPendingItems.length}
            </span>
          )}
        </button>
      </div>

      {/* Content Stream */}
      {activeTab === "pending" ? (
        /* To Review Tab View */
        loadingOrders ? (
          <div className="p-12 rounded-md bg-white border border-[#E2E8F0] shadow-sm text-center space-y-3">
            <Loader2 className="w-8 h-8 text-[#007D9C] animate-spin mx-auto" />
            <p className="text-xs text-[#64748B] font-medium">
              Checking for delivered items to review...
            </p>
          </div>
        ) : unreviewedPendingItems.length > 0 ? (
          <div className="space-y-4">
            {unreviewedPendingItems.map((item, idx) => (
              <div
                key={`${item.orderUuid}-${item.productUuid}-${idx}`}
                className="bg-white border border-[#E2E8F0] rounded-md p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4 hover:border-[#00ADD8]/40 transition-colors shadow-2xs"
              >
                <div className="flex items-center gap-4">
                  <div className="w-14 h-14 rounded-md border border-[#E2E8F0] bg-[#F1F5F9] shrink-0 flex items-center justify-center overflow-hidden">
                    {item.productImageUrl ? (
                      <img
                        src={getImageUrl(item.productImageUrl)}
                        alt={item.productName}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <ShoppingBag className="w-6 h-6 text-[#64748B]" />
                    )}
                  </div>
                  <div>
                    <h3
                      className="font-bold text-sm text-[#0F172A]"
                      style={{
                        fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                      }}
                    >
                      {item.productName}
                    </h3>
                    <p className="text-xs text-[#64748B] mt-0.5">
                      Order #{item.orderNumber} • Delivered
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setWritingItem(item)}
                  className="bg-[#007D9C] text-white text-xs font-bold px-4 py-2 rounded-md hover:bg-[#00657E] transition-colors self-start sm:self-auto text-center cursor-pointer"
                >
                  Write a Review
                </button>
              </div>
            ))}
          </div>
        ) : (
          <div className="p-12 rounded-md bg-white border border-[#E2E8F0] shadow-sm text-center space-y-3">
            <CheckCircle2 className="w-10 h-10 text-emerald-500 mx-auto" />
            <div className="space-y-1">
              <h3 className="text-base font-bold text-[#0F172A]">
                All Caught Up!
              </h3>
              <p className="text-xs text-[#64748B]">
                You have reviewed all delivered items from your orders.
              </p>
            </div>
          </div>
        )
      ) : loading ? (
        <div className="p-12 rounded-md bg-white border border-[#E2E8F0] shadow-sm text-center space-y-3">
          <Loader2 className="w-8 h-8 text-[#007D9C] animate-spin mx-auto" />
          <p className="text-xs text-[#64748B] font-medium">
            Loading your reviews...
          </p>
        </div>
      ) : reviews.length > 0 ? (
        <div className="space-y-6">
          {reviews.map((rev) => (
            <article
              key={rev.uuid}
              className="bg-white border border-[#E2E8F0] rounded-md p-6 space-y-4 shadow-2xs hover:border-[#00ADD8]/40 transition-all"
            >
              {/* Header Product Info */}
              <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4 pb-4 border-b border-[#E2E8F0]">
                <div className="flex items-center gap-4">
                  <div className="w-16 h-16 rounded-md border border-[#E2E8F0] bg-[#F1F5F9] shrink-0 overflow-hidden flex items-center justify-center">
                    {rev.product_image_url ? (
                      <img
                        src={getImageUrl(rev.product_image_url)}
                        alt={rev.product_name}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <ShoppingBag className="w-6 h-6 text-[#64748B]" />
                    )}
                  </div>
                  <div>
                    <Link
                      href={`/products/${rev.product_slug}`}
                      className="font-bold text-sm text-[#0F172A] hover:text-[#007D9C] transition-colors line-clamp-1 inline-flex items-center gap-1.5 group"
                      style={{
                        fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                      }}
                    >
                      <span>{rev.product_name}</span>
                      <ExternalLink className="w-3.5 h-3.5 text-[#64748B] group-hover:text-[#007D9C] shrink-0" />
                    </Link>
                    <p className="text-xs text-[#64748B] mt-0.5 font-mono">
                      Submitted on {rev.created_at}
                    </p>
                  </div>
                </div>
              </div>

              {/* Rating & Date */}
              <div className="flex items-center gap-3">
                <div className="flex text-[#F59E0B]">
                  {[1, 2, 3, 4, 5].map((star) => (
                    <Star
                      key={star}
                      className={`w-4 h-4 ${
                        star <= rev.rating
                          ? "fill-amber-400 text-amber-400"
                          : "text-[#E2E8F0]"
                      }`}
                    />
                  ))}
                </div>
              </div>

              {/* Review Title & Feedback */}
              <div className="space-y-1.5">
                {rev.title && (
                  <h4
                    className="font-bold text-base text-[#0F172A]"
                    style={{
                      fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                    }}
                  >
                    {rev.title}
                  </h4>
                )}
                {rev.comment ? (
                  <p className="text-sm text-[#0F172A] leading-relaxed whitespace-pre-line">
                    {rev.comment}
                  </p>
                ) : (
                  <p className="text-xs text-[#64748B] italic">
                    No written comment provided.
                  </p>
                )}
              </div>

              {/* Photos Gallery */}
              {rev.images && rev.images.length > 0 && (
                <div className="flex flex-wrap gap-2.5 pt-1">
                  {rev.images.map((imgUrl, i) => (
                    <button
                      key={i}
                      type="button"
                      onClick={() => setPreviewMediaUrl(getImageUrl(imgUrl))}
                      className="w-20 h-20 rounded-md overflow-hidden border border-[#E2E8F0] bg-[#F1F5F9] hover:opacity-90 hover:scale-105 transition-all cursor-pointer shadow-2xs"
                    >
                      <img
                        src={getImageUrl(imgUrl)}
                        alt={`Review photo ${i + 1}`}
                        className="w-full h-full object-cover"
                      />
                    </button>
                  ))}
                </div>
              )}

              {/* Card Footer Actions */}
              <div className="flex items-center justify-between pt-3 border-t border-[#E2E8F0]">
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => handleOpenEdit(rev)}
                    disabled={!rev.can_edit}
                    className="border border-[#E2E8F0] text-[#0F172A] rounded-md px-3 py-1.5 text-xs font-bold hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed transition-all cursor-pointer inline-flex items-center gap-1.5"
                  >
                    <Edit3 className="w-3.5 h-3.5 text-[#64748B]" />
                    <span>Edit Review</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setDeletingReview(rev)}
                    className="text-[#DC2626] hover:underline text-xs font-bold ml-2 inline-flex items-center gap-1 cursor-pointer"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                    <span>Delete</span>
                  </button>
                </div>
              </div>
            </article>
          ))}

          {/* Full Pagination Controls */}
          {totalPages > 1 && (
            <div className="p-4 rounded-md bg-white border border-[#E2E8F0] shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
              <span className="text-[#64748B] font-medium text-center sm:text-left">
                Showing page{" "}
                <strong className="text-[#0F172A]">{currentPage}</strong> of{" "}
                <strong className="text-[#0F172A]">{totalPages}</strong> (
                {totalRecords} total items)
              </span>

              <div className="flex items-center justify-center gap-1">
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage(1)}
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-[#0F172A] font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="First Page"
                >
                  <ChevronsLeft className="w-4 h-4" />
                </button>
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-[#0F172A] font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="Previous Page"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>

                {getPaginationPages().map((pageNum, idx) => {
                  if (pageNum === "...") {
                    return (
                      <span
                        key={`dots-${idx}`}
                        className="w-8 h-8 flex items-center justify-center text-[#64748B] font-bold"
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
                      disabled={loading}
                      onClick={() => setCurrentPage(pageNumber)}
                      className={`w-8 h-8 rounded-md text-xs font-bold transition-all cursor-pointer ${
                        isActive
                          ? "bg-[#007D9C] text-white shadow-xs"
                          : "border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] text-[#0F172A]"
                      }`}
                    >
                      {pageNumber}
                    </button>
                  );
                })}

                <button
                  type="button"
                  disabled={currentPage >= totalPages || loading}
                  onClick={() =>
                    setCurrentPage((p) => Math.min(totalPages, p + 1))
                  }
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-[#0F172A] font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="Next Page"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>
                <button
                  type="button"
                  disabled={currentPage >= totalPages || loading}
                  onClick={() => setCurrentPage(totalPages)}
                  className="w-8 h-8 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed text-[#0F172A] font-bold flex items-center justify-center transition-all cursor-pointer"
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
        <div className="p-12 sm:p-16 rounded-md bg-white border border-[#E2E8F0] shadow-sm text-center space-y-4">
          <div className="w-16 h-16 rounded-md bg-[#E0F7FC] text-[#007D9C] flex items-center justify-center mx-auto shadow-xs">
            <MessageSquare className="w-8 h-8" />
          </div>
          <div className="space-y-1 max-w-md mx-auto">
            <h3
              className="text-lg font-bold text-[#0F172A]"
              style={{ fontFamily: 'var(--font-heading, "Rubik", sans-serif)' }}
            >
              No Reviews Yet
            </h3>
            <p className="text-xs text-[#64748B] leading-relaxed">
              You haven&apos;t written any product reviews yet. Reviewing your
              delivered orders helps the community make informed decisions!
            </p>
          </div>
          <div className="pt-2">
            <Link
              href="/account/orders"
              className="inline-flex items-center gap-2 px-6 py-2.5 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-xs font-bold transition-all shadow-md cursor-pointer"
            >
              <ShoppingBag className="w-4 h-4" />
              <span>Go to My Orders</span>
            </Link>
          </div>
        </div>
      )}

      {/* Write Review Modal (Opened from To Review tab) */}
      {writingItem && (
        <WriteReviewModal
          isOpen={Boolean(writingItem)}
          onClose={() => setWritingItem(null)}
          productUuid={writingItem.productUuid}
          productName={writingItem.productName}
          productImageUrl={writingItem.productImageUrl}
          orderUuid={writingItem.orderUuid}
          onSuccess={() => {
            fetchMyReviews(1);
            fetchDeliveredOrders();
            setActiveTab("reviewed");
          }}
        />
      )}

      {/* Edit Review Modal */}
      {editingReview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs">
          <div
            className="relative w-full max-w-lg bg-white rounded-md shadow-2xl border border-[#E2E8F0] overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="p-5 border-b border-[#E2E8F0] flex items-center justify-between bg-[#F8FAFC]">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-md bg-[#E0F7FC] text-[#007D9C] flex items-center justify-center shadow-xs">
                  <Edit3 className="w-5 h-5" />
                </div>
                <div>
                  <h2
                    className="text-sm font-bold text-[#0F172A]"
                    style={{
                      fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                    }}
                  >
                    Edit Review
                  </h2>
                  <p className="text-xs text-[#64748B] truncate max-w-xs">
                    {editingReview.product_name}
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setEditingReview(null)}
                className="w-8 h-8 rounded-full bg-[#E2E8F0]/50 text-[#64748B] hover:text-[#0F172A] hover:bg-[#E2E8F0] flex items-center justify-center transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSaveEdit} className="p-6 space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-[#0F172A] block">
                  Overall Rating <span className="text-rose-500">*</span>
                </label>
                <div className="flex items-center gap-2">
                  <div className="flex items-center gap-1">
                    {[1, 2, 3, 4, 5].map((star) => {
                      const isFilled = (editHoverRating || editRating) >= star;
                      return (
                        <button
                          key={star}
                          type="button"
                          onClick={() => setEditRating(star)}
                          onMouseEnter={() => setEditHoverRating(star)}
                          onMouseLeave={() => setEditHoverRating(0)}
                          className="p-1 -m-1 text-[#E2E8F0] hover:scale-110 transition-transform cursor-pointer"
                        >
                          <Star
                            className={`w-6 h-6 ${
                              isFilled
                                ? "fill-amber-400 text-amber-400"
                                : "text-[#E2E8F0]"
                            }`}
                          />
                        </button>
                      );
                    })}
                  </div>
                  <span className="text-xs font-bold text-[#0F172A] ml-2">
                    {editRating}.0 Stars
                  </span>
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-bold text-[#0F172A] block">
                  Review Headline{" "}
                  <span className="text-[#64748B] font-normal">(Optional)</span>
                </label>
                <input
                  type="text"
                  maxLength={200}
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  placeholder="e.g. Excellent build quality and fast shipping!"
                  className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C]"
                />
              </div>

              <div className="space-y-1.5">
                <div className="flex items-center justify-between text-xs">
                  <label className="font-bold text-[#0F172A]">
                    Review Details{" "}
                    <span className="text-[#64748B] font-normal">
                      (Optional)
                    </span>
                  </label>
                  <span className="text-[11px] text-[#64748B] font-mono">
                    {editComment.length}/2000
                  </span>
                </div>
                <textarea
                  rows={4}
                  maxLength={2000}
                  value={editComment}
                  onChange={(e) => setEditComment(e.target.value)}
                  placeholder="Share details about your experience..."
                  className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C] resize-none"
                />
              </div>

              <div className="pt-3 flex items-center justify-end gap-3 border-t border-[#E2E8F0]">
                <button
                  type="button"
                  onClick={() => setEditingReview(null)}
                  disabled={isUpdating}
                  className="px-4 py-2 rounded-md border border-[#E2E8F0] text-[#64748B] text-xs font-bold hover:bg-[#F8FAFC] transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isUpdating}
                  className="inline-flex items-center gap-2 px-5 py-2 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-xs font-bold transition-colors cursor-pointer disabled:opacity-50"
                >
                  {isUpdating ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>Saving...</span>
                    </>
                  ) : (
                    <>
                      <CheckCircle2 className="w-4 h-4" />
                      <span>Save Changes</span>
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deletingReview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs">
          <div
            className="relative w-full max-w-md bg-white rounded-md shadow-2xl border border-[#E2E8F0] p-6 space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="w-12 h-12 rounded-md bg-rose-50 border border-rose-200 text-rose-600 flex items-center justify-center mx-auto shadow-xs">
              <AlertTriangle className="w-6 h-6" />
            </div>

            <div className="text-center space-y-1">
              <h3
                className="text-base font-bold text-[#0F172A]"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                Delete Review?
              </h3>
              <p className="text-xs text-[#64748B] leading-relaxed">
                Are you sure you want to delete your review for{" "}
                <strong className="text-[#0F172A] font-bold">
                  {deletingReview.product_name}
                </strong>
                ? This action cannot be undone.
              </p>
            </div>

            <div className="flex items-center justify-center gap-3 pt-2">
              <button
                type="button"
                onClick={() => setDeletingReview(null)}
                disabled={isDeleting}
                className="w-1/2 py-2.5 rounded-md border border-[#E2E8F0] text-[#64748B] text-xs font-bold hover:bg-[#F8FAFC] transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmDelete}
                disabled={isDeleting}
                className="w-1/2 inline-flex items-center justify-center gap-2 py-2.5 rounded-md bg-[#DC2626] hover:bg-rose-700 text-white text-xs font-bold transition-colors cursor-pointer disabled:opacity-50"
              >
                {isDeleting ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    <span>Deleting...</span>
                  </>
                ) : (
                  <>
                    <Trash2 className="w-4 h-4" />
                    <span>Delete</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Media Lightbox Modal */}
      {previewMediaUrl && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs"
          onClick={() => setPreviewMediaUrl(null)}
        >
          <div className="relative max-w-3xl max-h-[85vh]">
            <img
              src={previewMediaUrl}
              alt="Expanded photo preview"
              className="max-w-full max-h-[85vh] rounded-md object-contain shadow-2xl"
            />
            <button
              type="button"
              onClick={() => setPreviewMediaUrl(null)}
              className="absolute top-3 right-3 w-8 h-8 rounded-full bg-black/60 text-white flex items-center justify-center hover:bg-black/80 transition-colors cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
