import { useState } from 'react'
import {
  CheckCircle2,
  Ban,
  ChevronDown,
  X,
  Loader2,
  PackageCheck,
  Truck,
  RotateCcw,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Textarea } from '@/components/ui/textarea'

interface BulkActionBarProps {
  selectedCount: number
  isProcessing: boolean
  onBulkConfirm: () => void
  onBulkChangeStatus: (status: 1 | 2 | 3 | 4 | 5 | 6) => void
  onBulkCancel: (reason?: string) => void
  onClearSelection: () => void
}

export function BulkActionBar({
  selectedCount,
  isProcessing,
  onBulkConfirm,
  onBulkChangeStatus,
  onBulkCancel,
  onClearSelection,
}: BulkActionBarProps) {
  const [cancelDialogOpen, setCancelDialogOpen] = useState(false)
  const [cancelReason, setCancelReason] = useState('')

  if (selectedCount === 0) return null

  const handleConfirmCancel = () => {
    onBulkCancel(cancelReason.trim() || undefined)
    setCancelReason('')
    setCancelDialogOpen(false)
  }

  return (
    <>
      <div className='fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-2 sm:gap-3 px-4 py-2.5 rounded-xl border bg-background/95 backdrop-blur-md shadow-2xl animate-in fade-in slide-in-from-bottom-5 duration-200'>
        <div className='flex items-center gap-2 pr-2 border-r'>
          <Badge variant='secondary' className='px-2 py-0.5 font-semibold text-xs'>
            {selectedCount}
          </Badge>
          <span className='text-xs font-medium text-muted-foreground hidden sm:inline'>
            selected
          </span>
        </div>

        {/* Quick Confirm */}
        <Button
          size='sm'
          variant='default'
          className='gap-1.5 h-8 text-xs bg-emerald-600 hover:bg-emerald-700 text-white'
          disabled={isProcessing}
          onClick={onBulkConfirm}
        >
          {isProcessing ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <CheckCircle2 className='size-3.5' />
          )}
          <span>Confirm</span>
        </Button>

        {/* Change Status Dropdown */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              size='sm'
              variant='outline'
              className='gap-1.5 h-8 text-xs'
              disabled={isProcessing}
            >
              <span>Change Status</span>
              <ChevronDown className='size-3.5' />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align='center' className='w-48'>
            <DropdownMenuItem
              className='gap-2 text-xs cursor-pointer'
              onClick={() => onBulkChangeStatus(2)}
            >
              <PackageCheck className='size-3.5 text-blue-500' />
              <span>Mark as Confirmed</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              className='gap-2 text-xs cursor-pointer'
              onClick={() => onBulkChangeStatus(3)}
            >
              <Truck className='size-3.5 text-amber-500' />
              <span>Mark as Shipped</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              className='gap-2 text-xs cursor-pointer'
              onClick={() => onBulkChangeStatus(4)}
            >
              <CheckCircle2 className='size-3.5 text-emerald-500' />
              <span>Mark as Delivered</span>
            </DropdownMenuItem>
            <DropdownMenuItem
              className='gap-2 text-xs cursor-pointer'
              onClick={() => onBulkChangeStatus(6)}
            >
              <RotateCcw className='size-3.5 text-rose-500' />
              <span>Mark as Returned</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Bulk Cancel Button */}
        <Button
          size='sm'
          variant='destructive'
          className='gap-1.5 h-8 text-xs'
          disabled={isProcessing}
          onClick={() => setCancelDialogOpen(true)}
        >
          <Ban className='size-3.5' />
          <span>Cancel</span>
        </Button>

        {/* Deselect All */}
        <Button
          size='sm'
          variant='ghost'
          className='size-8 p-0 text-muted-foreground hover:text-foreground'
          disabled={isProcessing}
          onClick={onClearSelection}
          title='Clear selection'
        >
          <X className='size-4' />
        </Button>
      </div>

      {/* Bulk Cancel Confirmation Dialog */}
      <AlertDialog open={cancelDialogOpen} onOpenChange={setCancelDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Cancel {selectedCount} Selected Orders?</AlertDialogTitle>
            <AlertDialogDescription>
              This will cancel the selected orders and restore their item stock back to inventory.
              Orders that have already shipped or completed cannot be cancelled.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className='py-2'>
            <label className='text-xs font-medium text-muted-foreground mb-1 block'>
              Cancellation Reason (optional):
            </label>
            <Textarea
              placeholder='e.g., Customer requested bulk cancellation, out of stock...'
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
              rows={2}
              className='text-xs'
            />
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isProcessing}>Back</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              disabled={isProcessing}
              onClick={handleConfirmCancel}
            >
              {isProcessing ? (
                <Loader2 className='size-3.5 animate-spin mr-1' />
              ) : null}
              Confirm Cancellation
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
