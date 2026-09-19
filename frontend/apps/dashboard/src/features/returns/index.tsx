import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import {
  Search as SearchIcon,
  RotateCcw,
  Filter,
  Eye,
  ImageIcon,
  ExternalLink,
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
import { useAdminReturns } from '../orders/hooks/use-returns'
import { formatCurrency, formatDateTime } from '../orders/order-utils'
import { RETURN_STATUS_MAP, type AdminReturnSummary } from '../orders/types'

export function Returns() {
  const navigate = useNavigate()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [searchTerm, setSearchTerm] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  const parsedStatus =
    statusFilter !== 'all'
      ? (Number(statusFilter) as 1 | 2 | 3 | 4 | 5)
      : undefined

  const { data: returnsData, isLoading } = useAdminReturns({
    page,
    limit: pageSize,
    search: activeSearch || undefined,
    status: parsedStatus,
  })

  const returns = returnsData?.data || []
  const total = returnsData?.total || 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

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

  const handleOpenOrderDetail = (orderUUID: string) => {
    navigate({ to: '/orders/$uuid', params: { uuid: orderUUID } })
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
            <div className='flex items-center gap-2'>
              <div className='flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary'>
                <RotateCcw className='size-5' />
              </div>
              <h2 className='text-2xl font-bold tracking-tight'>
                Return Requests
              </h2>
            </div>
            <p className='text-muted-foreground text-xs sm:text-sm mt-1'>
              Inspect returned items, manage approval decisions, restock inventory, and process customer refunds.
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
                placeholder='Search order #, customer name, reason...'
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
                <SelectTrigger className='w-[170px] h-9 text-xs'>
                  <SelectValue placeholder='All Return Statuses' />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>All Statuses</SelectItem>
                  <SelectItem value='1'>Pending Approval</SelectItem>
                  <SelectItem value='2'>Approved (Awaiting Return)</SelectItem>
                  <SelectItem value='3'>Rejected</SelectItem>
                  <SelectItem value='4'>Completed / Refunded</SelectItem>
                  <SelectItem value='5'>Cancelled</SelectItem>
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

        {/* Returns Table */}
        <div className='rounded-md border bg-card'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Order #</TableHead>
                <TableHead>Customer</TableHead>
                <TableHead>Reason & Proof</TableHead>
                <TableHead>Return Carrier / Code</TableHead>
                <TableHead>Order Amount</TableHead>
                <TableHead>Return Status</TableHead>
                <TableHead>Updated At</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell
                    colSpan={8}
                    className='h-32 text-center text-muted-foreground'
                  >
                    Loading return requests...
                  </TableCell>
                </TableRow>
              ) : returns.length === 0 ? (
                <TableRow>
                  <TableCell
                    colSpan={8}
                    className='h-32 text-center text-muted-foreground'
                  >
                    No return requests found.
                  </TableCell>
                </TableRow>
              ) : (
                returns.map((item: AdminReturnSummary) => {
                  const statusConfig = RETURN_STATUS_MAP[item.status] || {
                    label: item.status_text || item.status_name,
                    variant: 'secondary' as const,
                  }

                  return (
                    <TableRow
                      key={item.return_uuid}
                      className='cursor-pointer hover:bg-muted/50 transition-colors'
                      onClick={() => handleOpenOrderDetail(item.order_uuid)}
                    >
                      <TableCell className='font-mono font-medium text-xs'>
                        <span className='font-semibold text-foreground'>
                          #{item.order_number}
                        </span>
                        <div className='text-[10px] text-muted-foreground font-normal truncate max-w-[120px]'>
                          {item.return_uuid.slice(0, 8)}...
                        </div>
                      </TableCell>

                      <TableCell>
                        <div className='font-medium text-xs sm:text-sm'>
                          {item.customer_name || item.user_fullname || 'Customer'}
                        </div>
                        {(item.customer_email || item.user_email) && (
                          <div className='text-xs text-muted-foreground truncate max-w-[150px]'>
                            {item.customer_email || item.user_email}
                          </div>
                        )}
                      </TableCell>

                      <TableCell>
                        <div
                          className='text-xs font-medium text-foreground truncate max-w-[200px]'
                          title={item.reason}
                        >
                          {item.reason}
                        </div>
                        <div className='mt-1 flex items-center gap-2 text-[11px] text-muted-foreground'>
                          {item.images && item.images.length > 0 && (
                            <span className='inline-flex items-center gap-1'>
                              <ImageIcon className='size-3 text-muted-foreground' />
                              {item.images.length} photos
                            </span>
                          )}
                          {item.video_url && (
                            <span className='inline-flex items-center gap-1 text-primary'>
                              <ExternalLink className='size-3' /> Video proof
                            </span>
                          )}
                        </div>
                      </TableCell>

                      <TableCell>
                        {item.shipping_code ? (
                          <div className='space-y-0.5'>
                            <div className='text-xs font-semibold text-foreground'>
                              {item.shipping_provider || 'Carrier'}
                            </div>
                            <div className='font-mono text-[11px] text-muted-foreground'>
                              {item.shipping_code}
                            </div>
                          </div>
                        ) : (
                          <span className='text-xs text-muted-foreground/60 italic'>
                            Not submitted
                          </span>
                        )}
                      </TableCell>

                      <TableCell className='font-semibold text-xs sm:text-sm'>
                        {formatCurrency(item.total_amount)}
                      </TableCell>

                      <TableCell>
                        <Badge variant={statusConfig.variant} className='text-xs'>
                          {statusConfig.label}
                        </Badge>
                      </TableCell>

                      <TableCell className='text-muted-foreground text-xs'>
                        {formatDateTime(item.updated_at || item.created_at)}
                      </TableCell>

                      <TableCell className='text-right'>
                        <div className='flex items-center justify-end gap-1'>
                          <Button
                            variant='outline'
                            size='sm'
                            className='h-7 text-xs gap-1'
                            onClick={(e) => {
                              e.stopPropagation()
                              handleOpenOrderDetail(item.order_uuid)
                            }}
                          >
                            <Eye className='size-3.5' />
                            Inspect
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })
              )}
            </TableBody>
          </Table>
        </div>

        {/* Pagination */}
        <DataTablePagination
          page={page}
          pageSize={pageSize}
          totalPages={totalPages}
          onPageChange={setPage}
          onPageSizeChange={(newSize) => {
            setPageSize(newSize)
            setPage(1)
          }}
          className='mt-auto'
        />
      </Main>
    </>
  )
}
