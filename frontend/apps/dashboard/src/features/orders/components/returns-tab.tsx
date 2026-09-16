import { useState } from 'react'
import {
  AlertCircle,
  Building2,
  CheckCircle2,
  DollarSign,
  ExternalLink,
  ImageIcon,
  PackageCheck,
  RotateCcw,
  XCircle,
} from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import {
  useOrderReturn,
  useProcessReturnRefund,
  useUpdateReturnStatus,
} from '../hooks/use-returns'
import { formatCurrency, formatDateTime } from '../order-utils'
import { RETURN_STATUS_MAP, type Order } from '../types'

interface ReturnsTabProps {
  orderUUID: string
  order?: Order
}

export function ReturnsTab({ orderUUID, order }: ReturnsTabProps) {
  const { data: returnData, isLoading, refetch } = useOrderReturn(orderUUID)
  const updateStatusMutation = useUpdateReturnStatus()
  const refundMutation = useProcessReturnRefund()

  // Modal states
  const [rejectDialogOpen, setRejectDialogOpen] = useState(false)
  const [rejectReason, setRejectReason] = useState('')

  const [refundDialogOpen, setRefundDialogOpen] = useState(false)
  const [refundAmount, setRefundAmount] = useState<string>('')
  const [bankTxnRef, setBankTxnRef] = useState('')
  const [accountingNote, setAccountingNote] = useState('')

  const [previewImage, setPreviewImage] = useState<string | null>(null)

  if (isLoading) {
    return (
      <div className='space-y-4'>
        <Skeleton className='h-32 w-full rounded-xl' />
        <Skeleton className='h-48 w-full rounded-xl' />
      </div>
    )
  }

  if (!returnData) {
    return (
      <Card>
        <CardContent className='flex flex-col items-center justify-center py-16 text-center text-muted-foreground'>
          <RotateCcw className='size-12 stroke-[1.5] text-muted-foreground/50' />
          <h3 className='mt-4 text-base font-semibold text-foreground'>
            No Return Request
          </h3>
          <p className='mt-1 max-w-sm text-xs leading-relaxed'>
            No return, exchange, or refund request has been initiated for this
            order.
          </p>
        </CardContent>
      </Card>
    )
  }

  const statusConfig = RETURN_STATUS_MAP[returnData.status] || {
    label: returnData.status_text || returnData.status_name,
    variant: 'secondary' as const,
  }

  const handleApprove = async () => {
    try {
      await updateStatusMutation.mutateAsync({
        returnUUID: returnData.return_uuid,
        input: { status: 2, admin_note: 'Return request approved by admin' },
      })
      toast.success('Return request approved successfully')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to approve return')
    }
  }

  const handleReject = async () => {
    if (!rejectReason.trim()) {
      toast.error('Please provide a rejection reason')
      return
    }
    try {
      await updateStatusMutation.mutateAsync({
        returnUUID: returnData.return_uuid,
        input: { status: 3, admin_note: rejectReason },
      })
      toast.success('Return request rejected')
      setRejectDialogOpen(false)
      setRejectReason('')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to reject return')
    }
  }

  const handleMarkCompleted = async () => {
    try {
      await updateStatusMutation.mutateAsync({
        returnUUID: returnData.return_uuid,
        input: {
          status: 4,
          admin_note: 'Return parcel received and processed by admin',
        },
      })
      toast.success('Return marked as completed')
      refetch()
    } catch (err: unknown) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to complete return'
      )
    }
  }

  const handleProcessRefund = async () => {
    try {
      const amountVal = refundAmount ? parseFloat(refundAmount) : undefined
      await refundMutation.mutateAsync({
        returnUUID: returnData.return_uuid,
        input: {
          amount: amountVal,
          bank_txn_ref: bankTxnRef || undefined,
          accounting_note: accountingNote || undefined,
        },
      })
      toast.success('Refund processed successfully')
      setRefundDialogOpen(false)
      setRefundAmount('')
      setBankTxnRef('')
      setAccountingNote('')
      refetch()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : 'Failed to process refund')
    }
  }

  const openRefundModal = () => {
    const defaultAmount =
      returnData.order?.total_amount || order?.total_amount || 0
    setRefundAmount(defaultAmount > 0 ? defaultAmount.toString() : '')
    setRefundDialogOpen(true)
  }

  return (
    <div className='space-y-6'>
      {/* Top Header Card */}
      <Card>
        <CardHeader className='flex flex-row items-start justify-between pb-3'>
          <div>
            <div className='flex items-center gap-2'>
              <CardTitle className='text-base font-semibold'>
                Return Request Details
              </CardTitle>
              <Badge variant={statusConfig.variant}>{statusConfig.label}</Badge>
            </div>
            <CardDescription className='mt-1 text-xs'>
              Return ID: <span className='font-mono'>{returnData.return_uuid}</span> •
              Created on {formatDateTime(returnData.created_at)}
            </CardDescription>
          </div>

          {/* Action Buttons */}
          <div className='flex flex-wrap items-center gap-2'>
            {returnData.status === 1 && (
              <>
                <Button
                  size='sm'
                  variant='default'
                  className='bg-emerald-600 hover:bg-emerald-700'
                  onClick={handleApprove}
                  disabled={updateStatusMutation.isPending}
                >
                  <CheckCircle2 className='mr-1.5 size-4' />
                  Approve Return
                </Button>
                <Button
                  size='sm'
                  variant='destructive'
                  onClick={() => setRejectDialogOpen(true)}
                  disabled={updateStatusMutation.isPending}
                >
                  <XCircle className='mr-1.5 size-4' />
                  Reject
                </Button>
              </>
            )}

            {returnData.status === 2 && (
              <>
                <Button
                  size='sm'
                  variant='default'
                  onClick={handleMarkCompleted}
                  disabled={updateStatusMutation.isPending}
                >
                  <PackageCheck className='mr-1.5 size-4' />
                  Mark Received & Complete
                </Button>
                <Button
                  size='sm'
                  variant='outline'
                  onClick={openRefundModal}
                  disabled={refundMutation.isPending}
                >
                  <DollarSign className='mr-1.5 size-4 text-emerald-600' />
                  Process Refund
                </Button>
              </>
            )}

            {returnData.status === 4 && (
              <Button
                size='sm'
                variant='outline'
                onClick={openRefundModal}
                disabled={refundMutation.isPending}
              >
                <DollarSign className='mr-1.5 size-4 text-emerald-600' />
                Issue Refund
              </Button>
            )}
          </div>
        </CardHeader>

        <CardContent className='grid gap-6 md:grid-cols-2'>
          {/* Reason & Customer Note */}
          <div className='space-y-3 rounded-lg border bg-muted/20 p-4 text-xs'>
            <div className='flex items-center gap-2 text-foreground font-medium'>
              <AlertCircle className='size-4 text-amber-500' />
              <span>Return Reason & Customer Statement</span>
            </div>
            <div>
              <p className='font-semibold text-foreground'>{returnData.reason}</p>
              {returnData.admin_note && (
                <div className='mt-2 rounded border border-amber-200 bg-amber-50 p-2 text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-300'>
                  <strong>Admin Note:</strong> {returnData.admin_note}
                </div>
              )}
            </div>

            {/* Proof Images */}
            {returnData.images && returnData.images.length > 0 && (
              <div className='pt-2'>
                <p className='mb-2 font-medium text-muted-foreground'>
                  Proof Images ({returnData.images.length})
                </p>
                <div className='flex flex-wrap gap-2'>
                  {returnData.images.map((imgUrl, idx) => (
                    <button
                      key={idx}
                      type='button'
                      onClick={() => setPreviewImage(imgUrl)}
                      className='group relative size-16 overflow-hidden rounded-md border bg-muted transition hover:opacity-90'
                    >
                      <img
                        src={imgUrl}
                        alt={`Proof ${idx + 1}`}
                        className='size-full object-cover'
                      />
                      <span className='absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition group-hover:opacity-100'>
                        <ImageIcon className='size-4 text-white' />
                      </span>
                    </button>
                  ))}
                </div>
              </div>
            )}

            {returnData.video_url && (
              <div className='pt-1'>
                <a
                  href={returnData.video_url}
                  target='_blank'
                  rel='noreferrer'
                  className='inline-flex items-center gap-1.5 font-medium text-primary hover:underline'
                >
                  <ExternalLink className='size-3.5' />
                  Watch Customer Proof Video
                </a>
              </div>
            )}
          </div>

          {/* Refund & Bank / Carrier Info */}
          <div className='space-y-3 rounded-lg border bg-muted/20 p-4 text-xs'>
            <div className='flex items-center gap-2 text-foreground font-medium'>
              <Building2 className='size-4 text-blue-500' />
              <span>Refund Banking & Logistics Info</span>
            </div>

            <div className='space-y-2'>
              <div className='flex justify-between border-b pb-1.5'>
                <span className='text-muted-foreground'>Bank Name:</span>
                <span className='font-semibold text-foreground'>
                  {returnData.bank_name || '—'}
                </span>
              </div>
              <div className='flex justify-between border-b pb-1.5'>
                <span className='text-muted-foreground'>Account Number:</span>
                <span className='font-mono font-semibold text-foreground'>
                  {returnData.bank_account || '—'}
                </span>
              </div>
              <div className='flex justify-between border-b pb-1.5'>
                <span className='text-muted-foreground'>Beneficiary Name:</span>
                <span className='font-semibold text-foreground'>
                  {returnData.bank_account_name || '—'}
                </span>
              </div>
              <div className='flex justify-between border-b pb-1.5'>
                <span className='text-muted-foreground'>Return Carrier:</span>
                <span className='font-semibold text-foreground'>
                  {returnData.shipping_provider || 'Not provided'}
                </span>
              </div>
              <div className='flex justify-between border-b pb-1.5'>
                <span className='text-muted-foreground'>Return Waybill Code:</span>
                <span className='font-mono font-semibold text-foreground'>
                  {returnData.shipping_code || 'Not provided'}
                </span>
              </div>
              <div className='flex justify-between pt-1'>
                <span className='text-muted-foreground'>Order Total:</span>
                <span className='font-semibold text-foreground'>
                  {formatCurrency(
                    returnData.order?.total_amount || order?.total_amount || 0
                  )}
                </span>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Reject Return Dialog */}
      <Dialog open={rejectDialogOpen} onOpenChange={setRejectDialogOpen}>
        <DialogContent className='sm:max-w-[425px]'>
          <DialogHeader>
            <DialogTitle>Reject Return Request</DialogTitle>
            <DialogDescription>
              Please state the reason why this return request is being rejected.
              This note will be communicated to the customer.
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-3 py-2'>
            <Label htmlFor='reject-reason'>Rejection Reason</Label>
            <Textarea
              id='reject-reason'
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
              placeholder='e.g., Product tag removed, item damaged by user misuse...'
              rows={3}
            />
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setRejectDialogOpen(false)}
              disabled={updateStatusMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              variant='destructive'
              onClick={handleReject}
              disabled={updateStatusMutation.isPending}
            >
              {updateStatusMutation.isPending ? 'Rejecting...' : 'Reject Request'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Refund Dialog */}
      <Dialog open={refundDialogOpen} onOpenChange={setRefundDialogOpen}>
        <DialogContent className='sm:max-w-[450px]'>
          <DialogHeader>
            <DialogTitle>Process Return Refund</DialogTitle>
            <DialogDescription>
              Confirm and record the refund transaction for this return request.
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-3 py-2 text-xs'>
            <div className='space-y-1'>
              <Label htmlFor='refund-amount'>Refund Amount (VND)</Label>
              <Input
                id='refund-amount'
                type='number'
                value={refundAmount}
                onChange={(e) => setRefundAmount(e.target.value)}
                placeholder='Leave empty for full order amount'
              />
            </div>
            <div className='space-y-1'>
              <Label htmlFor='bank-txn-ref'>
                Gateway / Bank Transaction Reference (Optional)
              </Label>
              <Input
                id='bank-txn-ref'
                value={bankTxnRef}
                onChange={(e) => setBankTxnRef(e.target.value)}
                placeholder='e.g. FT2409158888 or VNPay Transaction ID'
              />
            </div>
            <div className='space-y-1'>
              <Label htmlFor='accounting-note'>Accounting Note (Optional)</Label>
              <Textarea
                id='accounting-note'
                value={accountingNote}
                onChange={(e) => setAccountingNote(e.target.value)}
                placeholder='Internal note regarding refund authorization'
                rows={2}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setRefundDialogOpen(false)}
              disabled={refundMutation.isPending}
            >
              Cancel
            </Button>
            <Button
              className='bg-emerald-600 hover:bg-emerald-700'
              onClick={handleProcessRefund}
              disabled={refundMutation.isPending}
            >
              {refundMutation.isPending ? 'Processing...' : 'Confirm Refund'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Image Preview Dialog */}
      <Dialog
        open={Boolean(previewImage)}
        onOpenChange={(open) => !open && setPreviewImage(null)}
      >
        <DialogContent className='max-w-2xl p-2'>
          {previewImage && (
            <img
              src={previewImage}
              alt='Proof Full Preview'
              className='max-h-[80vh] w-full rounded-md object-contain'
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
