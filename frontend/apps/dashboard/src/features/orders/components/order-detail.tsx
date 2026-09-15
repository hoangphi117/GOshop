import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  ArrowLeft,
  Package,
  User,
  Phone,
  MapPin,
  CreditCard,
  Receipt,
  FileText,
  Clock,
  Loader2,
  Truck,
  RotateCcw,
} from 'lucide-react'
import { toast } from 'sonner'
import { getImageUrl } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { useOrderDetails, useOrderMutations } from '../hooks/use-orders'
import { ShippingTab } from './shipping-tab'
import type { Order } from '../types'
import {
  formatCurrency,
  formatDateTime,
  renderOrderStatusBadge,
  renderPaymentStatusBadge,
} from '../order-utils'

export function OrderDetail({ uuid }: { uuid: string }) {
  const navigate = useNavigate()
  const { data, isLoading } = useOrderDetails(uuid)
  const { updateOrderStatus, updateOrderPaymentStatus } = useOrderMutations()

  // Cast the API response to a usable shape
  const order = data as Record<string, unknown> | null

  const [selectedStatus, setSelectedStatus] = useState<string>('')
  const [selectedPaymentStatus, setSelectedPaymentStatus] = useState<string>('')

  // Sync selects when order loads
  const currentStatus = (order?.status as string)?.toLowerCase()
  const currentPaymentStatus = (order?.payment_status as string)?.toLowerCase()

  // Initialize select values from order data on first render
  if (order && !selectedStatus) {
    const statusMap: Record<string, string> = {
      pending: '1',
      confirmed: '2',
      shipped: '3',
      delivered: '4',
      cancelled: '5',
      returned: '6',
    }
    if (statusMap[currentStatus]) setSelectedStatus(statusMap[currentStatus])
  }
  if (order && !selectedPaymentStatus) {
    const paymentMap: Record<string, string> = {
      unpaid: '1',
      paid: '2',
      'partially refunded': '3',
      refunded: '4',
    }
    if (paymentMap[currentPaymentStatus])
      setSelectedPaymentStatus(paymentMap[currentPaymentStatus])
  }

  const handleUpdateStatus = async () => {
    if (!selectedStatus) return
    try {
      await updateOrderStatus.mutateAsync({
        uuid,
        status: Number(selectedStatus) as 1 | 2 | 3 | 4 | 5 | 6,
      })
      toast.success('Order status updated successfully')
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to update status'
      )
    }
  }

  const handleUpdatePaymentStatus = async () => {
    if (!selectedPaymentStatus) return
    const psNum = Number(selectedPaymentStatus)
    if (psNum !== 2 && psNum !== 3 && psNum !== 4) return
    try {
      await updateOrderPaymentStatus.mutateAsync({
        uuid,
        payment_status: psNum as 2 | 3 | 4,
      })
      toast.success('Payment status updated successfully')
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : 'Failed to update payment status'
      )
    }
  }

  const items = (order?.items as Array<Record<string, unknown>>) || []

  if (isLoading) {
    return (
      <>
        <Header fixed>
          <Search className='me-auto' />
          <ThemeSwitch />
          <ProfileDropdown />
        </Header>
        <Main className='flex flex-1 flex-col gap-6'>
          <div className='flex items-center gap-3'>
            <Skeleton className='h-9 w-9' />
            <Skeleton className='h-8 w-48' />
          </div>
          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
            <Skeleton className='h-32' />
            <Skeleton className='h-32' />
          </div>
          <Skeleton className='h-64' />
        </Main>
      </>
    )
  }

  if (!order) {
    return (
      <>
        <Header fixed>
          <Search className='me-auto' />
          <ThemeSwitch />
          <ProfileDropdown />
        </Header>
        <Main className='flex flex-1 flex-col items-center justify-center gap-4'>
          <Package className='size-12 text-muted-foreground' />
          <p className='text-muted-foreground'>Order not found</p>
          <Button variant='outline' onClick={() => navigate({ to: '/orders' })}>
            Back to Orders
          </Button>
        </Main>
      </>
    )
  }

  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        {/* Page Header */}
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <div className='flex items-center gap-3'>
            <Button
              variant='ghost'
              size='icon'
              className='size-9'
              onClick={() => navigate({ to: '/orders' })}
              title='Back to orders'
            >
              <ArrowLeft className='size-4' />
            </Button>
            <div>
              <h2 className='font-mono text-2xl font-bold tracking-tight'>
                #{order.order_number as string}
              </h2>
              <p className='flex items-center gap-1.5 text-xs text-muted-foreground'>
                <Clock className='size-3.5' />
                Created on {formatDateTime(order.created_at as string)}
              </p>
            </div>
          </div>
          <div className='flex items-center gap-2'>
            {renderOrderStatusBadge(order.status as string)}
            {renderPaymentStatusBadge(order.payment_status as string)}
          </div>
        </div>

        {/* Tabs */}
        <Tabs defaultValue='overview'>
          <TabsList>
            <TabsTrigger value='overview'>
              <Package className='size-4' />
              Overview
            </TabsTrigger>
            <TabsTrigger value='shipping'>
              <Truck className='size-4' />
              Shipping
            </TabsTrigger>
            <TabsTrigger value='returns'>
              <RotateCcw className='size-4' />
              Returns
            </TabsTrigger>
          </TabsList>

          {/* Overview Tab */}
          <TabsContent value='overview' className='space-y-6'>
            {/* Status Quick Updater */}
            <div className='grid grid-cols-1 gap-3 rounded-lg border bg-muted/40 p-3 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <label className='text-xs font-semibold text-muted-foreground'>
                  Update Order Status
                </label>
                <div className='flex items-center gap-2'>
                  <Select
                    value={selectedStatus}
                    onValueChange={setSelectedStatus}
                  >
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
                    className='h-8 shrink-0 text-xs'
                    onClick={handleUpdateStatus}
                    disabled={updateOrderStatus.isPending}
                  >
                    {updateOrderStatus.isPending ? (
                      <Loader2 className='size-3 animate-spin' />
                    ) : (
                      'Save'
                    )}
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
                    className='h-8 shrink-0 text-xs'
                    onClick={handleUpdatePaymentStatus}
                    disabled={updateOrderPaymentStatus.isPending}
                  >
                    {updateOrderPaymentStatus.isPending ? (
                      <Loader2 className='size-3 animate-spin' />
                    ) : (
                      'Save'
                    )}
                  </Button>
                </div>
              </div>
            </div>

            {/* Customer & Shipping Info */}
            <div className='grid grid-cols-1 gap-4 sm:grid-cols-2'>
              <div className='space-y-2 rounded-lg border bg-card p-3.5'>
                <div className='flex items-center gap-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase'>
                  <User className='size-3.5' /> Customer
                </div>
                <div className='font-medium'>
                  {order.shipping_name as string}
                </div>
                <div className='flex items-center gap-1.5 text-xs text-muted-foreground'>
                  <Phone className='size-3.5' />{' '}
                  {order.shipping_phone as string}
                </div>
                {Boolean(order.user_name) && (
                  <div className='text-xs text-muted-foreground'>
                    Account:{' '}
                    <span className='font-medium'>
                      {order.user_name as string}
                    </span>
                  </div>
                )}
              </div>

              <div className='space-y-2 rounded-lg border bg-card p-3.5'>
                <div className='flex items-center gap-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase'>
                  <MapPin className='size-3.5' /> Shipping Address
                </div>
                <div className='text-xs leading-relaxed break-words text-foreground'>
                  {(order.shipping_address as string) || (
                    <span className='text-muted-foreground italic'>
                      No shipping address provided
                    </span>
                  )}
                </div>
                {Boolean(order.note) && (
                  <div className='flex items-start gap-1 pt-1 text-xs text-muted-foreground italic'>
                    <FileText className='mt-0.5 size-3.5 shrink-0' /> "
                    {order.note as string}"
                  </div>
                )}
              </div>
            </div>

            {/* Order Items */}
            <div className='space-y-2.5'>
              <div className='flex items-center gap-2 text-xs font-semibold tracking-wider text-muted-foreground uppercase'>
                <Package className='size-3.5' /> Order Items ({items.length})
              </div>
              <div className='divide-y overflow-hidden rounded-lg border bg-card'>
                {items.length > 0 ? (
                  items.map((item, idx) => (
                    <div
                      key={(item.uuid as string) || idx}
                      className='flex items-center justify-between gap-3 p-3'
                    >
                      <div className='flex min-w-0 items-center gap-3'>
                        {item.image_url ? (
                          <img
                            src={getImageUrl(item.image_url as string)}
                            alt={(item.product_name as string) || 'Product'}
                            className='size-12 shrink-0 rounded border bg-muted object-cover'
                          />
                        ) : (
                          <div className='flex size-12 shrink-0 items-center justify-center rounded border bg-muted text-muted-foreground'>
                            <Package className='size-5' />
                          </div>
                        )}
                        <div className='min-w-0'>
                          <div className='truncate text-xs font-medium sm:text-sm'>
                            {item.product_name as string}
                          </div>
                          <div className='text-xs text-muted-foreground'>
                            {formatCurrency(item.price as number)} ×{' '}
                            {item.quantity as number}
                          </div>
                        </div>
                      </div>
                      <div className='shrink-0 text-right text-xs font-semibold sm:text-sm'>
                        {formatCurrency(item.subtotal as number)}
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
            <div className='space-y-2 rounded-lg border bg-card p-4 text-xs'>
              <div className='flex items-center gap-2 pb-1 font-semibold tracking-wider text-muted-foreground uppercase'>
                <Receipt className='size-3.5' /> Payment Summary
              </div>
              <div className='flex justify-between text-muted-foreground'>
                <span>Payment Method</span>
                <span className='flex items-center gap-1 font-medium text-foreground'>
                  <CreditCard className='size-3' />{' '}
                  {order.payment_method as string}
                </span>
              </div>
              <div className='flex justify-between text-muted-foreground'>
                <span>Subtotal</span>
                <span className='font-medium text-foreground'>
                  {formatCurrency(order.subtotal_amount as number)}
                </span>
              </div>
              {Boolean(
                order.discount_amount && (order.discount_amount as number) > 0
              ) && (
                <div className='flex justify-between text-emerald-600 dark:text-emerald-400'>
                  <span>
                    Discount {order.coupon_code ? `(${order.coupon_code})` : ''}
                  </span>
                  <span>
                    -{formatCurrency(order.discount_amount as number)}
                  </span>
                </div>
              )}
              <div className='flex justify-between text-muted-foreground'>
                <span>Shipping Fee</span>
                <span className='font-medium text-foreground'>
                  {formatCurrency(order.shipping_fee as number)}
                </span>
              </div>
              <Separator className='my-2' />
              <div className='flex justify-between text-sm font-bold text-foreground'>
                <span>Total Amount</span>
                <span className='text-base text-primary'>
                  {formatCurrency(order.total_amount as number)}
                </span>
              </div>
              {Boolean(
                order.refunded_amount && (order.refunded_amount as number) > 0
              ) && (
                <div className='flex justify-between pt-1 text-xs text-destructive'>
                  <span>Refunded Amount</span>
                  <span>
                    -{formatCurrency(order.refunded_amount as number)}
                  </span>
                </div>
              )}
            </div>
          </TabsContent>

          {/* Shipping Tab — Phase 2 GHN & Carrier Integration */}
          <TabsContent value='shipping' className='space-y-4'>
            {order && <ShippingTab order={order as unknown as Order} />}
          </TabsContent>

          {/* Returns Tab — placeholder for Phase 4 */}
          <TabsContent value='returns'>
            <div className='flex flex-col items-center justify-center gap-3 py-16 text-muted-foreground'>
              <RotateCcw className='size-10' />
              <p className='text-sm font-medium'>
                Return management coming soon
              </p>
              <p className='text-xs'>
                Return requests, refund processing, and RMA tracking will be
                available here.
              </p>
            </div>
          </TabsContent>
        </Tabs>
      </Main>
    </>
  )
}
