import { useState } from 'react'
import { Ban, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useShippingMutations } from '../hooks/use-shipping'

interface CancelShippingDialogProps {
  orderUuid: string
  shippingCode: string
}

export function CancelShippingDialog({
  orderUuid,
  shippingCode,
}: CancelShippingDialogProps) {
  const [open, setOpen] = useState(false)
  const [reason, setReason] = useState('')
  const { cancelShipping } = useShippingMutations()

  const handleCancel = async () => {
    try {
      await cancelShipping.mutateAsync({
        uuid: orderUuid,
        reason: reason.trim() || undefined,
      })
      toast.success('Shipping order cancelled successfully with carrier')
      setOpen(false)
      setReason('')
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to cancel shipping order'
      )
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button
          size='sm'
          variant='destructive'
          className='h-8 gap-1.5 text-xs'
        >
          <Ban className='size-3.5' /> Cancel Shipment
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className='text-destructive flex items-center gap-2'>
            <Ban className='size-4' /> Cancel Carrier Shipment
          </AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to cancel the active shipment for tracking code{' '}
            <span className='font-mono font-semibold text-foreground'>
              {shippingCode}
            </span>
            ? This action will notify the delivery carrier to cancel pickup and
            delivery.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className='space-y-2 py-2'>
          <label
            htmlFor='cancel-reason'
            className='text-xs font-semibold text-muted-foreground'
          >
            Reason for cancellation (optional)
          </label>
          <Textarea
            id='cancel-reason'
            placeholder='e.g., Customer requested delivery address update, incorrect parcel details...'
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            className='text-xs resize-none h-20'
            maxLength={500}
          />
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={cancelShipping.isPending}>
            Close
          </AlertDialogCancel>
          <Button
            variant='destructive'
            disabled={cancelShipping.isPending}
            onClick={handleCancel}
            className='h-9 text-xs'
          >
            {cancelShipping.isPending ? (
              <>
                <Loader2 className='size-3.5 animate-spin mr-1' /> Cancelling...
              </>
            ) : (
              'Confirm Cancel Shipment'
            )}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
