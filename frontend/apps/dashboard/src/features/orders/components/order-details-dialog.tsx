import { useState, useEffect } from 'react'
import {
  Package,
  User,
  Phone,
  MapPin,
  CreditCard,
  Receipt,
  FileText,
  Clock,
  Loader2,
} from 'lucide-react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useOrderMutations, useOrderDetails } from '../hooks/use-orders'
import { getImageUrl } from '@/lib/utils'
import type { Order } from '../types'

interface OrderDetailsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  order: Order | null
}

export function OrderDetailsDialog({
  open,
  onOpenChange,
  order,
}: OrderDetailsDialogProps) {
  const { updateOrderStatus, updateOrderPaymentStatus } = useOrderMutations()
  const { data: detailsData, isLoading: isDetailsLoading } = useOrderDetails(
    open && order?.uuid ? order.uuid : null
  )

  const currentOrder: Order | null = (detailsData as unknown as Order) || order

  const [selectedStatus, setSelectedStatus] = useState<string>('')
  const [selectedPaymentStatus, setSelectedPaymentStatus] = useState<string>('')

  useEffect(() => {
    if (currentOrder) {
      const s = currentOrder.status?.toLowerCase()
      if (s === 'pending') setSelectedStatus('1')
      else if (s === 'confirmed') setSelectedStatus('2')
      else if (s === 'shipped') setSelectedStatus('3')
      else if (s === 'delivered') setSelectedStatus('4')
      else if (s === 'cancelled') setSelectedStatus('5')
      else if (s === 'returned') setSelectedStatus('6')

      const ps = currentOrder.payment_status?.toLowerCase()
      if (ps === 'unpaid') setSelectedPaymentStatus('1')
      else if (ps === 'paid') setSelectedPaymentStatus('2')
      else if (ps === 'partially refunded') setSelectedPaymentStatus('3')
      else if (ps === 'refunded') setSelectedPaymentStatus('4')
    }
  }, [currentOrder?.status, currentOrder?.payment_status])

  if (!order) return null

  const formatCurrency = (val?: number) => {
    return new Intl.NumberFormat('vi-VN', {
      style: 'currency',
      currency: 'VND',
    }).format(val ?? 0)
  }

  const handleUpdateStatus = async () => {
    if (!order.uuid || !selectedStatus) return
    try {
      await updateOrderStatus.mutateAsync({
        uuid: order.uuid,
        status: Number(selectedStatus) as 1 | 2 | 3 | 4 | 5 | 6,
      })
      toast.success('Order status updated successfully')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update status')
    }
  }

  const handleUpdatePaymentStatus = async () => {
    if (!order.uuid || !selectedPaymentStatus) return
    const psNum = Number(selectedPaymentStatus)
    if (psNum !== 2 && psNum !== 3 && psNum !== 4) return
    try {
      await updateOrderPaymentStatus.mutateAsync({
        uuid: order.uuid,
        payment_status: psNum as 2 | 3 | 4,
      })
      toast.success('Payment status updated successfully')
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to update payment status'
      )
    }
  }

  const renderOrderStatusBadge = (status?: string) => {
    const s = status?.toLowerCase()
    if (s === 'delivered') {
      return (
        <Badge
          variant='outline'
          className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
        >
          Delivered
        </Badge>
      )
    }
    if (s === 'shipped') {
      return (
        <Badge
          variant='outline'
          className='bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/20'
        >
          Shipped
        </Badge>
      )
    }
    if (s === 'confirmed') {
      return (
        <Badge
          variant='outline'
          className='bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
        >
          Confirmed
        </Badge>
      )
    }
    if (s === 'pending') {
      return (
        <Badge
          variant='outline'
          className='bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
        >
          Pending
        </Badge>
      )
    }
    if (s === 'cancelled') {
      return <Badge variant='destructive'>Cancelled</Badge>
    }
    return <Badge variant='secondary'>{status || 'Unknown'}</Badge>
  }

  const renderPaymentStatusBadge = (status?: string) => {
    const ps = status?.toLowerCase()
    if (ps === 'paid') {
      return (
        <Badge
          variant='outline'
          className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
        >
          Paid
        </Badge>
      )
    }
    if (ps === 'unpaid') {
      return (
        <Badge
          variant='outline'
          className='bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
        >
          Unpaid
        </Badge>
      )
    }
    if (ps === 'refunded' || ps === 'partially refunded') {
      return (
        <Badge
          variant='outline'
          className='bg-destructive/10 text-destructive border-destructive/20'
        >
          {status}
        </Badge>
      )
    }
    return <Badge variant='secondary'>{status || 'Unknown'}</Badge>
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <div className='flex flex-wrap items-center justify-between gap-2 pr-6'>
            <DialogTitle className='text-xl font-bold font-mono'>
              #{currentOrder.order_number}
            </DialogTitle>
            <div className='flex items-center gap-2'>
              {renderOrderStatusBadge(currentOrder.status)}
              {renderPaymentStatusBadge(currentOrder.payment_status)}
            </div>
          </div>
          <DialogDescription className='flex items-center gap-1.5 text-xs text-muted-foreground pt-1'>
            <Clock className='size-3.5' /> Created on {currentOrder.created_at || '—'}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-6 pt-2 text-sm'>
          {/* Status Quick Updater */}
          <div className='grid grid-cols-1 sm:grid-cols-2 gap-3 p-3 bg-muted/40 rounded-lg border'>
            <div className='space-y-1.5'>
              <label className='text-xs font-semibold text-muted-foreground'>
                Update Order Status
              </label>
              <div className='flex items-center gap-2'>
                <Select value={selectedStatus} onValueChange={setSelectedStatus}>
                  <SelectTrigger className='h-8 text-xs'>
                    <SelectValue placeholder='Select status' />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='1'>Pending</SelectItem>
                    <SelectItem value='2'>Confirmed</SelectItem>
                    <SelectItem value='3'>Shipped</SelectItem>
                    <SelectItem value='4'>Delivered</SelectItem>
                    <SelectItem value='5'>Cancelled</SelectItem>
                    <SelectItem value='6'>Returned</SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  size='sm'
                  variant='outline'
                  className='h-8 text-xs shrink-0'
                  onClick={handleUpdateStatus}
                  disabled={updateOrderStatus.isPending}
                >
                  Save
                </Button>
              </div>
            </div>

            <div className='space-y-1.5'>
              <label className='text-xs font-semibold text-muted-foreground'>
                Update Payment Status
              </label>
              <div className='flex items-center gap-2'>
                <Select
                  value={selectedPaymentStatus}
                  onValueChange={setSelectedPaymentStatus}
                >
                  <SelectTrigger className='h-8 text-xs'>
                    <SelectValue placeholder='Select payment status' />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='2'>Paid</SelectItem>
                    <SelectItem value='3'>Partially Refunded</SelectItem>
                    <SelectItem value='4'>Refunded</SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  size='sm'
                  variant='outline'
                  className='h-8 text-xs shrink-0'
                  onClick={handleUpdatePaymentStatus}
                  disabled={updateOrderPaymentStatus.isPending}
                >
                  Save
                </Button>
              </div>
            </div>
          </div>

          {/* Customer & Shipping Info */}
          <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
            <div className='rounded-lg border p-3.5 space-y-2 bg-card'>
              <div className='flex items-center gap-2 font-semibold text-foreground text-xs uppercase tracking-wider text-muted-foreground'>
                <User className='size-3.5' /> Customer
              </div>
              <div className='font-medium'>{currentOrder.shipping_name}</div>
              <div className='flex items-center gap-1.5 text-xs text-muted-foreground'>
                <Phone className='size-3.5' /> {currentOrder.shipping_phone}
              </div>
              {currentOrder.user_name && (
                <div className='text-xs text-muted-foreground'>
                  Account: <span className='font-medium'>{currentOrder.user_name}</span>
                </div>
              )}
            </div>

            <div className='rounded-lg border p-3.5 space-y-2 bg-card'>
              <div className='flex items-center gap-2 font-semibold text-foreground text-xs uppercase tracking-wider text-muted-foreground'>
                <MapPin className='size-3.5' /> Shipping Address
              </div>
              <div className='text-xs text-foreground leading-relaxed break-words'>
                {currentOrder.shipping_address || (
                  <span className='text-muted-foreground italic'>No shipping address provided</span>
                )}
              </div>
              {currentOrder.note && (
                <div className='pt-1 text-xs text-muted-foreground italic flex items-start gap-1'>
                  <FileText className='size-3.5 shrink-0 mt-0.5' /> "{currentOrder.note}"
                </div>
              )}
            </div>
          </div>

          {/* Order Items */}
          <div className='space-y-2.5'>
            <div className='flex items-center justify-between'>
              <div className='flex items-center gap-2 font-semibold text-foreground text-xs uppercase tracking-wider text-muted-foreground'>
                <Package className='size-3.5' /> Order Items ({currentOrder.items?.length || 0})
              </div>
              {isDetailsLoading && (
                <div className='flex items-center gap-1.5 text-xs text-muted-foreground'>
                  <Loader2 className='size-3.5 animate-spin' /> Loading details...
                </div>
              )}
            </div>
            <div className='divide-y rounded-lg border bg-card overflow-hidden'>
              {isDetailsLoading && (!currentOrder.items || currentOrder.items.length === 0) ? (
                <div className='p-6 flex items-center justify-center gap-2 text-xs text-muted-foreground'>
                  <Loader2 className='size-4 animate-spin' /> Loading order items...
                </div>
              ) : currentOrder.items && currentOrder.items.length > 0 ? (
                currentOrder.items.map((item, idx) => (
                  <div
                    key={item.uuid || idx}
                    className='flex items-center justify-between p-3 gap-3'
                  >
                    <div className='flex items-center gap-3 min-w-0'>
                      {item.image_url ? (
                        <img
                          src={getImageUrl(item.image_url)}
                          alt={item.product_name || 'Product'}
                          className='size-12 rounded object-cover border bg-muted shrink-0'
                        />
                      ) : (
                        <div className='size-12 rounded bg-muted border flex items-center justify-center text-muted-foreground shrink-0'>
                          <Package className='size-5' />
                        </div>
                      )}
                      <div className='min-w-0'>
                        <div className='font-medium text-xs sm:text-sm truncate'>
                          {item.product_name}
                        </div>
                        <div className='text-xs text-muted-foreground'>
                          {formatCurrency(item.price)} × {item.quantity}
                        </div>
                      </div>
                    </div>
                    <div className='font-semibold text-xs sm:text-sm text-right shrink-0'>
                      {formatCurrency(item.subtotal)}
                    </div>
                  </div>
                ))
              ) : (
                <div className='p-4 text-center text-xs text-muted-foreground'>
                  No items in this order.
                </div>
              )}
            </div>
          </div>

          {/* Financial Summary */}
          <div className='rounded-lg border bg-card p-4 space-y-2 text-xs'>
            <div className='flex items-center gap-2 font-semibold uppercase tracking-wider text-muted-foreground pb-1'>
              <Receipt className='size-3.5' /> Payment Summary
            </div>
            <div className='flex justify-between text-muted-foreground'>
              <span>Payment Method</span>
              <span className='font-medium text-foreground flex items-center gap-1'>
                <CreditCard className='size-3' /> {currentOrder.payment_method}
              </span>
            </div>
            <div className='flex justify-between text-muted-foreground'>
              <span>Subtotal</span>
              <span className='font-medium text-foreground'>
                {formatCurrency(currentOrder.subtotal_amount)}
              </span>
            </div>
            {Boolean(currentOrder.discount_amount && currentOrder.discount_amount > 0) && (
              <div className='flex justify-between text-emerald-600 dark:text-emerald-400'>
                <span>
                  Discount {currentOrder.coupon_code ? `(${currentOrder.coupon_code})` : ''}
                </span>
                <span>-{formatCurrency(currentOrder.discount_amount)}</span>
              </div>
            )}
            <div className='flex justify-between text-muted-foreground'>
              <span>Shipping Fee</span>
              <span className='font-medium text-foreground'>
                {formatCurrency(currentOrder.shipping_fee)}
              </span>
            </div>
            <Separator className='my-2' />
            <div className='flex justify-between text-sm font-bold text-foreground'>
              <span>Total Amount</span>
              <span className='text-base text-primary'>
                {formatCurrency(currentOrder.total_amount)}
              </span>
            </div>
            {Boolean(currentOrder.refunded_amount && currentOrder.refunded_amount > 0) && (
              <div className='flex justify-between text-destructive text-xs pt-1'>
                <span>Refunded Amount</span>
                <span>-{formatCurrency(currentOrder.refunded_amount)}</span>
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
