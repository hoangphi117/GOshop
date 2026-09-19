import { useState } from 'react'
import {
  Search as SearchIcon,
  Filter,
  PlusCircle,
  Package2,
  ArrowRight,
  TrendingUp,
  TrendingDown,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DataTablePagination } from '@/components/data-table'
import { useStockMovements } from './hooks/use-stock-movements'
import { MovementTypeBadge } from './components/movement-type-badge'
import { AdjustStockDialog } from './components/adjust-stock-dialog'
import type { StockMovement } from './types'

export function Inventory() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [searchTerm, setSearchTerm] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [typeFilter, setTypeFilter] = useState<string>('all')
  const [adjustDialogOpen, setAdjustDialogOpen] = useState(false)

  const { data: movementsData, isLoading } = useStockMovements({
    page,
    limit: pageSize,
    search: activeSearch || undefined,
    change_type: typeFilter !== 'all' ? typeFilter : undefined,
  })

  const movements = (movementsData?.data as unknown as StockMovement[]) || []
  const pagination = movementsData?.pagination

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setActiveSearch(searchTerm.trim())
    setPage(1)
  }

  const handleTypeFilterChange = (val: string) => {
    setTypeFilter(val)
    setPage(1)
  }

  const handleResetFilters = () => {
    setSearchTerm('')
    setActiveSearch('')
    setTypeFilter('all')
    setPage(1)
  }

  return (
    <>
      <Header fixed>
        <Search className='me-auto' />
        <ThemeSwitch />
        <ProfileDropdown />
      </Header>

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        {/* Title and Adjust Action */}
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Stock Movement Log</h2>
            <p className='text-muted-foreground'>
              Complete audit trail of product inventory changes across sales, cancellations, returns, and restocks.
            </p>
          </div>
          <Button
            size='sm'
            className='gap-1.5'
            onClick={() => setAdjustDialogOpen(true)}
          >
            <PlusCircle className='size-4' />
            <span>Adjust Stock</span>
          </Button>
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
                placeholder='Search product, SKU, or order #...'
                className='pl-9 text-xs h-9'
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
            </div>
            <Button type='submit' variant='secondary' size='sm' className='h-9 text-xs'>
              Search
            </Button>
          </form>

          <div className='flex items-center gap-2'>
            <div className='flex items-center gap-1.5'>
              <Filter className='size-4 text-muted-foreground' />
              <Select
                value={typeFilter}
                onValueChange={handleTypeFilterChange}
              >
                <SelectTrigger className='w-[170px] h-9 text-xs'>
                  <SelectValue placeholder='All Movement Types' />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all' className='text-xs'>All Movement Types</SelectItem>
                  <SelectItem value='ORDER_PLACED' className='text-xs'>Order Placed</SelectItem>
                  <SelectItem value='ORDER_CANCELLED' className='text-xs'>Order Cancelled</SelectItem>
                  <SelectItem value='ORDER_RETURNED' className='text-xs'>Order Returned</SelectItem>
                  <SelectItem value='RESTOCK' className='text-xs'>Restock</SelectItem>
                  <SelectItem value='MANUAL_ADJUSTMENT' className='text-xs'>Manual Adjustment</SelectItem>
                  <SelectItem value='INITIAL_STOCK' className='text-xs'>Initial Stock</SelectItem>
                  <SelectItem value='DAMAGED_DISCARD' className='text-xs'>Damaged / Write-off</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {(activeSearch || typeFilter !== 'all') && (
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='h-9 text-xs'
                onClick={handleResetFilters}
              >
                Reset
              </Button>
            )}
          </div>
        </div>

        {/* Movements Table */}
        <div className='rounded-md border bg-card'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Timestamp</TableHead>
                <TableHead>Product</TableHead>
                <TableHead>Movement Type</TableHead>
                <TableHead className='text-center'>Qty Change</TableHead>
                <TableHead>Stock Level</TableHead>
                <TableHead>Reference</TableHead>
                <TableHead>Actor / Staff</TableHead>
                <TableHead>Audit Note</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    Loading stock movements...
                  </TableCell>
                </TableRow>
              ) : movements.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    No stock movements recorded yet.
                  </TableCell>
                </TableRow>
              ) : (
                movements.map((m) => {
                  const isPositive = m.quantity_change > 0
                  return (
                    <TableRow key={m.movement_uuid} className='hover:bg-muted/50'>
                      <TableCell className='text-xs text-muted-foreground whitespace-nowrap'>
                        {m.created_at}
                      </TableCell>
                      <TableCell>
                        <div className='flex items-center gap-2.5 max-w-[240px]'>
                          {m.product_image_url ? (
                            <img
                              src={m.product_image_url}
                              alt={m.product_name}
                              className='size-9 rounded object-cover border shrink-0'
                            />
                          ) : (
                            <div className='size-9 rounded bg-muted flex items-center justify-center shrink-0 border'>
                              <Package2 className='size-4 text-muted-foreground' />
                            </div>
                          )}
                          <div className='min-w-0'>
                            <div className='font-medium text-xs truncate' title={m.product_name}>
                              {m.product_name}
                            </div>
                            {m.product_sku && (
                              <div className='text-[10px] font-mono text-muted-foreground'>
                                SKU: {m.product_sku}
                              </div>
                            )}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>
                        <MovementTypeBadge type={m.change_type} />
                      </TableCell>
                      <TableCell className='text-center whitespace-nowrap font-mono font-semibold text-xs'>
                        {isPositive ? (
                          <span className='inline-flex items-center gap-0.5 text-emerald-600 dark:text-emerald-400'>
                            <TrendingUp className='size-3.5' /> +{m.quantity_change}
                          </span>
                        ) : (
                          <span className='inline-flex items-center gap-0.5 text-rose-600 dark:text-rose-400'>
                            <TrendingDown className='size-3.5' /> {m.quantity_change}
                          </span>
                        )}
                      </TableCell>
                      <TableCell className='whitespace-nowrap'>
                        <div className='flex items-center gap-1.5 text-xs font-mono'>
                          <span className='text-muted-foreground'>{m.stock_before}</span>
                          <ArrowRight className='size-3 text-muted-foreground' />
                          <span className='font-semibold'>{m.stock_after}</span>
                        </div>
                      </TableCell>
                      <TableCell className='text-xs'>
                        {m.reference_id ? (
                          <span className='font-mono font-medium text-primary'>
                            #{m.reference_id}
                          </span>
                        ) : (
                          <span className='text-muted-foreground text-xs'>—</span>
                        )}
                      </TableCell>
                      <TableCell className='text-xs text-muted-foreground whitespace-nowrap'>
                        {m.actor_name || 'System'}
                      </TableCell>
                      <TableCell className='text-xs text-muted-foreground max-w-[180px] truncate' title={m.note || ''}>
                        {m.note || '—'}
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
          totalPages={pagination?.total_pages || 1}
          onPageChange={setPage}
          onPageSizeChange={(newSize) => {
            setPageSize(newSize)
            setPage(1)
          }}
          className='mt-auto'
        />

        {/* Adjust Stock Dialog */}
        <AdjustStockDialog
          open={adjustDialogOpen}
          onOpenChange={setAdjustDialogOpen}
        />
      </Main>
    </>
  )
}
