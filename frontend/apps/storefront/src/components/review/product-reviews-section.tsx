"use client";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import { useRouter } from "next/navigation";
import type { Review, ReviewSummary } from "@/lib/api-server";
import { authFetch } from "@/lib/api";
import { getImageUrl, extractErrorMessage } from "@/lib/utils";
import { useAuthStore } from "@/stores/auth-store";
import { WriteReviewModal } from "./write-review-modal";
import { ReportReviewModal } from "./report-review-modal";
import {
  Star,
  ThumbsUp,
  MessageSquare,
  Sparkles,
  Camera,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Loader2,
  Flag,
  X,
} from "lucide-react";
import { toast } from "sonner";

interface ProductReviewsSectionProps {
  productUuid: string;
  productName: string;
  productImageUrl?: string;
  initialSummary?: ReviewSummary | null;
  initialReviews?: Review[];
}

type FilterType = "all" | 5 | 4 | 3 | 2 | 1 | "comment" | "media";

export function ProductReviewsSection({
  productUuid,
  productName,
  productImageUrl,
  initialSummary,
  initialReviews = [],
}: ProductReviewsSectionProps) {
  const router = useRouter();
  const { accessToken } = useAuthStore();

  const [summary, setSummary] = useState<ReviewSummary | null>(
    initialSummary || null,
  );
  const [reviews, setReviews] = useState<Review[]>(initialReviews);
  const [loading, setLoading] = useState(false);
  const [activeFilter, setActiveFilter] = useState<FilterType>("all");
  const [orderBy, setOrderBy] = useState<"created_at" | "rating" | "helpful">(
    "created_at",
  );
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");
  const [currentPage, setCurrentPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [totalRecords, setTotalRecords] = useState(
    initialSummary?.total_reviews || 0,
  );

  // Write Review State
  const [isWriteModalOpen, setIsWriteModalOpen] = useState(false);
  const [eligibleOrders, setEligibleOrders] = useState<
    Array<{ order_uuid: string; order_number: string; created_at?: string }>
  >([]);
  const [checkingEligibility, setCheckingEligibility] = useState(false);

  // Helpful Voting State
  const [votedReviews, setVotedReviews] = useState<Record<string, boolean>>({});
  const [votingReviewUuid, setVotingReviewUuid] = useState<string | null>(null);

  // Report Review State
  const [reportingReview, setReportingReview] = useState<{
    uuid: string;
    author?: string;
  } | null>(null);

  // Lightbox Preview Image
  const [previewMediaUrl, setPreviewMediaUrl] = useState<string | null>(null);

  const fetchSummary = useCallback(async () => {
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await fetch(
        `${baseUrl}/api/v1/products/${productUuid}/reviews/summary`,
      );
      if (res.ok) {
        const json = await res.json();
        if (json?.data) {
          setSummary(json.data);
          if (json.data.total_reviews !== undefined) {
            setTotalRecords(json.data.total_reviews);
          }
        }
      }
    } catch {
      // Ignore background refresh errors
    }
  }, [productUuid]);

  const fetchReviews = useCallback(async () => {
    setLoading(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const params = new URLSearchParams();
      params.set("page", String(currentPage));
      params.set("limit", "10");

      if (typeof activeFilter === "number") {
        params.set("rating", String(activeFilter));
      } else if (activeFilter === "media") {
        params.set("has_media", "true");
      } else if (activeFilter === "comment") {
        params.set("has_comment", "true");
      }

      params.set("order_by", orderBy);
      params.set("sort", sortOrder);

      const res = await fetch(
        `${baseUrl}/api/v1/products/${productUuid}/reviews?${params.toString()}`,
      );
      if (res.ok) {
        const json = await res.json();
        let fetchedReviews: Review[] = json?.data || [];

        // Client fallback filter if backend does not filter has_comment
        if (activeFilter === "comment") {
          fetchedReviews = fetchedReviews.filter(
            (r) => r.comment && r.comment.trim().length > 0,
          );
        }

        setReviews(fetchedReviews);

        const pagination = json?.pagination;
        if (pagination) {
          setTotalPages(pagination.total_pages || 1);
          setTotalRecords(pagination.total_records || fetchedReviews.length);
        }
      }
    } catch {
      toast.error("Unable to load product reviews.");
    } finally {
      setLoading(false);
    }
  }, [productUuid, currentPage, activeFilter, orderBy, sortOrder]);

  useEffect(() => {
    fetchReviews();
  }, [fetchReviews]);

  // Toggle Helpful Vote
  const handleToggleHelpful = async (reviewUuid: string) => {
    if (!accessToken) {
      toast.error("Please sign in to vote on customer reviews.");
      router.push(
        `/login?redirect=${encodeURIComponent(window.location.pathname)}`,
      );
      return;
    }

    if (votingReviewUuid) return;
    setVotingReviewUuid(reviewUuid);

    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(
        `${baseUrl}/api/v1/reviews/${reviewUuid}/helpful`,
        {
          method: "POST",
        },
      );

      if (res.ok) {
        const json = await res.json();
        const voteData = json?.data;
        const isVoted = voteData?.voted ?? !votedReviews[reviewUuid];
        const newCount = voteData?.helpful_count;

        setVotedReviews((prev) => ({ ...prev, [reviewUuid]: isVoted }));
        setReviews((prev) =>
          prev.map((r) =>
            r.uuid === reviewUuid
              ? {
                  ...r,
                  helpful_count:
                    newCount !== undefined
                      ? newCount
                      : isVoted
                        ? (r.helpful_count || 0) + 1
                        : Math.max(0, (r.helpful_count || 1) - 1),
                }
              : r,
          ),
        );

        toast.success(isVoted ? "Helpful vote recorded!" : "Vote removed.");
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(
          errJson,
          "Failed to register helpful vote.",
        );
        toast.error(msg);
      }
    } catch {
      toast.error("Network error while registering vote.");
    } finally {
      setVotingReviewUuid(null);
    }
  };

  // Handle Write Review Click
  const handleOpenWriteReview = async () => {
    if (!accessToken) {
      toast.error("Please sign in to review this product.");
      router.push(
        `/login?redirect=${encodeURIComponent(window.location.pathname)}`,
      );
      return;
    }

    setCheckingEligibility(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(`${baseUrl}/api/v1/orders?limit=100`);
      if (res.ok) {
        const json = await res.json();
        const ordersList: Array<{
          uuid: string;
          order_number: string;
          created_at?: string;
          items?: Array<{ product_uuid?: string }>;
        }> = json?.data || [];

        const matchedOrders = ordersList
          .filter((order) =>
            order.items?.some((item) => item.product_uuid === productUuid),
          )
          .map((ord) => ({
            order_uuid: ord.uuid,
            order_number: ord.order_number,
            created_at: ord.created_at,
          }));

        if (matchedOrders.length > 0) {
          setEligibleOrders(matchedOrders);
          setIsWriteModalOpen(true);
        } else {
          toast.error(
            "You can only review products you have purchased. Please place an order first!",
          );
        }
      } else {
        toast.error("Failed to verify purchase history.");
      }
    } catch {
      toast.error("Unable to verify order eligibility.");
    } finally {
      setCheckingEligibility(false);
    }
  };

  // Summary counts
  const totalReviewsCount = summary?.total_reviews || 0;
  const avgRating = summary?.avg_rating || 0;
  const fiveStar = summary?.five_star || 0;
  const fourStar = summary?.four_star || 0;
  const threeStar = summary?.three_star || 0;
  const twoStar = summary?.two_star || 0;
  const oneStar = summary?.one_star || 0;
  const mediaCount = summary?.with_media_count || 0;

  // Count reviews with comments from current reviews or calculate
  const withCommentCount = useMemo(() => {
    if (summary?.with_comment_count !== undefined) {
      return summary.with_comment_count;
    }
    return totalReviewsCount;
  }, [summary, totalReviewsCount]);

  const scrollToSection = () => {
    const el = document.getElementById("customer-reviews");
    if (el) {
      const top = el.getBoundingClientRect().top + window.scrollY - 80;
      window.scrollTo({ top, behavior: "smooth" });
    }
  };

  const handleFilterChange = (filter: FilterType) => {
    setActiveFilter(filter);
    setCurrentPage(1);
    scrollToSection();
  };

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    scrollToSection();
  };

  // Pagination page numbers generator: < 1 2 3 4 5 ... totalPages >
  const paginationItems = useMemo(() => {
    if (totalPages <= 1) return [];
    const pages: (number | string)[] = [];

    if (totalPages <= 7) {
      for (let i = 1; i <= totalPages; i++) pages.push(i);
    } else {
      if (currentPage <= 4) {
        for (let i = 1; i <= 5; i++) pages.push(i);
        pages.push("...");
        pages.push(totalPages);
      } else if (currentPage >= totalPages - 3) {
        pages.push(1);
        pages.push("...");
        for (let i = totalPages - 4; i <= totalPages; i++) pages.push(i);
      } else {
        pages.push(1);
        pages.push("...");
        pages.push(currentPage - 1);
        pages.push(currentPage);
        pages.push(currentPage + 1);
        pages.push("...");
        pages.push(totalPages);
      }
    }
    return pages;
  }, [currentPage, totalPages]);

  return (
    <section
      id="customer-reviews"
      className="bg-white border border-[#E2E8F0] rounded-md p-5 sm:p-6 shadow-xs space-y-6"
    >
      {/* 1. Header & Rating Score Card with Filters */}
      <div className="space-y-4 pb-5 border-b border-[#E2E8F0]">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h2 className="font-heading text-lg font-bold text-[#0F172A] tracking-tight">
              Customer Reviews
            </h2>
            <p className="text-xs text-[#64748B]">
              Real feedback and ratings from verified buyers.
            </p>
          </div>

          <button
            type="button"
            onClick={handleOpenWriteReview}
            disabled={checkingEligibility}
            className="inline-flex items-center justify-center gap-1.5 px-4 py-2 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-xs font-semibold transition-colors shadow-xs disabled:opacity-50 cursor-pointer self-start sm:self-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#007D9C]"
          >
            {checkingEligibility ? (
              <>
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                <span>Checking...</span>
              </>
            ) : (
              <>
                <Sparkles className="w-3.5 h-3.5" />
                <span>Write a Review</span>
              </>
            )}
          </button>
        </div>

        {/* Rating Score & Filter Chips Container */}
        <div className="bg-[#F8FAFC] border border-[#E2E8F0] rounded-md p-4 flex flex-col md:flex-row items-start md:items-center gap-5">
          {/* Left: Rating X / 5.0 + Stars */}
          <div className="flex items-center gap-3 shrink-0 pr-0 md:pr-5 md:border-r border-[#E2E8F0]">
            <div className="flex items-baseline gap-1">
              <span className="font-heading text-3xl font-extrabold text-[#0F172A] leading-none tracking-tight">
                {avgRating > 0 ? avgRating.toFixed(1) : "5.0"}
              </span>
              <span className="text-xs text-[#64748B] font-semibold">
                / 5.0
              </span>
            </div>

            <div className="space-y-0.5">
              <div className="flex text-amber-500">
                {[1, 2, 3, 4, 5].map((star) => (
                  <Star
                    key={star}
                    className={`w-4 h-4 ${
                      star <= Math.round(avgRating || 5)
                        ? "fill-amber-400 text-amber-400"
                        : "text-slate-200"
                    }`}
                  />
                ))}
              </div>
              <p className="text-[11px] text-[#64748B]">
                {totalReviewsCount}{" "}
                {totalReviewsCount === 1 ? "rating" : "ratings"}
              </p>
            </div>
          </div>

          {/* Right: Filter Chips (All, 5 Stars, 4 Stars, 3 Stars, 2 Stars, 1 Star, With Comments, With Media) */}
          <div className="flex items-center gap-1.5 sm:gap-2 flex-wrap text-xs">
            <button
              type="button"
              onClick={() => handleFilterChange("all")}
              className={`min-h-[36px] sm:min-h-[32px] px-3.5 py-1.5 rounded-md text-xs font-semibold border transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#007D9C] ${
                activeFilter === "all"
                  ? "bg-[#007D9C] text-white border-[#007D9C]"
                  : "bg-white text-[#0F172A] border-[#E2E8F0] hover:border-[#CBD5E1]"
              }`}
            >
              All ({totalReviewsCount})
            </button>

            {[
              { star: 5 as const, count: fiveStar },
              { star: 4 as const, count: fourStar },
              { star: 3 as const, count: threeStar },
              { star: 2 as const, count: twoStar },
              { star: 1 as const, count: oneStar },
            ].map(({ star, count }) => (
              <button
                key={star}
                type="button"
                onClick={() => handleFilterChange(star)}
                className={`min-h-[36px] sm:min-h-[32px] inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold border transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#007D9C] ${
                  activeFilter === star
                    ? "bg-[#007D9C] text-white border-[#007D9C]"
                    : "bg-white text-[#0F172A] border-[#E2E8F0] hover:border-[#CBD5E1]"
                }`}
              >
                <span>{star}</span>
                <Star
                  className={`w-3 h-3 ${activeFilter === star ? "fill-white text-white" : "fill-amber-400 text-amber-400"}`}
                />
                <span className="opacity-80">({count})</span>
              </button>
            ))}

            <button
              type="button"
              onClick={() => handleFilterChange("comment")}
              className={`min-h-[36px] sm:min-h-[32px] inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold border transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#007D9C] ${
                activeFilter === "comment"
                  ? "bg-[#007D9C] text-white border-[#007D9C]"
                  : "bg-white text-[#0F172A] border-[#E2E8F0] hover:border-[#CBD5E1]"
              }`}
            >
              <MessageSquare className="w-3 h-3" />
              <span>With Comments ({withCommentCount})</span>
            </button>

            <button
              type="button"
              onClick={() => handleFilterChange("media")}
              className={`min-h-[36px] sm:min-h-[32px] inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold border transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#007D9C] ${
                activeFilter === "media"
                  ? "bg-[#007D9C] text-white border-[#007D9C]"
                  : "bg-white text-[#0F172A] border-[#E2E8F0] hover:border-[#CBD5E1]"
              }`}
            >
              <Camera className="w-3 h-3" />
              <span>With Media ({mediaCount})</span>
            </button>
          </div>
        </div>
      </div>

      {/* 2. Review List */}
      {loading ? (
        <div className="py-12 text-center space-y-2">
          <Loader2 className="w-6 h-6 text-[#007D9C] animate-spin mx-auto" />
          <p className="text-xs text-[#64748B]">Loading reviews...</p>
        </div>
      ) : reviews.length > 0 ? (
        <div className="space-y-4">
          {reviews.map((rev) => {
            const hasVoted = votedReviews[rev.uuid];
            const isVoting = votingReviewUuid === rev.uuid;

            return (
              <div
                key={rev.uuid}
                className="bg-white border border-[#E2E8F0] rounded-md p-4 space-y-3"
              >
                {/* Review Header: User info + Stars */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                  <div className="flex items-center gap-2.5">
                    <div className="w-8 h-8 rounded-full bg-[#007D9C] text-white font-bold text-xs flex items-center justify-center shrink-0">
                      {rev.reviewer_name?.slice(0, 1) || "U"}
                    </div>

                    <div>
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-[#0F172A]">
                          {rev.reviewer_name || "Verified Buyer"}
                        </span>
                      </div>
                      <p className="text-[11px] text-[#94A3B8] font-mono">
                        {rev.created_at}
                      </p>

                      {/* Rating Stars */}
                      <div className="flex text-amber-400">
                        {[1, 2, 3, 4, 5].map((star) => (
                          <Star
                            key={star}
                            className={`w-3.5 h-3.5 ${
                              star <= rev.rating
                                ? "fill-amber-400 text-amber-400"
                                : "text-slate-200"
                            }`}
                          />
                        ))}
                      </div>
                    </div>
                  </div>
                </div>

                {/* Review Content */}
                <div className="space-y-1 text-sm">
                  {rev.title && (
                    <h4 className="font-bold text-[#0F172A]">{rev.title}</h4>
                  )}
                  {rev.comment ? (
                    <p className="text-[#0F172A] leading-relaxed whitespace-pre-line">
                      {rev.comment}
                    </p>
                  ) : (
                    <p className="text-[#94A3B8] italic">
                      No additional comment provided.
                    </p>
                  )}
                </div>

                {/* Media Gallery (Photos) */}
                {rev.images && rev.images.length > 0 && (
                  <div className="flex flex-wrap gap-2 pt-1">
                    {rev.images.map((imgUrl, i) => (
                      <button
                        key={i}
                        type="button"
                        onClick={() => setPreviewMediaUrl(getImageUrl(imgUrl))}
                        className="w-16 h-16 rounded-md overflow-hidden border border-[#E2E8F0] bg-[#F8FAFC] hover:opacity-90 transition-opacity cursor-pointer p-0.5"
                        aria-label={`Preview review photo ${i + 1}`}
                      >
                        <img
                          src={getImageUrl(imgUrl)}
                          alt={`Review photo ${i + 1}`}
                          className="w-full h-full object-cover rounded-sm"
                          onError={(e) => {
                            (e.currentTarget as HTMLElement).parentElement?.classList.add("hidden");
                          }}
                        />
                      </button>
                    ))}
                  </div>
                )}

                {/* Seller Reply */}
                {rev.reply && (
                  <div className="p-3 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] space-y-1 text-xs">
                    <div className="flex items-center justify-between font-semibold text-[#007D9C]">
                      <div className="flex items-center gap-1">
                        <MessageSquare className="w-3 h-3" />
                        <span>Seller Response</span>
                      </div>
                      {rev.reply_at && (
                        <span className="text-[10px] text-[#94A3B8] font-mono font-normal">
                          {rev.reply_at}
                        </span>
                      )}
                    </div>
                    <p className="text-[#64748B] leading-relaxed">
                      {rev.reply}
                    </p>
                  </div>
                )}

                {/* Review Footer: Helpful & Report */}
                <div className="pt-2 border-t border-[#E2E8F0] flex items-center justify-between text-xs">
                  <span className="text-[11px] text-[#94A3B8]">
                    Was this review helpful?
                  </span>

                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => handleToggleHelpful(rev.uuid)}
                      disabled={isVoting}
                      className={`inline-flex items-center gap-1 px-2.5 py-1 rounded-md border text-xs font-semibold transition-colors cursor-pointer ${
                        hasVoted
                          ? "bg-[#F0F9FB] border-[#007D9C] text-[#007D9C]"
                          : "bg-white border-[#E2E8F0] hover:bg-[#F8FAFC] text-[#64748B]"
                      }`}
                    >
                      {isVoting ? (
                        <Loader2 className="w-3 h-3 animate-spin" />
                      ) : (
                        <ThumbsUp
                          className={`w-3 h-3 ${hasVoted ? "fill-[#007D9C] text-[#007D9C]" : ""}`}
                        />
                      )}
                      <span>Helpful ({rev.helpful_count || 0})</span>
                    </button>

                    <button
                      type="button"
                      onClick={() => {
                        if (!accessToken) {
                          toast.error("Please sign in to report a review.");
                          router.push(
                            `/login?redirect=${encodeURIComponent(window.location.pathname)}`,
                          );
                          return;
                        }
                        setReportingReview({
                          uuid: rev.uuid,
                          author: rev.reviewer_name,
                        });
                      }}
                      title="Report this review"
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-md border border-[#E2E8F0] bg-white hover:bg-rose-50 hover:border-rose-200 hover:text-rose-600 text-[#94A3B8] text-xs font-medium transition-colors cursor-pointer"
                    >
                      <Flag className="w-3 h-3" />
                      <span className="hidden sm:inline">Report</span>
                    </button>
                  </div>
                </div>
              </div>
            );
          })}

          {/* 3. Numbered Pagination: < 1 2 3 4 5 ... > */}
          {totalPages > 1 && (
            <div className="pt-3 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs">
              <span className="text-[#64748B]">
                Showing page{" "}
                <strong className="text-[#0F172A]">{currentPage}</strong> of{" "}
                <strong className="text-[#0F172A]">{totalPages}</strong>
              </span>

              <div className="flex items-center gap-1">
                {/* Previous Button < */}
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => handlePageChange(Math.max(1, currentPage - 1))}
                  className="px-2.5 py-1.5 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed font-semibold text-[#0F172A] transition-colors flex items-center justify-center cursor-pointer"
                  aria-label="Previous Page"
                >
                  <ChevronLeft className="w-3.5 h-3.5" />
                </button>

                {/* Page Number Chips */}
                {paginationItems.map((item, idx) => {
                  if (item === "...") {
                    return (
                      <span
                        key={`dots-${idx}`}
                        className="px-2 py-1 text-xs text-[#94A3B8]"
                      >
                        ...
                      </span>
                    );
                  }

                  const pageNum = Number(item);
                  const isActive = currentPage === pageNum;

                  return (
                    <button
                      key={`page-${pageNum}`}
                      type="button"
                      disabled={loading}
                      onClick={() => handlePageChange(pageNum)}
                      className={`min-w-[32px] h-8 px-2 rounded-md text-xs font-semibold border transition-colors flex items-center justify-center cursor-pointer ${
                        isActive
                          ? "bg-[#007D9C] text-white border-[#007D9C]"
                          : "bg-white text-[#0F172A] border-[#E2E8F0] hover:bg-[#F8FAFC]"
                      }`}
                    >
                      {pageNum}
                    </button>
                  );
                })}

                {/* Next Button > */}
                <button
                  type="button"
                  disabled={currentPage >= totalPages || loading}
                  onClick={() =>
                    handlePageChange(Math.min(totalPages, currentPage + 1))
                  }
                  className="px-2.5 py-1.5 rounded-md border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] disabled:opacity-40 disabled:cursor-not-allowed font-semibold text-[#0F172A] transition-colors flex items-center justify-center cursor-pointer"
                  aria-label="Next Page"
                >
                  <ChevronRight className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          )}
        </div>
      ) : (
        /* Empty State */
        <div className="py-10 text-center space-y-3">
          <div className="w-10 h-10 rounded-md bg-[#E0F7FC] text-[#007D9C] flex items-center justify-center mx-auto">
            <MessageSquare className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-sm font-bold text-[#0F172A]">
              No Reviews Found
            </h3>
            <p className="text-xs text-[#64748B] max-w-sm mx-auto mt-0.5">
              {activeFilter !== "all"
                ? "No reviews match the selected filter. Try selecting another rating or filter."
                : "Be the first to review this product and help other customers!"}
            </p>
          </div>
          {activeFilter !== "all" ? (
            <button
              type="button"
              onClick={() => setActiveFilter("all")}
              className="px-3 py-1.5 rounded-md bg-white border border-[#E2E8F0] hover:bg-[#F8FAFC] text-[#0F172A] text-xs font-semibold transition-colors cursor-pointer"
            >
              Clear Filter
            </button>
          ) : (
            <button
              type="button"
              onClick={handleOpenWriteReview}
              disabled={checkingEligibility}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-xs font-semibold transition-colors shadow-xs cursor-pointer disabled:opacity-50"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span>Write the First Review</span>
            </button>
          )}
        </div>
      )}

      {/* Write Review Modal */}
      <WriteReviewModal
        isOpen={isWriteModalOpen}
        onClose={() => setIsWriteModalOpen(false)}
        productUuid={productUuid}
        productName={productName}
        productImageUrl={productImageUrl}
        eligibleOrders={eligibleOrders}
        onSuccess={() => {
          fetchReviews();
          fetchSummary();
        }}
      />

      {/* Report Review Modal */}
      {reportingReview && (
        <ReportReviewModal
          isOpen={!!reportingReview}
          onClose={() => setReportingReview(null)}
          reviewUuid={reportingReview.uuid}
          reviewAuthor={reportingReview.author}
        />
      )}

      {/* Lightbox Modal */}
      {previewMediaUrl && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-xs"
          onClick={() => setPreviewMediaUrl(null)}
        >
          <div className="relative max-w-3xl max-h-[85vh]">
            <img
              src={previewMediaUrl}
              alt="Expanded preview"
              className="max-w-full max-h-[85vh] rounded-md object-contain shadow-2xl"
            />
            <button
              type="button"
              onClick={() => setPreviewMediaUrl(null)}
              className="absolute top-2 right-2 w-7 h-7 rounded-md bg-black/60 text-white flex items-center justify-center hover:bg-black/80 transition-colors cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
