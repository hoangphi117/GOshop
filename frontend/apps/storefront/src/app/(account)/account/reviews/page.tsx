'use client'

import React, { useState, useEffect, useCallback } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { authFetch } from '@/lib/api'
import { getImageUrl, extractErrorMessage } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { useMounted } from '@/hooks/use-mounted'
import {
  Star,
  MessageSquare,
  Sparkles,
  ShoppingBag,
  ExternalLink,
  Edit3,
  Trash2,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Loader2,
  AlertCircle,
  AlertTriangle,
  Clock,
  CheckCircle2,
  X,
  Camera,
} from 'lucide-react'
import { toast } from 'sonner'

export interface MyReview {
  uuid: string
  product_uuid: string
  product_name: string
  product_slug: string
  product_image_url?: string
  rating: number
  title?: string
  comment?: string
  images: string[]
  video_url?: string
  has_media: boolean
  can_edit: boolean
  created_at: string
}

export default function MyReviewsPage() {
  const router = useRouter()
  const { accessToken } = useAuthStore()
  const mounted = useMounted()

  const [reviews, setReviews] = useState<MyReview[]>([])
  const [loading, setLoading] = useState(true)
  const [currentPage, setCurrentPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [totalRecords, setTotalRecords] = useState(0)
  const limit = 10

  // Edit Modal State
  const [editingReview, setEditingReview] = useState<MyReview | null>(null)
  const [editRating, setEditRating] = useState(5)
  const [editHoverRating, setEditHoverRating] = useState(0)
  const [editTitle, setEditTitle] = useState('')
  const [editComment, setEditComment] = useState('')
  const [isUpdating, setIsUpdating] = useState(false)

  // Delete Confirmation State
  const [deletingReview, setDeletingReview] = useState<MyReview | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  // Lightbox Preview State
  const [previewMediaUrl, setPreviewMediaUrl] = useState<string | null>(null)

  const fetchMyReviews = useCallback(async (page: number) => {
    if (!accessToken) return
    setLoading(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/reviews/me?page=${page}&limit=${limit}`)
      if (res.ok) {
        const json = await res.json()
        const data: MyReview[] = json?.data || []
        setReviews(data)

        const pagination = json?.pagination
        if (pagination) {
          setTotalPages(pagination.total_pages || 1)
          setTotalRecords(pagination.total_records || data.length)
        } else {
          setTotalRecords(data.length)
          setTotalPages(Math.ceil(data.length / limit) || 1)
        }
      } else {
        const errJson = await res.json().catch(() => null)
        toast.error(extractErrorMessage(errJson, 'Failed to fetch your reviews.'))
      }
    } catch {
      toast.error('Network error loading your reviews.')
    } finally {
      setLoading(false)
    }
  }, [accessToken, limit])

  useEffect(() => {
    if (mounted && accessToken) {
      fetchMyReviews(currentPage)
    }
  }, [mounted, accessToken, currentPage, fetchMyReviews])

  // Open Edit Modal
  const handleOpenEdit = (rev: MyReview) => {
    if (!rev.can_edit) {
      toast.error('This review cannot be edited because the 30-day editing window has expired.')
      return
    }
    setEditingReview(rev)
    setEditRating(rev.rating)
    setEditTitle(rev.title || '')
    setEditComment(rev.comment || '')
  }

  // Submit Edit
  const handleSaveEdit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!editingReview) return

    setIsUpdating(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const payload: { rating: number; title?: string; comment?: string } = {
        rating: editRating,
      }
      if (editTitle.trim()) payload.title = editTitle.trim()
      if (editComment.trim()) payload.comment = editComment.trim()

      const res = await authFetch(`${baseUrl}/api/v1/reviews/${editingReview.uuid}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      })

      if (res.ok) {
        toast.success('Your review has been updated successfully!')
        setEditingReview(null)
        fetchMyReviews(currentPage)
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to update review.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error while saving review.')
    } finally {
      setIsUpdating(false)
    }
  }

  // Submit Delete
  const handleConfirmDelete = async () => {
    if (!deletingReview) return

    setIsDeleting(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await authFetch(`${baseUrl}/api/v1/reviews/${deletingReview.uuid}`, {
        method: 'DELETE',
      })

      if (res.ok) {
        toast.success('Review deleted successfully.')
        setDeletingReview(null)
        // If this was the only review on the page and not page 1, go to previous page
        if (reviews.length === 1 && currentPage > 1) {
          setCurrentPage((p) => p - 1)
        } else {
          fetchMyReviews(currentPage)
        }
      } else {
        const errJson = await res.json().catch(() => null)
        const msg = extractErrorMessage(errJson, 'Failed to delete review.')
        toast.error(msg)
      }
    } catch {
      toast.error('Network error while deleting review.')
    } finally {
      setIsDeleting(false)
    }
  }

  // Pagination page numbers generator
  const getPaginationPages = () => {
    const pages: (number | string)[] = []
    const delta = 2

    for (let i = 1; i <= totalPages; i++) {
      if (
        i === 1 ||
        i === totalPages ||
        (i >= currentPage - delta && i <= currentPage + delta)
      ) {
        pages.push(i)
      } else if (
        (i === currentPage - delta - 1 || i === currentPage + delta + 1) &&
        pages[pages.length - 1] !== '...'
      ) {
        pages.push('...')
      }
    }

    return pages
  }

  if (!mounted) {
    return (
      <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-2xs flex items-center justify-center min-h-[300px]">
        <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Top Header Card */}
      <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-2xs flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl sm:text-2xl font-black text-[#0D1B2A] tracking-tight">
              My Reviews
            </h1>
            <span className="px-2.5 py-0.5 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-bold">
              {totalRecords} {totalRecords === 1 ? 'Review' : 'Reviews'}
            </span>
          </div>
          <p className="text-xs text-slate-500 mt-1">
            Manage, edit, or remove your submitted product reviews and ratings.
          </p>
        </div>

        <Link
          href="/account/orders"
          className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold transition-all self-start sm:self-auto cursor-pointer"
        >
          <ShoppingBag className="w-4 h-4 text-[#2196F3]" />
          <span>Review Other Orders</span>
        </Link>
      </div>

      {/* Content Stream */}
      {loading ? (
        <div className="p-12 rounded-3xl bg-white border border-slate-200/80 shadow-2xs text-center space-y-3">
          <Loader2 className="w-8 h-8 text-[#2196F3] animate-spin mx-auto" />
          <p className="text-xs text-slate-500 font-medium">Loading your reviews...</p>
        </div>
      ) : reviews.length > 0 ? (
        <div className="space-y-4">
          {reviews.map((rev) => (
            <div
              key={rev.uuid}
              className="p-5 sm:p-6 rounded-3xl bg-white border border-slate-200/80 shadow-2xs space-y-4 hover:border-slate-300 transition-all"
            >
              {/* Product Header & Rating */}
              <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3 pb-3 border-b border-slate-100">
                <div className="flex items-center gap-3.5">
                  <div className="w-14 h-14 rounded-2xl bg-slate-100 border border-slate-200/80 overflow-hidden shrink-0 flex items-center justify-center shadow-2xs">
                    {rev.product_image_url ? (
                      <img
                        src={getImageUrl(rev.product_image_url)}
                        alt={rev.product_name}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <ShoppingBag className="w-6 h-6 text-slate-400" />
                    )}
                  </div>

                  <div className="space-y-1 min-w-0">
                    <Link
                      href={`/products/${rev.product_slug}`}
                      className="font-bold text-[#0D1B2A] text-sm hover:text-[#2196F3] transition-colors line-clamp-1 inline-flex items-center gap-1.5 group"
                    >
                      <span>{rev.product_name}</span>
                      <ExternalLink className="w-3.5 h-3.5 text-slate-400 group-hover:text-[#2196F3] shrink-0" />
                    </Link>
                    <div className="flex items-center gap-2 text-[11px] text-slate-400 font-mono">
                      <span>Submitted on {rev.created_at}</span>
                      {rev.can_edit ? (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-700 text-[10px] font-bold border border-emerald-200/60 font-sans">
                          <CheckCircle2 className="w-3 h-3" />
                          Editable
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-slate-100 text-slate-500 text-[10px] font-bold font-sans">
                          <Clock className="w-3 h-3" />
                          Locked (30+ days)
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Rating Display */}
                <div className="flex items-center gap-1.5 self-start sm:self-auto">
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
                  <span className="text-xs font-black text-[#0D1B2A] ml-1">
                    {rev.rating}.0
                  </span>
                </div>
              </div>

              {/* Review Title & Comment */}
              <div className="space-y-1.5 text-xs text-slate-700">
                {rev.title && (
                  <h3 className="font-extrabold text-[#0D1B2A] text-sm">
                    {rev.title}
                  </h3>
                )}
                {rev.comment ? (
                  <p className="leading-relaxed whitespace-pre-line text-slate-600">
                    {rev.comment}
                  </p>
                ) : (
                  <p className="text-slate-400 italic">No written comment provided.</p>
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

              {/* Card Actions Footer */}
              <div className="pt-3 border-t border-slate-100 flex items-center justify-between">
                <Link
                  href={`/products/${rev.product_slug}`}
                  className="text-xs font-bold text-[#2196F3] hover:text-[#0D47A1] inline-flex items-center gap-1 transition-colors"
                >
                  <span>View Product Page</span>
                  <ExternalLink className="w-3 h-3" />
                </Link>

                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => handleOpenEdit(rev)}
                    disabled={!rev.can_edit}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:hover:bg-white text-xs font-bold text-slate-700 transition-all cursor-pointer disabled:cursor-not-allowed"
                    title={
                      rev.can_edit
                        ? 'Edit your review rating or comment'
                        : 'Editing is only allowed within 30 days of submission'
                    }
                  >
                    <Edit3 className="w-3.5 h-3.5 text-slate-500" />
                    <span>Edit</span>
                  </button>

                  <button
                    type="button"
                    onClick={() => setDeletingReview(rev)}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-rose-200 bg-rose-50 hover:bg-rose-100 text-xs font-bold text-rose-600 transition-all cursor-pointer"
                    title="Delete this review"
                  >
                    <Trash2 className="w-3.5 h-3.5 text-rose-500" />
                    <span>Delete</span>
                  </button>
                </div>
              </div>
            </div>
          ))}

          {/* Full Pagination Controls */}
          {totalPages > 1 && (
            <div className="p-4 rounded-3xl bg-white border border-slate-200/80 shadow-2xs flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
              <span className="text-slate-500 font-medium text-center sm:text-left">
                Showing page <strong className="text-slate-800">{currentPage}</strong> of{' '}
                <strong className="text-slate-800">{totalPages}</strong> ({totalRecords} total items)
              </span>

              <div className="flex items-center justify-center gap-1">
                {/* First Page */}
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage(1)}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="First Page"
                >
                  <ChevronsLeft className="w-4 h-4" />
                </button>

                {/* Prev Page */}
                <button
                  type="button"
                  disabled={currentPage <= 1 || loading}
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="Previous Page"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>

                {/* Numbered Buttons */}
                {getPaginationPages().map((pageNum, idx) => {
                  if (pageNum === '...') {
                    return (
                      <span
                        key={`dots-${idx}`}
                        className="w-8 h-8 flex items-center justify-center text-slate-400 font-bold"
                      >
                        ...
                      </span>
                    )
                  }

                  const pageNumber = Number(pageNum)
                  const isActive = pageNumber === currentPage

                  return (
                    <button
                      key={pageNumber}
                      type="button"
                      disabled={loading}
                      onClick={() => setCurrentPage(pageNumber)}
                      className={`w-8 h-8 rounded-xl text-xs font-bold transition-all cursor-pointer ${
                        isActive
                          ? 'bg-[#2196F3] text-white shadow-2xs'
                          : 'border border-slate-200 bg-white hover:bg-slate-50 text-slate-700'
                      }`}
                    >
                      {pageNumber}
                    </button>
                  )
                })}

                {/* Next Page */}
                <button
                  type="button"
                  disabled={currentPage >= totalPages || loading}
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
                  title="Next Page"
                >
                  <ChevronRight className="w-4 h-4" />
                </button>

                {/* Last Page */}
                <button
                  type="button"
                  disabled={currentPage >= totalPages || loading}
                  onClick={() => setCurrentPage(totalPages)}
                  className="w-8 h-8 rounded-xl border border-slate-200 bg-white hover:bg-slate-50 disabled:opacity-40 disabled:cursor-not-allowed text-slate-700 font-bold flex items-center justify-center transition-all cursor-pointer"
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
        <div className="p-12 sm:p-16 rounded-3xl bg-white border border-slate-200/80 shadow-2xs text-center space-y-4">
          <div className="w-16 h-16 rounded-3xl bg-blue-50 text-[#2196F3] flex items-center justify-center mx-auto shadow-xs">
            <MessageSquare className="w-8 h-8" />
          </div>
          <div className="space-y-1 max-w-md mx-auto">
            <h3 className="text-lg font-extrabold text-[#0D1B2A]">No Reviews Yet</h3>
            <p className="text-xs text-slate-500 leading-relaxed">
              You haven&apos;t written any product reviews yet. Reviewing your delivered orders helps the community make informed decisions and unlocks verified customer badges!
            </p>
          </div>
          <div className="pt-2">
            <Link
              href="/account/orders"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-2xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-md cursor-pointer"
            >
              <ShoppingBag className="w-4 h-4" />
              <span>Go to My Orders</span>
            </Link>
          </div>
        </div>
      )}

      {/* Edit Review Modal */}
      {editingReview && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-fade-in">
          <div
            className="relative w-full max-w-lg bg-white rounded-3xl shadow-2xl border border-slate-200 overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="p-6 border-b border-slate-100 flex items-center justify-between bg-slate-50/50">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-2xl bg-[#E3F2FD] text-[#0D47A1] flex items-center justify-center shadow-xs">
                  <Edit3 className="w-5 h-5" />
                </div>
                <div>
                  <h2 className="text-base font-extrabold text-[#0D1B2A]">Edit Review</h2>
                  <p className="text-xs text-slate-500 truncate max-w-xs">
                    {editingReview.product_name}
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setEditingReview(null)}
                className="w-8 h-8 rounded-full bg-slate-100 text-slate-400 hover:text-slate-600 hover:bg-slate-200 flex items-center justify-center transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSaveEdit} className="p-6 space-y-5">
              {/* Star Rating Picker */}
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-700 block">
                  Overall Rating <span className="text-rose-500">*</span>
                </label>
                <div className="flex items-center gap-2">
                  <div className="flex items-center gap-1">
                    {[1, 2, 3, 4, 5].map((star) => {
                      const isFilled =
                        (editHoverRating || editRating) >= star
                      return (
                        <button
                          key={star}
                          type="button"
                          onClick={() => setEditRating(star)}
                          onMouseEnter={() => setEditHoverRating(star)}
                          onMouseLeave={() => setEditHoverRating(0)}
                          className="p-1 -m-1 text-slate-200 hover:scale-110 transition-transform cursor-pointer"
                        >
                          <Star
                            className={`w-7 h-7 ${
                              isFilled
                                ? 'fill-amber-400 text-amber-400'
                                : 'text-slate-200'
                            }`}
                          />
                        </button>
                      )
                    })}
                  </div>
                  <span className="text-xs font-bold text-slate-700 ml-2">
                    {editRating === 5 && '⭐️⭐️⭐️⭐️⭐️ Excellent'}
                    {editRating === 4 && '⭐️⭐️⭐️⭐️ Great'}
                    {editRating === 3 && '⭐️⭐️⭐️ Average'}
                    {editRating === 2 && '⭐️⭐️ Poor'}
                    {editRating === 1 && '⭐️ Terrible'}
                  </span>
                </div>
              </div>

              {/* Review Title */}
              <div className="space-y-1.5">
                <label className="text-xs font-bold text-slate-700 block">
                  Review Headline <span className="text-slate-400 font-normal">(Optional)</span>
                </label>
                <input
                  type="text"
                  maxLength={200}
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  placeholder="e.g. Excellent build quality and fast shipping!"
                  className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] focus:bg-white"
                />
              </div>

              {/* Review Comment */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between text-xs">
                  <label className="font-bold text-slate-700">
                    Review Details <span className="text-slate-400 font-normal">(Optional)</span>
                  </label>
                  <span className="text-[11px] text-slate-400 font-mono">
                    {editComment.length}/2000
                  </span>
                </div>
                <textarea
                  rows={4}
                  maxLength={2000}
                  value={editComment}
                  onChange={(e) => setEditComment(e.target.value)}
                  placeholder="Share details about what you liked or disliked about this product..."
                  className="w-full px-3.5 py-2.5 rounded-2xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] focus:bg-white resize-none"
                />
              </div>

              {/* Action Buttons */}
              <div className="pt-2 flex items-center justify-end gap-3 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => setEditingReview(null)}
                  disabled={isUpdating}
                  className="px-5 py-2.5 rounded-xl border border-slate-200 text-slate-600 text-xs font-bold hover:bg-slate-50 transition-all cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isUpdating}
                  className="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl bg-[#2196F3] hover:bg-[#0D47A1] text-white text-xs font-bold transition-all shadow-md cursor-pointer disabled:opacity-50"
                >
                  {isUpdating ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>Saving Changes...</span>
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
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-fade-in">
          <div
            className="relative w-full max-w-md bg-white rounded-3xl shadow-2xl border border-slate-200 p-6 space-y-5"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="w-12 h-12 rounded-2xl bg-rose-50 border border-rose-200 text-rose-600 flex items-center justify-center mx-auto shadow-xs">
              <AlertTriangle className="w-6 h-6" />
            </div>

            <div className="text-center space-y-1.5">
              <h3 className="text-base font-extrabold text-[#0D1B2A]">
                Delete Review?
              </h3>
              <p className="text-xs text-slate-500 leading-relaxed">
                Are you sure you want to delete your review for{' '}
                <strong className="text-slate-800 font-bold">
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
                className="w-1/2 py-2.5 rounded-xl border border-slate-200 text-slate-600 text-xs font-bold hover:bg-slate-50 transition-all cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmDelete}
                disabled={isDeleting}
                className="w-1/2 inline-flex items-center justify-center gap-2 py-2.5 rounded-xl bg-rose-600 hover:bg-rose-700 text-white text-xs font-bold transition-all shadow-md cursor-pointer disabled:opacity-50"
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
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-xs animate-fade-in"
          onClick={() => setPreviewMediaUrl(null)}
        >
          <div className="relative max-w-3xl max-h-[85vh]">
            <img
              src={previewMediaUrl}
              alt="Expanded photo preview"
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
