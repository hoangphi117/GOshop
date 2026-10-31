import { useState } from 'react'
import { AlertTriangle, ShieldCheck, EyeOff, Trash2, Clock, User, MessageSquare } from 'lucide-react'
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
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useReviewReports, useReviewMutations } from '../hooks/use-reviews'
import type { AdminReview, ReviewReport } from '../types'

interface ReviewReportsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  review: AdminReview | null
  onRefresh?: () => void
}

export function ReviewReportsDialog({
  open,
  onOpenChange,
  review,
  onRefresh,
}: ReviewReportsDialogProps) {
  const { data: reports = [], isLoading } = useReviewReports(review?.uuid || null)
  const { dismissReports, updateStatus, forceDeleteReview } = useReviewMutations()
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

  const handleDismiss = async () => {
    if (!review?.uuid) return
    try {
      await dismissReports.mutateAsync(review.uuid)
      toast.success('Reports dismissed and review visibility restored')
      onOpenChange(false)
      onRefresh?.()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to dismiss reports'
      toast.error(msg)
    }
  }

  const handleHide = async () => {
    if (!review?.uuid) return
    try {
      await updateStatus.mutateAsync({
        uuid: review.uuid,
        status: 2, // Hidden
      })
      toast.success('Review status set to Hidden')
      onOpenChange(false)
      onRefresh?.()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to hide review'
      toast.error(msg)
    }
  }

  const handleConfirmDelete = async () => {
    if (!review?.uuid) return
    try {
      await forceDeleteReview.mutateAsync(review.uuid)
      toast.success('Review permanently deleted')
      setDeleteConfirmOpen(false)
      onOpenChange(false)
      onRefresh?.()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete review'
      toast.error(msg)
    }
  }

  const getReasonBadge = (reason: string) => {
    switch (reason.toLowerCase()) {
      case 'spam':
        return <Badge variant="secondary" className="bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300">Spam</Badge>
      case 'inappropriate':
        return <Badge variant="destructive">Inappropriate</Badge>
      case 'fake':
        return <Badge variant="secondary" className="bg-purple-100 text-purple-800 dark:bg-purple-950 dark:text-purple-300">Fake</Badge>
      default:
        return <Badge variant="outline">{reason}</Badge>
    }
  }

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleString('en-US', {
        dateStyle: 'medium',
        timeStyle: 'short',
      })
    } catch {
      return dateStr
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-[650px] max-h-[85vh] flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-5 w-5" />
              Reported Review Moderation
            </DialogTitle>
            <DialogDescription>
              Review the reports filed against this content and select an appropriate moderation action.
            </DialogDescription>
          </DialogHeader>

          {review && (
            <div className="rounded-lg border bg-muted/30 p-3 text-sm space-y-1.5 shrink-0">
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span className="font-semibold text-foreground">{review.product_name}</span>
                <span className="text-amber-500 font-bold">★ {review.rating}/5</span>
              </div>
              <div className="text-xs">
                <span className="text-muted-foreground">Author: </span>
                <span className="font-medium text-foreground">{review.reviewer_name}</span> ({review.reviewer_email})
              </div>
              {review.title && <p className="font-medium text-foreground">{review.title}</p>}
              {review.comment && <p className="text-muted-foreground text-xs">{review.comment}</p>}
            </div>
          )}

          <div className="flex-1 overflow-y-auto space-y-3 py-2">
            <div className="flex items-center justify-between">
              <h4 className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                User Reports ({reports.length})
              </h4>
            </div>

            {isLoading ? (
              <div className="text-center py-6 text-sm text-muted-foreground">Loading reports...</div>
            ) : reports.length === 0 ? (
              <div className="text-center py-6 text-sm text-muted-foreground border rounded-lg border-dashed">
                No reports found for this review.
              </div>
            ) : (
              <div className="space-y-2.5">
                {(reports as ReviewReport[]).map((rep) => (
                  <div key={rep.report_id} className="border rounded-lg p-3 bg-card space-y-2 text-xs">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-1.5 font-medium">
                        <User className="h-3.5 w-3.5 text-muted-foreground" />
                        <span>{rep.user_name || 'Anonymous User'}</span>
                        <span className="text-muted-foreground">({rep.user_email})</span>
                      </div>
                      <div className="flex items-center gap-2">
                        {getReasonBadge(rep.reason)}
                        <span className="text-muted-foreground flex items-center gap-1">
                          <Clock className="h-3 w-3" />
                          {formatDate(rep.created_at)}
                        </span>
                      </div>
                    </div>
                    {rep.details && (
                      <div className="bg-muted/50 rounded p-2 text-muted-foreground flex items-start gap-1.5">
                        <MessageSquare className="h-3.5 w-3.5 mt-0.5 shrink-0" />
                        <span>{rep.details}</span>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          <DialogFooter className="flex-wrap gap-2 sm:justify-between border-t pt-3 mt-2">
            <div className="flex gap-2">
              <Button
                type="button"
                variant="destructive"
                size="sm"
                onClick={() => setDeleteConfirmOpen(true)}
                disabled={forceDeleteReview.isPending}
              >
                <Trash2 className="mr-1.5 h-4 w-4" />
                Delete Review
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={handleHide}
                disabled={updateStatus.isPending}
              >
                <EyeOff className="mr-1.5 h-4 w-4" />
                Hide Review
              </Button>
            </div>
            <div className="flex gap-2">
              <Button type="button" variant="outline" size="sm" onClick={() => onOpenChange(false)}>
                Close
              </Button>
              <Button
                type="button"
                size="sm"
                onClick={handleDismiss}
                disabled={dismissReports.isPending}
                className="bg-emerald-600 hover:bg-emerald-700 text-white"
              >
                <ShieldCheck className="mr-1.5 h-4 w-4" />
                Dismiss & Keep Visible
              </Button>
            </div>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Permanently Delete Review"
        desc="Are you sure you want to permanently delete this review? This action will remove the review and all uploaded media files from the server and cannot be undone."
        confirmText="Delete Review"
        destructive
        handleConfirm={handleConfirmDelete}
        isLoading={forceDeleteReview.isPending}
      />
    </>
  )
}
