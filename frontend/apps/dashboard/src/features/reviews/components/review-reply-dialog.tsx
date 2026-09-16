import { useState, useEffect } from 'react'
import { MessageSquareReply, Sparkles, Send } from 'lucide-react'
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
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { useReviewMutations } from '../hooks/use-reviews'
import type { AdminReview } from '../types'

interface ReviewReplyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  review: AdminReview | null
}

const QUICK_TEMPLATES = [
  {
    label: 'Thank you (5★)',
    text: 'Thank you so much for your wonderful feedback! We are thrilled to hear that you love our product and look forward to serving you again soon.',
  },
  {
    label: 'Apology & Resolution',
    text: 'We are truly sorry to hear about your experience. Please reach out to our customer support team directly with your order details so we can resolve this for you immediately.',
  },
  {
    label: 'Appreciation',
    text: 'Thank you for taking the time to share your honest review. Your feedback helps us continuously improve our product quality and customer service.',
  },
]

export function ReviewReplyDialog({ open, onOpenChange, review }: ReviewReplyDialogProps) {
  const [replyText, setReplyText] = useState('')
  const { replyReview } = useReviewMutations()

  useEffect(() => {
    if (review) {
      setReplyText(review.reply || '')
    } else {
      setReplyText('')
    }
  }, [review, open])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!review?.uuid || !replyText.trim()) return

    try {
      await replyReview.mutateAsync({
        uuid: review.uuid,
        payload: { reply: replyText.trim() },
      })
      toast.success('Reply submitted successfully')
      onOpenChange(false)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to post reply'
      toast.error(msg)
    }
  }

  const handleApplyTemplate = (text: string) => {
    setReplyText(text)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[600px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <MessageSquareReply className="h-5 w-5 text-primary" />
            {review?.reply ? 'Edit Store Reply' : 'Reply to Customer Review'}
          </DialogTitle>
          <DialogDescription>
            Post an official store response to {review?.reviewer_name || 'the customer'}. A notification will be dispatched to the customer.
          </DialogDescription>
        </DialogHeader>

        {review && (
          <div className="rounded-lg border bg-muted/40 p-3 text-sm space-y-1">
            <div className="flex items-center justify-between font-medium">
              <span>{review.reviewer_name}</span>
              <span className="text-amber-500 font-bold">★ {review.rating}/5</span>
            </div>
            {review.title && <p className="font-medium text-foreground">{review.title}</p>}
            {review.comment ? (
              <p className="text-muted-foreground text-xs line-clamp-3">{review.comment}</p>
            ) : (
              <p className="text-muted-foreground text-xs italic">No text comment provided</p>
            )}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="reply-text">Store Response</Label>
              <span className="text-xs text-muted-foreground">{replyText.length}/2000</span>
            </div>
            <Textarea
              id="reply-text"
              rows={4}
              placeholder="Write a professional and polite response..."
              value={replyText}
              onChange={(e) => setReplyText(e.target.value)}
              maxLength={2000}
              required
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-xs text-muted-foreground flex items-center gap-1">
              <Sparkles className="h-3 w-3 text-primary" /> Quick Response Templates:
            </Label>
            <div className="flex flex-wrap gap-1.5">
              {QUICK_TEMPLATES.map((tmpl, idx) => (
                <Button
                  key={idx}
                  type="button"
                  variant="outline"
                  size="sm"
                  className="text-xs h-7"
                  onClick={() => handleApplyTemplate(tmpl.text)}
                >
                  {tmpl.label}
                </Button>
              ))}
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={replyReview.isPending}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={replyReview.isPending || !replyText.trim()}>
              {replyReview.isPending ? (
                'Posting...'
              ) : (
                <>
                  <Send className="mr-2 h-4 w-4" />
                  {review?.reply ? 'Update Reply' : 'Send Reply'}
                </>
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
