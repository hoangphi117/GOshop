import { useState } from 'react'
import {
  Star,
  Calendar,
  MessageSquare,
  Eye,
  EyeOff,
  AlertTriangle,
  ThumbsUp,
  Image as ImageIcon,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useReviewMutations } from '../hooks/use-reviews'
import type { AdminReview, ReviewStatus } from '../types'

interface ReviewDetailModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  review: AdminReview | null
  onOpenReply?: (review: AdminReview) => void
  onOpenReports?: (review: AdminReview) => void
}

export function ReviewDetailModal({
  open,
  onOpenChange,
  review,
  onOpenReply,
  onOpenReports,
}: ReviewDetailModalProps) {
  const { updateStatus } = useReviewMutations()
  const [selectedImage, setSelectedImage] = useState<string | null>(null)

  if (!review) return null

  const handleStatusChange = async (newStatus: ReviewStatus) => {
    try {
      await updateStatus.mutateAsync({
        uuid: review.uuid,
        status: newStatus,
      })
      toast.success(
        `Review status updated to ${newStatus === 1 ? 'Visible' : newStatus === 2 ? 'Hidden' : 'Reported'}`
      )
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update review status'
      toast.error(msg)
    }
  }

  const getStatusBadge = (status: ReviewStatus) => {
    switch (status) {
      case 1:
        return (
          <Badge variant="outline" className="bg-emerald-50 text-emerald-700 border-emerald-300 dark:bg-emerald-950 dark:text-emerald-300">
            Visible
          </Badge>
        )
      case 2:
        return (
          <Badge variant="outline" className="bg-zinc-100 text-zinc-700 border-zinc-300 dark:bg-zinc-800 dark:text-zinc-300">
            Hidden
          </Badge>
        )
      case 3:
        return (
          <Badge variant="destructive" className="flex items-center gap-1">
            <AlertTriangle className="h-3 w-3" />
            Reported
          </Badge>
        )
      default:
        return <Badge variant="secondary">Unknown</Badge>
    }
  }

  const formatDate = (d?: string) => {
    if (!d) return '—'
    try {
      return new Date(d).toLocaleString('en-US', {
        dateStyle: 'medium',
        timeStyle: 'short',
      })
    } catch {
      return d
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-[700px] max-h-[90vh] flex flex-col">
          <DialogHeader>
            <div className="flex items-center justify-between pr-4">
              <DialogTitle className="flex items-center gap-2 text-lg">
                Review Details
              </DialogTitle>
              {getStatusBadge(review.status)}
            </div>
            <DialogDescription className="text-xs text-muted-foreground">
              Review ID: <span className="font-mono">{review.uuid}</span>
            </DialogDescription>
          </DialogHeader>

          <div className="flex-1 overflow-y-auto space-y-4 py-2 text-sm pr-1">
            {/* Header Stats / Product & Customer info */}
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 p-3 bg-muted/40 rounded-lg border text-xs">
              <div>
                <span className="text-muted-foreground block">Customer:</span>
                <span className="font-medium text-foreground text-sm">{review.reviewer_name}</span>
                <span className="text-muted-foreground block truncate">{review.reviewer_email}</span>
              </div>
              <div>
                <span className="text-muted-foreground block">Product:</span>
                <span className="font-medium text-foreground text-sm line-clamp-1">
                  {review.product_name}
                </span>
                <span className="text-muted-foreground block font-mono text-[11px] truncate">
                  {review.product_uuid}
                </span>
              </div>
            </div>

            {/* Rating and Title */}
            <div className="space-y-1">
              <div className="flex items-center gap-2">
                <div className="flex items-center gap-0.5 text-amber-500">
                  {[...Array(5)].map((_, i) => (
                    <Star
                      key={i}
                      className={`h-4 w-4 ${
                        i < review.rating ? 'fill-amber-500 text-amber-500' : 'text-zinc-300 dark:text-zinc-600'
                      }`}
                    />
                  ))}
                </div>
                <span className="font-bold text-sm">{review.rating} out of 5</span>
              </div>
              {review.title && <h3 className="font-semibold text-base">{review.title}</h3>}
            </div>

            {/* Review Comment */}
            <div className="bg-card border rounded-lg p-3">
              <p className="text-foreground text-sm whitespace-pre-wrap leading-relaxed">
                {review.comment || <span className="italic text-muted-foreground">No written comment provided.</span>}
              </p>
            </div>

            {/* Media Gallery (Images & Video) */}
            {((review.images && review.images.length > 0) || review.video_url) && (
              <div className="space-y-2">
                <h4 className="text-xs font-bold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                  <ImageIcon className="h-3.5 w-3.5" /> Attached Media ({review.images?.length || 0} images)
                </h4>
                <div className="flex flex-wrap gap-2">
                  {review.images?.map((imgUrl, idx) => (
                    <button
                      key={idx}
                      type="button"
                      onClick={() => setSelectedImage(imgUrl)}
                      className="relative h-20 w-20 rounded-md border overflow-hidden hover:opacity-80 transition group"
                    >
                      <img
                        src={imgUrl}
                        alt={`Review media ${idx + 1}`}
                        className="h-full w-full object-cover"
                      />
                    </button>
                  ))}
                  {review.video_url && (
                    <div className="relative h-20 w-32 rounded-md border overflow-hidden bg-black flex items-center justify-center">
                      <video src={review.video_url} className="h-full w-full object-cover" controls />
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Store Reply */}
            <div className="rounded-lg border bg-blue-50/50 dark:bg-blue-950/20 p-3 space-y-2">
              <div className="flex items-center justify-between">
                <h4 className="text-xs font-bold uppercase tracking-wider text-blue-800 dark:text-blue-300 flex items-center gap-1.5">
                  <MessageSquare className="h-3.5 w-3.5" /> Official Store Response
                </h4>
                {review.reply_at && (
                  <span className="text-[11px] text-muted-foreground flex items-center gap-1">
                    <Calendar className="h-3 w-3" /> {formatDate(review.reply_at)}
                  </span>
                )}
              </div>
              {review.reply ? (
                <p className="text-xs text-foreground bg-background/80 rounded p-2.5 border">
                  {review.reply}
                </p>
              ) : (
                <p className="text-xs italic text-muted-foreground">No response posted yet.</p>
              )}
            </div>

            {/* Metrics and Timestamps */}
            <div className="flex flex-wrap items-center justify-between text-xs text-muted-foreground border-t pt-2 gap-2">
              <div className="flex items-center gap-3">
                <span className="flex items-center gap-1">
                  <ThumbsUp className="h-3.5 w-3.5" /> {review.helpful_count} helpful votes
                </span>
                {review.report_count > 0 && (
                  <span className="text-destructive font-medium flex items-center gap-1">
                    <AlertTriangle className="h-3.5 w-3.5" /> {review.report_count} reports
                  </span>
                )}
              </div>
              <div>
                <span>Posted on: {formatDate(review.created_at)}</span>
              </div>
            </div>
          </div>

          <DialogFooter className="flex-wrap gap-2 sm:justify-between border-t pt-3">
            <div className="flex items-center gap-2">
              <span className="text-xs text-muted-foreground mr-1">Status:</span>
              <Button
                type="button"
                variant={review.status === 1 ? 'default' : 'outline'}
                size="sm"
                className="h-8 text-xs"
                onClick={() => handleStatusChange(1)}
                disabled={updateStatus.isPending || review.status === 1}
              >
                <Eye className="mr-1 h-3.5 w-3.5" /> Visible
              </Button>
              <Button
                type="button"
                variant={review.status === 2 ? 'default' : 'outline'}
                size="sm"
                className="h-8 text-xs"
                onClick={() => handleStatusChange(2)}
                disabled={updateStatus.isPending || review.status === 2}
              >
                <EyeOff className="mr-1 h-3.5 w-3.5" /> Hidden
              </Button>
            </div>

            <div className="flex items-center gap-2">
              {review.report_count > 0 && onOpenReports && (
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  className="h-8 text-xs"
                  onClick={() => {
                    onOpenChange(false)
                    onOpenReports(review)
                  }}
                >
                  <AlertTriangle className="mr-1.5 h-3.5 w-3.5" /> View Reports ({review.report_count})
                </Button>
              )}
              {onOpenReply && (
                <Button
                  type="button"
                  size="sm"
                  className="h-8 text-xs"
                  onClick={() => {
                    onOpenChange(false)
                    onOpenReply(review)
                  }}
                >
                  <MessageSquare className="mr-1.5 h-3.5 w-3.5" />
                  {review.reply ? 'Edit Reply' : 'Post Reply'}
                </Button>
              )}
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Media Lightbox Zoom */}
      {selectedImage && (
        <Dialog open={Boolean(selectedImage)} onOpenChange={() => setSelectedImage(null)}>
          <DialogContent className="max-w-[85vw] max-h-[90vh] p-1 bg-black/90 border-none flex items-center justify-center">
            <img
              src={selectedImage}
              alt="Zoomed review media"
              className="max-h-[85vh] max-w-[80vw] object-contain rounded"
            />
          </DialogContent>
        </Dialog>
      )}
    </>
  )
}
