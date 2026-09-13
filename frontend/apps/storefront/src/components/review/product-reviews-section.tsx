'use client'

import React, { useState, useEffect, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import type { Review, ReviewSummary } from '@/lib/api-server'
import { authFetch } from '@/lib/api'
import { getImageUrl, extractErrorMessage } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { WriteReviewModal } from './write-review-modal'
import { ReportReviewModal } from './report-review-modal'
import {
  Star,
  ThumbsUp,
  MessageSquare,
  Sparkles,
  Camera,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Filter,
  Loader2,
  AlertCircle,
  ExternalLink,
  ShieldCheck,
  Flag,
  X,
} from 'lucide-react'
import { toast } from 'sonner'

interface ProductReviewsSectionProps {
  productUuid: string
  productName: string
  productImageUrl?: string
  initialSummary?: ReviewSummary | null
  initialReviews?: Review[]
}

export function ProductReviewsSection({
  productUuid,
  productName,
  productImageUrl,
  initialSummary,
  initialReviews = [],
}: ProductReviewsSectionProps) {
  const router = useRouter()
  const { accessToken, user } = useAuthStore()

  const [summary, setSummary] = useState<ReviewSummary | null>(initialSummary || null)
  const [reviews, setReviews] = useState<Review[]>(initialReviews)
  const [loading, setLoading] = useState(false)
  const [activeRatingFilter, setActiveRatingFilter] = useState<number | 'all' | 'media'>('all')
  const [orderBy, setOrderBy] = useState<'created_at' | 'rating' | 'helpful'>('created_at')
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const [currentPage, setCurrentPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [totalRecords, setTotalRecords] = useState(initialSummary?.total_reviews || 0)

  // Write Review State
  const [isWriteModalOpen, setIsWriteModalOpen] = useState(false)
  const [eligibleOrders, setEligibleOrders] = useState<
    Array<{ order_uuid: string; order_number: string; created_at?: string }>
  >([])
  const [checkingEligibility, setCheckingEligibility] = useState(false)

  // Helpful Voting State
  const [votedReviews, setVotedReviews] = useState<Record<string, boolean>>({})
  const [votingReviewUuid, setVotingReviewUuid] = useState<string | null>(null)

  // Report Review State
  const [reportingReview, setReportingReview] = useState<{
    uuid: string
    author?: string
  } | null>(null)

  // Lightbox Preview Image
  const [previewMediaUrl, setPreviewMediaUrl] = useState<string | null>(null)

  const fetchSummary = useCallback(async () => {
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await fetch(`${baseUrl}/api/v1/products/${productUuid}/reviews/summary`)
      if (res.ok) {
        const json = await res.json()
        if (json?.data) {
          setSummary(json.data)
          if (json.data.total_reviews !== undefined) {
            setTotalRecords(json.data.total_reviews)
          }
        }
      }
    } catch {
      // Ignore background refresh errors
    }
  }, [productUuid])

  const fetchReviews = useCallback(async () => {
    setLoading(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const params = new URLSearchParams()
      params.set('page', String(currentPage))
      params.set('limit', '10')

      if (activeRatingFilter !== 'all') {
        if (activeRatingFilter === 'media') {
          params.set('has_media', 'true')
        } else {
          params.set('rating', String(activeRatingFilter))
        }
      }

      params.set('order_by', orderBy)
      params.set('sort', sortOrder)

      const res = await fetch(`${baseUrl}/api/v1/products/${productUuid}/reviews?${params.toString()}`)
      if (res.ok) {
        const json = await res.json()
        const fetchedReviews = json?.data || []
        setReviews(fetchedReviews)

        const pagination = json?.pagination
        if (pagination) {
          setTotalPages(pagination.total_pages || 1)
          setTotalRecords(pagination.total_records || fetchedReviews.length)
        }
      }
    } catch {
      toast.error('Unable to load product reviews.')
    } finally {
      setLoading(false)
    }
  }, [productUuid, currentPage, activeRatingFilter, orderBy, sortOrder])

  useEffect(() => {
    fetchReviews()
  }, [fetchReviews])

  // Toggle Helpful Vote
  const handleToggleHelpful = async (reviewUuid: string) => {
    if (!accessToken) {
      toast.error('Please sign in to vote on customer reviews.')
      router.push(`/login?redirect=${encodeURIComponent(window.location.pathname)}`)
      return
    }

    if (votingReviewUuid) return
    setVotingReviewUuid(reviewUuid)

    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/reviews/${reviewUuid}/helpful`, {
        method: 'POST',
      })

      if (res.ok) {
        const json = await res.json()
        const voteData = json?.data // { voted: boolean, helpful_count: number }
        const isVoted = voteData?.voted ?? !votedReviews[reviewUuid]
        const newCount = voteData?.helpful_count

        setVotedReviews((prev) => ({ ...prev, [reviewUuid]: isVoted }))
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
              : r
          )
        )

        toast.success(isVoted ? 'Helpful vote recorded!' : 'Vote removed.')
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to register helpful vote.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error while registering vote.')
    } finally {
      setVotingReviewUuid(null)
    }
  }

  // Handle Write Review Click
  const handleOpenWriteReview = async () => {
    if (!accessToken) {
      toast.error('Please sign in to review this product.')
      router.push(`/login?redirect=${encodeURIComponent(window.location.pathname)}`)
      return
    }

    setCheckingEligibility(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/orders?limit=100`)
      if (res.ok) {
        const json = await res.json()
        const ordersList: Array<{
          uuid: string
          order_number: string
          created_at?: string
          status?: string
          items?: Array<{ product_uuid?: string }>
        }> = json?.data || []

        // Find orders containing this product
        const matchedOrders = ordersList
          .filter((order) =>
            order.items?.some((item) => item.product_uuid === productUuid)
          )
          .map((ord) => ({
            order_uuid: ord.uuid,
            order_number: ord.order_number,
            created_at: ord.created_at,
          }))

        if (matchedOrders.length > 0) {
          setEligibleOrders(matchedOrders)
          setIsWriteModalOpen(true)
        } else {
          toast.error(
            'You can only review products you have purchased. Please place an order first!'
          )
        }
      } else {
        toast.error('Failed to verify purchase history.')
      }
    } catch {
      toast.error('Unable to verify order eligibility.')
    } finally {
      setCheckingEligibility(false)
    }
  }

  // Breakdown metrics
  const totalReviewsCount = summary?.total_reviews || 0
  const avgRating = summary?.avg_rating || 0
  const fiveStar = summary?.five_star || 0
  const fourStar = summary?.four_star || 0
  const threeStar = summary?.three_star || 0
  const twoStar = summary?.two_star || 0
  const oneStar = summary?.one_star || 0
  const mediaCount = summary?.with_media_count || 0

  const getPercentage = (count: number) => {
    if (totalReviewsCount === 0) return 0
    return Math.round((count / totalReviewsCount) * 100)
  }

  return (
    <div className="space-y-8">
      {/* 1. Rating Summary Header Card */}
      <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-2xs grid grid-cols-1 lg:grid-cols-12 gap-8 items-center">
        {/* Left Column: Big Average Score */}
        <div className="lg:col-span-4 text-center lg:border-r lg:border-slate-100 lg:pr-6 space-y-2.5">
          <div className="inline-flex items-baseline gap-1">
            <span className="text-5xl sm:text-6xl font-black text-[#0D47A1] tracking-tight">
              {avgRating > 0 ? avgRating.toFixed(1) : '5.0'}
            </span>
            <span className="text-slate-400 font-bold text-lg">/ 5.0</span>
          </div>

          <div className="flex justify-center text-amber-400">
            {[1, 2, 3, 4, 5].map((star) => (
              <Star
                key={star}
                className={`w-5 h-5 ${
                  star <= Math.round(avgRating || 5)
                    ? 'fill-amber-400 text-amber-400'
                    : 'text-slate-200'
                }`}
              />
            ))}
          </div>

          <p className="text-xs font-semibold text-slate-500">
            Based on <strong className="text-slate-800">{totalReviewsCount}</strong> verified ratings
          </p>

          <div className="pt-2">
            <button
              type="button"
              onClick={handleOpenWriteReview}
              disabled={checkingEligibility}
              className="inline-flex items-center justify-center gap-2 px-5 py-2.5 rounded-xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-xs disabled:opacity-50 cursor-pointer"
            >
              {checkingEligibility ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Verifying Purchase...</span>
                </>
              ) : (
                <>
                  <Sparkles className="w-4 h-4" />
                  <span>Write a Review</span>
                </>
              )}
            </button>
          </div>
        </div>

        {/* Middle/Right Column: Rating Distribution Bars */}
        <div className="lg:col-span-8 space-y-2.5 text-xs text-slate-600">
          {[
            { star: 5, count: fiveStar },
            { star: 4, count: fourStar },
            { star: 3, count: threeStar },
            { star: 2, count: twoStar },
            { star: 1, count: oneStar },
          ].map(({ star, count }) => {
            const percent = getPercentage(count)
            const isSelected = activeRatingFilter === star

            return (
              <button
                key={star}
                type="button"
                onClick={() => {
                  setActiveRatingFilter(isSelected ? 'all' : star)
                  setCurrentPage(1)
                }}
                className={`w-full flex items-center gap-3 p-1.5 rounded-xl transition-all text-left cursor-pointer ${
                  isSelected
                    ? 'bg-amber-50 ring-1 ring-amber-300 font-bold'
                    : 'hover:bg-slate-50'
                }`}
              >
                <div className="w-16 flex items-center gap-1 shrink-0 font-medium">
                  <span>{star}</span>
                  <Star className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                </div>

                <div className="flex-1 h-2.5 bg-slate-100 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-amber-400 rounded-full transition-all duration-500"
                    style={{ width: `${percent}%` }}
                  />
                </div>

                <div className="w-20 text-right flex items-center justify-end gap-1.5 shrink-0">
                  <span className="text-slate-400 text-[11px]">({count})</span>
                  <span className="w-10 font-bold text-slate-800 text-right">{percent}%</span>
                </div>
              </button>
            )
          })}
        </div>
      </div>

      {/* 2. Filter & Sorting Toolbar */}
      <div className="p-4 rounded-2xl bg-white border border-slate-200/80 shadow-2xs flex flex-col md:flex-row md:items-center justify-between gap-4">
        {/* Rating Pills */}
        <div className="flex items-center gap-2 flex-wrap text-xs">
          <span className="font-bold text-slate-500 flex items-center gap-1.5 mr-1">
            <Filter className="w-3.5 h-3.5 text-[#2196F3]" />
            Filter:
          </span>

          <button
            type="button"
            onClick={() => {
              setActiveRatingFilter('all')
              setCurrentPage(1)
            }}
            className={`px-3 py-1.5 rounded-xl font-bold transition-all cursor-pointer ${
              activeRatingFilter === 'all'
                ? 'bg-[#2196F3] text-white shadow-2xs'
                : 'bg-slate-100 hover:bg-slate-200 text-slate-700'
            }`}
          >
            All ({totalReviewsCount})
          </button>

          {[5, 4, 3, 2, 1].map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => {
                setActiveRatingFilter(s)
                setCurrentPage(1)
              }}
              className={`px-3 py-1.5 rounded-xl font-bold transition-all flex items-center gap-1 cursor-pointer ${
                activeRatingFilter === s
                  ? 'bg-amber-400 text-slate-900 shadow-2xs'
                  : 'bg-slate-100 hover:bg-slate-200 text-slate-700'
              }`}
            >
              <span>{s}</span>
              <Star className="w-3 h-3 fill-amber-400 text-amber-400" />
            </button>
          ))}

          {mediaCount > 0 && (
            <button
              type="button"
              onClick={() => {
                setActiveRatingFilter('media')
                setCurrentPage(1)
              }}
              className={`px-3 py-1.5 rounded-xl font-bold transition-all flex items-center gap-1.5 cursor-pointer ${
                activeRatingFilter === 'media'
                  ? 'bg-purple-600 text-white shadow-2xs'
                  : 'bg-purple-50 hover:bg-purple-100 text-purple-700 border border-purple-200/60'
              }`}
            >
              <Camera className="w-3.5 h-3.5" />
              <span>With Media ({mediaCount})</span>
            </button>
          )}
        </div>

        {/* Sort Dropdown */}
        <div className="flex items-center gap-2 text-xs">
          <span className="text-slate-400 font-medium">Sort by:</span>
          <select
            value={`${orderBy}_${sortOrder}`}
            onChange={(e) => {
              const val = e.target.value
              if (val === 'created_at_desc') {
                setOrderBy('created_at')
                setSortOrder('desc')
              } else if (val === 'rating_desc') {
                setOrderBy('rating')
                setSortOrder('desc')
              } else if (val === 'rating_asc') {
                setOrderBy('rating')
                setSortOrder('asc')
              } else if (val === 'helpful_desc') {
                setOrderBy('helpful')
                setSortOrder('desc')
              }
              setCurrentPage(1)
            }}
            className="px-3 py-1.5 rounded-xl bg-slate-50 border border-slate-200 text-xs font-semibold text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer"
          >
            <option value="created_at_desc">Newest First</option>
            <option value="helpful_desc">Most Helpful</option>
            <option value="rating_desc">Highest Rating</option>
            <option value="rating_asc">Lowest Rating</option>
          </select>
        </div>
      </div>

      {/* 3. Review Cards Stream */}
      {loading ? (
        <div className="p-12 rounded-3xl bg-white border border-slate-200/80 shadow-2xs text-center space-y-3">
          <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin mx-auto" />
          <p className="text-xs text-slate-500 font-medium">Loading verified reviews...</p>
        </div>
      ) : reviews.length > 0 ? (
        <div className="space-y-4">
          {reviews.map((rev) => {
            const hasVoted = votedReviews[rev.uuid]
            const isVoting = votingReviewUuid === rev.uuid

            return (
              <div
                key={rev.uuid}
                className="p-5 sm:p-6 rounded-3xl bg-white border border-slate-200/80 shadow-2xs space-y-4 hover:border-slate-300 transition-all"
              >
                {/* Author & Rating Header */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-2xl bg-gradient-to-tr from-[#0D47A1] to-[#2196F3] text-white font-black text-sm flex items-center justify-center shadow-xs">
                      {rev.reviewer_name?.slice(0, 1) || 'U'}
                    </div>

                    <div className="space-y-0.5">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-bold text-[#0D1B2A]">
                          {rev.reviewer_name || 'Verified Buyer'}
                        </span>
                        <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 text-[10px] font-bold border border-emerald-200/60 flex items-center gap-1">
                          <ShieldCheck className="w-3 h-3 text-emerald-600" />
                          Verified Purchase
                        </span>
                      </div>
                      <p className="text-[11px] text-slate-400 font-mono">{rev.created_at}</p>
                    </div>
                  </div>

                  {/* Rating Stars */}
                  <div className="flex text-amber-400">
                    {[1, 2, 3, 4, 5].map((star) => (
                      <Star
                        key={star}
                        className={`w-4 h-4 ${
                          star <= rev.rating
                            ? 'fill-amber-400 text-amber-400'
                            : 'text-slate-200'
                        }`}
                      />
                    ))}
                  </div>
                </div>

                {/* Review Title & Comment */}
                <div className="space-y-1.5 text-xs text-slate-700">
                  {rev.title && (
                    <h4 className="font-extrabold text-[#0D1B2A] text-sm leading-snug">
                      {rev.title}
                    </h4>
                  )}
                  {rev.comment ? (
                    <p className="leading-relaxed whitespace-pre-line">{rev.comment}</p>
                  ) : (
                    <p className="text-slate-400 italic">No additional comment provided.</p>
                  )}
                </div>

                {/* Media Gallery (Photos / Video) */}
                {rev.images && rev.images.length > 0 && (
                  <div className="flex flex-wrap gap-2.5 pt-1">
                    {rev.images.map((imgUrl, i) => (
                      <button
                        key={i}
                        type="button"
                        onClick={() => setPreviewMediaUrl(getImageUrl(imgUrl))}
                        className="w-16 h-16 sm:w-20 sm:h-20 rounded-xl overflow-hidden border border-slate-200 bg-slate-50 hover:opacity-90 hover:scale-105 transition-all cursor-pointer shadow-2xs"
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

                {/* Seller / Store Reply */}
                {rev.reply && (
                  <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200/80 space-y-1 text-xs">
                    <div className="flex items-center justify-between font-bold text-[#0D47A1]">
                      <div className="flex items-center gap-1.5">
                        <MessageSquare className="w-3.5 h-3.5" />
                        <span>Seller Response</span>
                      </div>
                      {rev.reply_at && (
                        <span className="text-[10px] text-slate-400 font-mono font-normal">
                          {rev.reply_at}
                        </span>
                      )}
                    </div>
                    <p className="text-slate-600 leading-relaxed">{rev.reply}</p>
                  </div>
                )}

                {/* Bottom Helpful & Report Actions */}
                <div className="pt-2 border-t border-slate-100 flex items-center justify-between text-xs">
                  <span className="text-slate-400 text-[11px]">Was this review helpful?</span>

                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => handleToggleHelpful(rev.uuid)}
                      disabled={isVoting}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border text-xs font-bold transition-all cursor-pointer ${
                        hasVoted
                          ? 'bg-blue-50 border-blue-200 text-[#0D47A1]'
                          : 'bg-white border-slate-200 hover:bg-slate-50 text-slate-600'
                      }`}
                    >
                      {isVoting ? (
                        <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      ) : (
                        <ThumbsUp
                          className={`w-3.5 h-3.5 ${hasVoted ? 'fill-blue-600 text-blue-600' : ''}`}
                        />
                      )}
                      <span>Helpful ({rev.helpful_count || 0})</span>
                    </button>

                    <button
                      type="button"
                      onClick={() => {
                        if (!accessToken) {
                          toast.error('Please sign in to report a review.')
                          router.push(`/login?redirect=${encodeURIComponent(window.location.pathname)}`)
                          return
                        }
                        setReportingReview({
                          uuid: rev.uuid,
                          author: rev.reviewer_name,
                        })
                      }}
                      title="Report this review"
                      className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-xl border border-slate-200 bg-white hover:bg-rose-50 hover:border-rose-200 hover:text-rose-600 text-slate-400 text-xs font-semibold transition-all cursor-pointer"
                    >
                      <Flag className="w-3.5 h-3.5" />
                      <span className="hidden sm:inline">Report</span>
                    </button>
                  </div>
                </div>
              </div>
            )
          })}

          {/* Pagination Controls */}
          {totalPages > 1 && (
            <div className="p-4 rounded-2xl bg-white border border-slate-200/80 shadow-2xs flex items-center justify-between text-xs">
              <span className="text-slate-500 font-medium">
                Page <strong className="text-slate-800">{currentPage}</strong> of{' '}
                <strong className="text-slate-800">{totalPages}</strong>
              </span>

              <div className="flex items-center gap-1.5">
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  className="px-3 py-1.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed font-bold text-slate-700 transition-all flex items-center gap-1 cursor-pointer"
                >
                  <ChevronLeft className="w-3.5 h-3.5" />
                  <span>Previous</span>
                </button>

                <button
                  type="button"
                  disabled={currentPage >= totalPages || loading}
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  className="px-3 py-1.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed font-bold text-slate-700 transition-all flex items-center gap-1 cursor-pointer"
                >
                  <span>Next</span>
                  <ChevronRight className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          )}
        </div>
      ) : (
        /* Empty State */
        <div className="p-10 rounded-3xl bg-white border border-slate-200/80 shadow-2xs text-center space-y-4">
          <div className="w-14 h-14 rounded-full bg-blue-50 text-[#2196F3] flex items-center justify-center mx-auto">
            <MessageSquare className="w-7 h-7" />
          </div>
          <div className="space-y-1">
            <h3 className="text-base font-extrabold text-[#0D1B2A]">No Reviews Found</h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto">
              {activeRatingFilter !== 'all'
                ? 'No customer reviews match the selected star filter. Try selecting another rating.'
                : 'Be the first customer to share your thoughts and help others make the right choice!'}
            </p>
          </div>
          {activeRatingFilter !== 'all' ? (
            <button
              type="button"
              onClick={() => setActiveRatingFilter('all')}
              className="px-4 py-2 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all cursor-pointer"
            >
              Clear Filter
            </button>
          ) : (
            <button
              type="button"
              onClick={handleOpenWriteReview}
              disabled={checkingEligibility}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-xs cursor-pointer disabled:opacity-50"
            >
              <Sparkles className="w-4 h-4" />
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
          fetchReviews()
          fetchSummary()
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
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs animate-fade-in"
          onClick={() => setPreviewMediaUrl(null)}
        >
          <div className="relative max-w-3xl max-h-[85vh]">
            <img
              src={previewMediaUrl}
              alt="Expanded preview"
              className="max-w-full max-h-[85vh] rounded-2xl object-contain shadow-2xl"
            />
            <button
              type="button"
              onClick={() => setPreviewMediaUrl(null)}
              className="absolute top-3 right-3 w-8 h-8 rounded-full bg-black/60 text-white flex items-center justify-center hover:bg-black/80 transition-colors cursor-pointer"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
