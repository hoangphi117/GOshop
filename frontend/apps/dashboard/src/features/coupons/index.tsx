import { useState } from 'react'
import { Plus, Search as SearchIcon, Edit2, Trash2, Tag, Calendar } from 'lucide-react'
import { toast } from 'sonner'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useCoupons, useCouponMutations } from './hooks/use-coupons'
import { CouponDialog } from './components/coupon-dialog'
import type { Coupon } from './types'

export function Coupons() {
  const [page, setPage] = useState(1)
  const [searchTerm, setSearchTerm] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [selectedCoupon, setSelectedCoupon] = useState<Coupon | null>(null)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [couponToDelete, setCouponToDelete] = useState<Coupon | null>(null)

  const { data, isLoading } = useCoupons({
    page,
    limit: 10,
    search: activeSearch || undefined,
  })

  const { deleteCoupon } = useCouponMutations()

  const coupons = (data?.data as Coupon[]) || []
  const pagination = data?.pagination

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setActiveSearch(searchTerm)
    setPage(1)
  }

  const handleOpenCreate = () => {
    setSelectedCoupon(null)
    setDialogOpen(true)
  }

  const handleOpenEdit = (c: Coupon) => {
    setSelectedCoupon(c)
    setDialogOpen(true)
  }

  const handleOpenDelete = (c: Coupon) => {
    setCouponToDelete(c)
    setDeleteConfirmOpen(true)
  }

  const handleConfirmDelete = async () => {
    if (!couponToDelete?.uuid) return
    try {
      await deleteCoupon.mutateAsync(couponToDelete.uuid)
      toast.success(`Coupon "${couponToDelete.code}" deleted successfully`)
      setDeleteConfirmOpen(false)
      setCouponToDelete(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete coupon'
      toast.error(msg)
    }
  }

  const formatDiscount = (c: Coupon) => {
    if (c.type === 1) {
      return `${c.discount_value}%`
    }
    return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(
      c.discount_value ?? 0
    )
  }

  const formatPrice = (price?: number) => {
    if (!price) return '0 ₫'
    return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(price)
  }

  const formatDate = (d?: string) => {
    if (!d) return '—'
    return new Date(d).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  }

  return (
    <>
      <DashboardHeader fixed />

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Coupons</h2>
            <p className='text-muted-foreground'>
              Manage discount campaigns, coupon codes, and redemption limits.
            </p>
          </div>
          <Button onClick={handleOpenCreate} className='gap-2'>
            <Plus className='size-4' /> Add Coupon
          </Button>
        </div>

        {/* Filter bar */}
        <form onSubmit={handleSearchSubmit} className='flex items-center gap-2 max-w-sm'>
          <div className='relative flex-1'>
            <SearchIcon className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
            <Input
              placeholder='Search by code or campaign name...'
              className='pl-9'
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
          <Button type='submit' variant='secondary' size='sm'>
            Search
          </Button>
          {activeSearch && (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => {
                setSearchTerm('')
                setActiveSearch('')
              }}
            >
              Reset
            </Button>
          )}
        </form>

        {/* Coupons Table */}
        <div className='rounded-md border bg-card'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Coupon Code</TableHead>
                <TableHead>Campaign Name</TableHead>
                <TableHead>Discount</TableHead>
                <TableHead>Min Order</TableHead>
                <TableHead>Usage</TableHead>
                <TableHead>Validity Period</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    Loading coupons...
                  </TableCell>
                </TableRow>
              ) : coupons.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    No coupons found.
                  </TableCell>
                </TableRow>
              ) : (
                coupons.map((c) => (
                  <TableRow key={c.uuid}>
                    <TableCell>
                      <div className='flex items-center gap-1.5 font-mono font-bold text-primary'>
                        <Tag className='size-3.5' /> {c.code}
                      </div>
                    </TableCell>
                    <TableCell className='font-medium'>{c.name}</TableCell>
                    <TableCell className='font-semibold text-emerald-600 dark:text-emerald-400'>
                      {formatDiscount(c)}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {formatPrice(c.min_order_amount)}
                    </TableCell>
                    <TableCell className='text-sm'>
                      <div>
                        <span className='font-semibold'>{c.used_count ?? 0}</span>
                        <span className='text-muted-foreground'>
                          {' '}
                          / {c.usage_limit ?? '∞'}
                        </span>
                      </div>
                      {Boolean(c.user_usage_limit) && (
                        <div className='text-xs text-muted-foreground'>
                          Max {c.user_usage_limit}/user
                        </div>
                      )}
                    </TableCell>
                    <TableCell className='text-xs text-muted-foreground'>
                      <div className='flex items-center gap-1'>
                        <Calendar className='size-3' />
                        {formatDate(c.start_date)} – {formatDate(c.end_date)}
                      </div>
                    </TableCell>
                    <TableCell>
                      {c.is_expired ? (
                        <Badge variant='outline' className='text-destructive border-destructive/20'>
                          Expired
                        </Badge>
                      ) : c.is_active ? (
                        <Badge
                          variant='outline'
                          className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                        >
                          Active
                        </Badge>
                      ) : (
                        <Badge variant='secondary'>Inactive</Badge>
                      )}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8'
                          onClick={() => handleOpenEdit(c)}
                          title='Edit'
                        >
                          <Edit2 className='size-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8 text-destructive hover:bg-destructive/10'
                          onClick={() => handleOpenDelete(c)}
                          title='Delete'
                        >
                          <Trash2 className='size-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>

        {/* Pagination */}
        {pagination && pagination.total_pages && pagination.total_pages > 1 && (
          <div className='flex items-center justify-between text-sm text-muted-foreground'>
            <div>
              Showing page {pagination.page} of {pagination.total_pages} ({pagination.total_records} total coupons)
            </div>
            <div className='flex items-center gap-2'>
              <Button
                variant='outline'
                size='sm'
                disabled={!pagination.has_prev}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Previous
              </Button>
              <Button
                variant='outline'
                size='sm'
                disabled={!pagination.has_next}
                onClick={() => setPage((p) => p + 1)}
              >
                Next
              </Button>
            </div>
          </div>
        )}
      </Main>

      <CouponDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        coupon={selectedCoupon}
      />

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title='Delete Coupon'
        desc={`Are you sure you want to delete coupon "${couponToDelete?.code}"?`}
        confirmText='Delete'
        destructive
        isLoading={deleteCoupon.isPending}
        handleConfirm={handleConfirmDelete}
      />
    </>
  )
}
