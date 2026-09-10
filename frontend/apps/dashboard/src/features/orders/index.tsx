import { useState } from 'react'
import {
  Search as SearchIcon,
  Eye,
  ShoppingBag,
  Filter,
  CreditCard,
} from 'lucide-react'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DataTablePagination } from '@/components/data-table'
import { useOrders } from './hooks/use-orders'
import { OrderDetailsDialog } from './components/order-details-dialog'
import type { Order } from './types'

export function Orders() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [searchTerm, setSearchTerm] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null)
  const [detailsOpen, setDetailsOpen] = useState(false)

  const parsedStatus =
    statusFilter !== 'all' ? (Number(statusFilter) as 1 | 2 | 3 | 4 | 5 | 6) : undefined

  const { data: ordersData, isLoading } = useOrders({
    page,
    limit: pageSize,
    search: activeSearch || undefined,
    status: parsedStatus,
  })

  const orders = (ordersData?.data as unknown as Order[]) || []
  const pagination = ordersData?.pagination

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setActiveSearch(searchTerm.trim())
    setPage(1)
  }

  const handleStatusFilterChange = (val: string) => {
    setStatusFilter(val)
    setPage(1)
  }

  const handleResetFilters = () => {
    setSearchTerm('')
    setActiveSearch('')
    setStatusFilter('all')
    setPage(1)
  }

  const handleOpenDetails = (order: Order) => {
    setSelectedOrder(order)
    setDetailsOpen(true)
  }

  const formatCurrency = (val?: number) => {
    return new Intl.NumberFormat('vi-VN', {
      style: 'currency',
      currency: 'VND',
    }).format(val ?? 0)
  }

  const formatDateTimeVN = (dateStr?: string) => {
    if (!dateStr) return '—'
    try {
      if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(dateStr)) {
        return dateStr
      }
      const d = new Date(dateStr)
      if (isNaN(d.getTime())) return dateStr
      return new Intl.DateTimeFormat('vi-VN', {
        timeZone: 'Asia/Ho_Chi_Minh',
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      }).format(d)
    } catch {
      return dateStr
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

  const renderPaymentBadge = (method?: string, paymentStatus?: string) => {
    const ps = paymentStatus?.toLowerCase()
    const isPaid = ps === 'paid'
    const isUnpaid = ps === 'unpaid'

    return (
      <div className='flex flex-col gap-1 items-start'>
        <span className='text-xs text-muted-foreground flex items-center gap-1 font-medium'>
          <CreditCard className='size-3' /> {method || 'COD'}
        </span>
        {isPaid ? (
          <Badge
            variant='outline'
            className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 text-[10px] px-1.5 py-0'
          >
            Paid
          </Badge>
        ) : isUnpaid ? (
          <Badge
            variant='outline'
            className='bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20 text-[10px] px-1.5 py-0'
          >
            Unpaid
          </Badge>
        ) : (
          <Badge variant='outline' className='text-[10px] px-1.5 py-0'>
            {paymentStatus || 'Pending'}
          </Badge>
        )}
      </div>
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
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Orders</h2>
            <p className='text-muted-foreground'>
              Track and manage customer purchases, delivery statuses, and payment records.
            </p>
          </div>
        </div>

        {/* Filters Bar */}
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <form
            onSubmit={handleSearchSubmit}
            className='flex items-center gap-2 max-w-sm flex-1'
          >
            <div className='relative flex-1'>
              <SearchIcon className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
              <Input
                placeholder='Search order #, recipient, or phone...'
                className='pl-9'
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
            </div>
            <Button type='submit' variant='secondary' size='sm'>
              Search
            </Button>
          </form>

          <div className='flex items-center gap-2'>
            <div className='flex items-center gap-1.5'>
              <Filter className='size-4 text-muted-foreground' />
              <Select
                value={statusFilter}
                onValueChange={handleStatusFilterChange}
              >
                <SelectTrigger className='w-[150px] h-9 text-xs'>
                  <SelectValue placeholder='All Statuses' />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>All Statuses</SelectItem>
                  <SelectItem value='1'>Pending</SelectItem>
                  <SelectItem value='2'>Confirmed</SelectItem>
                  <SelectItem value='3'>Shipped</SelectItem>
                  <SelectItem value='4'>Delivered</SelectItem>
                  <SelectItem value='5'>Cancelled</SelectItem>
                  <SelectItem value='6'>Returned</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {(activeSearch || statusFilter !== 'all') && (
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={handleResetFilters}
              >
                Reset
              </Button>
            )}
          </div>
        </div>

        {/* Orders Table */}
        <div className='rounded-md border bg-card'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Order #</TableHead>
                <TableHead>Recipient</TableHead>
                <TableHead>Total Amount</TableHead>
                <TableHead>Items</TableHead>
                <TableHead>Payment</TableHead>
                <TableHead>Order Status</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    Loading orders...
                  </TableCell>
                </TableRow>
              ) : orders.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    No orders found.
                  </TableCell>
                </TableRow>
              ) : (
                orders.map((order) => (
                  <TableRow
                    key={order.uuid}
                    className='cursor-pointer hover:bg-muted/50 transition-colors'
                    onClick={() => handleOpenDetails(order)}
                  >
                    <TableCell className='font-mono font-medium text-xs'>
                      #{order.order_number}
                    </TableCell>
                    <TableCell>
                      <div className='font-medium text-xs sm:text-sm'>
                        {order.shipping_name}
                      </div>
                      <div className='text-xs text-muted-foreground'>
                        {order.shipping_phone}
                      </div>
                      {order.shipping_address && (
                        <div
                          className='text-[11px] text-muted-foreground truncate max-w-[200px]'
                          title={order.shipping_address}
                        >
                          {order.shipping_address}
                        </div>
                      )}
                    </TableCell>
                    <TableCell className='font-semibold text-xs sm:text-sm'>
                      {formatCurrency(order.total_amount)}
                    </TableCell>
                    <TableCell className='text-xs text-muted-foreground'>
                      <span className='inline-flex items-center gap-1'>
                        <ShoppingBag className='size-3.5' /> {order.items?.length || 1} items
                      </span>
                    </TableCell>
                    <TableCell>
                      {renderPaymentBadge(order.payment_method, order.payment_status)}
                    </TableCell>
                    <TableCell>{renderOrderStatusBadge(order.status)}</TableCell>
                    <TableCell className='text-muted-foreground text-xs'>
                      {formatDateTimeVN(order.created_at)}
                    </TableCell>
                    <TableCell className='text-right'>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='size-8'
                        onClick={(e) => {
                          e.stopPropagation()
                          handleOpenDetails(order)
                        }}
                        title='View details'
                      >
                        <Eye className='size-4' />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        {/* Pagination */}
        <DataTablePagination
          page={page}
          pageSize={pageSize}
          totalPages={pagination?.total_pages || 1}
          onPageChange={setPage}
          onPageSizeChange={(newSize) => {
            setPageSize(newSize)
            setPage(1)
          }}
          className='mt-auto'
        />
      </Main>

      <OrderDetailsDialog
        open={detailsOpen}
        onOpenChange={setDetailsOpen}
        order={selectedOrder}
      />
    </>
  )
}
