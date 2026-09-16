import { useState } from 'react'
import {
  Star,
  Search as SearchIcon,
  MessageSquare,
  MessageSquareReply,
  Eye,
  EyeOff,
  AlertTriangle,
  Trash2,
  ArrowUpDown,
  CheckCircle2,
  RotateCcw,
  Image as ImageIcon,
  MoreHorizontal,
} from 'lucide-react'
import { toast } from 'sonner'
import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { ProfileDropdown } from '@/components/profile-dropdown'
import { Search } from '@/components/search'
import { ThemeSwitch } from '@/components/theme-switch'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useAdminReviews, useReviewMutations } from './hooks/use-reviews'
import { ReviewDetailModal } from './components/review-detail-modal'
import { ReviewReplyDialog } from './components/review-reply-dialog'
import { ReviewReportsDialog } from './components/review-reports-dialog'
import type { AdminReview, ReviewStatus } from './types'

export function Reviews() {
  const [page, setPage] = useState(1)
  const [searchTerm, setSearchTerm] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [selectedStatus, setSelectedStatus] = useState<string>('all')
  const [selectedRating, setSelectedRating] = useState<string>('all')
  const [selectedSort, setSelectedSort] = useState<'desc' | 'asc'>('desc')

  // Modals state
  const [detailReview, setDetailReview] = useState<AdminReview | null>(null)
  const [detailModalOpen, setDetailModalOpen] = useState(false)

  const [replyReview, setReplyReview] = useState<AdminReview | null>(null)
  const [replyDialogOpen, setReplyDialogOpen] = useState(false)

  const [reportsReview, setReportsReview] = useState<AdminReview | null>(null)
  const [reportsDialogOpen, setReportsDialogOpen] = useState(false)

  const [deleteReview, setDeleteReview] = useState<AdminReview | null>(null)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

  const statusParam =
    selectedStatus === 'all'
      ? undefined
      : (parseInt(selectedStatus, 10) as ReviewStatus)

  const ratingParam =
    selectedRating === 'all'
      ? undefined
      : (parseInt(selectedRating, 10) as 1 | 2 | 3 | 4 | 5)

  const { data, isLoading, refetch } = useAdminReviews({
    page,
    limit: 10,
    search: activeSearch || undefined,
    status: statusParam,
    rating: ratingParam,
    sort: selectedSort,
  })

  const { updateStatus, forceDeleteReview } = useReviewMutations()

  const reviews = (data?.data as AdminReview[]) || []
  const pagination = data?.pagination

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setActiveSearch(searchTerm)
    setPage(1)
  }

  const handleResetFilters = () => {
    setSearchTerm('')
    setActiveSearch('')
    setSelectedStatus('all')
    setSelectedRating('all')
    setSelectedSort('desc')
    setPage(1)
  }

  const handleToggleStatus = async (review: AdminReview) => {
    const nextStatus: ReviewStatus = review.status === 1 ? 2 : 1
    try {
      await updateStatus.mutateAsync({
        uuid: review.uuid,
        status: nextStatus,
      })
      toast.success(
        `Review marked as ${nextStatus === 1 ? 'Visible' : 'Hidden'}`
      )
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update status'
      toast.error(msg)
    }
  }

  const handleConfirmDelete = async () => {
    if (!deleteReview?.uuid) return
    try {
      await forceDeleteReview.mutateAsync(deleteReview.uuid)
      toast.success('Review permanently deleted')
      setDeleteConfirmOpen(false)
      setDeleteReview(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete review'
      toast.error(msg)
    }
  }

  const getStatusBadge = (status: ReviewStatus) => {
    switch (status) {
      case 1:
        return (
          <Badge
            variant="outline"
            className="bg-emerald-50 text-emerald-700 border-emerald-300 dark:bg-emerald-950 dark:text-emerald-300 text-[11px]"
          >
            Visible
          </Badge>
        )
      case 2:
        return (
          <Badge
            variant="outline"
            className="bg-zinc-100 text-zinc-700 border-zinc-300 dark:bg-zinc-800 dark:text-zinc-300 text-[11px]"
          >
            Hidden
          </Badge>
        )
      case 3:
        return (
          <Badge variant="destructive" className="flex items-center gap-1 text-[11px]">
            <AlertTriangle className="h-3 w-3" />
            Reported
          </Badge>
        )
      default:
        return <Badge variant="secondary">Unknown</Badge>
    }
  }

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleDateString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
      })
    } catch {
      return dateStr
    }
  }

  return (
    <>
      <Header fixed>
        <Search />
        <div className="ml-auto flex items-center space-x-4">
          <ThemeSwitch />
          <ProfileDropdown />
        </div>
      </Header>

      <Main className="space-y-6">
        {/* Page Title */}
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">
              Customer Reviews & Moderation
            </h1>
            <p className="text-sm text-muted-foreground">
              Monitor, moderate customer reviews, respond with store replies, and investigate flagged content.
            </p>
          </div>
        </div>

        {/* Filters and Search Bar */}
        <div className="flex flex-col md:flex-row gap-3 items-stretch md:items-center justify-between bg-card p-3 rounded-lg border shadow-sm">
          <form onSubmit={handleSearchSubmit} className="flex gap-2 flex-1 max-w-md">
            <div className="relative flex-1">
              <SearchIcon className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input
                type="text"
                placeholder="Search reviews by keyword, user, or product..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="pl-9 h-9"
              />
            </div>
            <Button type="submit" size="sm" className="h-9">
              Search
            </Button>
          </form>

          <div className="flex flex-wrap items-center gap-2">
            {/* Status Filter */}
            <Select
              value={selectedStatus}
              onValueChange={(val) => {
                setSelectedStatus(val)
                setPage(1)
              }}
            >
              <SelectTrigger className="h-9 w-[130px] text-xs">
                <SelectValue placeholder="Status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Statuses</SelectItem>
                <SelectItem value="1">Visible Only</SelectItem>
                <SelectItem value="2">Hidden Only</SelectItem>
                <SelectItem value="3">Reported Only</SelectItem>
              </SelectContent>
            </Select>

            {/* Rating Filter */}
            <Select
              value={selectedRating}
              onValueChange={(val) => {
                setSelectedRating(val)
                setPage(1)
              }}
            >
              <SelectTrigger className="h-9 w-[120px] text-xs">
                <SelectValue placeholder="Rating" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Stars</SelectItem>
                <SelectItem value="5">★★★★★ (5)</SelectItem>
                <SelectItem value="4">★★★★☆ (4)</SelectItem>
                <SelectItem value="3">★★★☆☆ (3)</SelectItem>
                <SelectItem value="2">★★☆☆☆ (2)</SelectItem>
                <SelectItem value="1">★☆☆☆☆ (1)</SelectItem>
              </SelectContent>
            </Select>

            {/* Sort Order */}
            <Button
              variant="outline"
              size="sm"
              className="h-9 text-xs flex items-center gap-1"
              onClick={() => {
                setSelectedSort((prev) => (prev === 'desc' ? 'asc' : 'desc'))
                setPage(1)
              }}
            >
              <ArrowUpDown className="h-3.5 w-3.5" />
              {selectedSort === 'desc' ? 'Newest First' : 'Oldest First'}
            </Button>

            {/* Reset */}
            {(activeSearch || selectedStatus !== 'all' || selectedRating !== 'all') && (
              <Button
                variant="ghost"
                size="sm"
                className="h-9 text-xs text-muted-foreground"
                onClick={handleResetFilters}
              >
                <RotateCcw className="h-3.5 w-3.5 mr-1" /> Reset
              </Button>
            )}
          </div>
        </div>

        {/* Reviews Data Table */}
        <div className="rounded-lg border bg-card shadow-sm overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="w-[180px]">Customer</TableHead>
                <TableHead className="w-[200px]">Product</TableHead>
                <TableHead className="w-[100px]">Rating</TableHead>
                <TableHead>Review Content</TableHead>
                <TableHead className="w-[100px] text-center">Status</TableHead>
                <TableHead className="w-[110px] text-center">Store Reply</TableHead>
                <TableHead className="w-[110px]">Date</TableHead>
                <TableHead className="w-[80px] text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center py-10 text-muted-foreground text-sm">
                    Loading reviews data...
                  </TableCell>
                </TableRow>
              ) : reviews.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="text-center py-12 text-muted-foreground">
                    <div className="flex flex-col items-center justify-center space-y-2">
                      <MessageSquare className="h-8 w-8 text-muted-foreground/60" />
                      <p className="font-medium text-sm">No reviews found</p>
                      <p className="text-xs">Try adjusting your filters or search query.</p>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                reviews.map((r) => (
                  <TableRow key={r.uuid} className="hover:bg-muted/30 transition">
                    {/* Customer */}
                    <TableCell>
                      <div className="space-y-0.5">
                        <div className="font-medium text-xs text-foreground truncate max-w-[160px]">
                          {r.reviewer_name}
                        </div>
                        <div className="text-[11px] text-muted-foreground truncate max-w-[160px]">
                          {r.reviewer_email}
                        </div>
                      </div>
                    </TableCell>

                    {/* Product */}
                    <TableCell>
                      <div className="font-medium text-xs text-foreground line-clamp-2 max-w-[180px]">
                        {r.product_name}
                      </div>
                    </TableCell>

                    {/* Rating */}
                    <TableCell>
                      <div className="flex items-center gap-1">
                        <Star className="h-3.5 w-3.5 fill-amber-500 text-amber-500" />
                        <span className="font-bold text-xs">{r.rating}</span>
                        <span className="text-[10px] text-muted-foreground">/5</span>
                      </div>
                    </TableCell>

                    {/* Review Content & Media */}
                    <TableCell>
                      <div className="space-y-1 max-w-[340px]">
                        {r.title && <div className="font-semibold text-xs text-foreground line-clamp-1">{r.title}</div>}
                        {r.comment && <div className="text-xs text-muted-foreground line-clamp-2">{r.comment}</div>}
                        
                        <div className="flex items-center gap-2 pt-0.5">
                          {r.images && r.images.length > 0 && (
                            <Badge variant="outline" className="text-[10px] h-4 py-0 px-1 gap-1 text-muted-foreground">
                              <ImageIcon className="h-2.5 w-2.5" />
                              {r.images.length}
                            </Badge>
                          )}
                          {r.report_count > 0 && (
                            <button
                              type="button"
                              onClick={() => {
                                setReportsReview(r)
                                setReportsDialogOpen(true)
                              }}
                              className="inline-flex items-center gap-1 text-[10px] font-bold text-destructive hover:underline"
                            >
                              <AlertTriangle className="h-3 w-3" />
                              {r.report_count} reports
                            </button>
                          )}
                        </div>
                      </div>
                    </TableCell>

                    {/* Status Badge */}
                    <TableCell className="text-center">{getStatusBadge(r.status)}</TableCell>

                    {/* Store Reply status */}
                    <TableCell className="text-center">
                      {r.reply ? (
                        <Badge
                          variant="secondary"
                          className="bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950 dark:text-blue-300 text-[10px] inline-flex items-center gap-1"
                        >
                          <CheckCircle2 className="h-2.5 w-2.5" />
                          Replied
                        </Badge>
                      ) : (
                        <span className="text-[11px] text-muted-foreground italic">No reply</span>
                      )}
                    </TableCell>

                    {/* Date */}
                    <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                      {formatDate(r.created_at)}
                    </TableCell>

                    {/* Actions Menu */}
                    <TableCell className="text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-44 text-xs">
                          <DropdownMenuItem
                            onClick={() => {
                              setDetailReview(r)
                              setDetailModalOpen(true)
                            }}
                          >
                            <Eye className="mr-2 h-3.5 w-3.5" />
                            View Details
                          </DropdownMenuItem>

                          <DropdownMenuItem
                            onClick={() => {
                              setReplyReview(r)
                              setReplyDialogOpen(true)
                            }}
                          >
                            <MessageSquareReply className="mr-2 h-3.5 w-3.5 text-primary" />
                            {r.reply ? 'Edit Reply' : 'Post Reply'}
                          </DropdownMenuItem>

                          <DropdownMenuItem onClick={() => handleToggleStatus(r)}>
                            {r.status === 1 ? (
                              <>
                                <EyeOff className="mr-2 h-3.5 w-3.5" />
                                Hide Review
                              </>
                            ) : (
                              <>
                                <Eye className="mr-2 h-3.5 w-3.5 text-emerald-600" />
                                Show Review
                              </>
                            )}
                          </DropdownMenuItem>

                          {r.report_count > 0 && (
                            <DropdownMenuItem
                              onClick={() => {
                                setReportsReview(r)
                                setReportsDialogOpen(true)
                              }}
                              className="text-destructive focus:text-destructive"
                            >
                              <AlertTriangle className="mr-2 h-3.5 w-3.5" />
                              View Reports ({r.report_count})
                            </DropdownMenuItem>
                          )}

                          <DropdownMenuSeparator />

                          <DropdownMenuItem
                            onClick={() => {
                              setDeleteReview(r)
                              setDeleteConfirmOpen(true)
                            }}
                            className="text-destructive focus:text-destructive"
                          >
                            <Trash2 className="mr-2 h-3.5 w-3.5" />
                            Force Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>

          {/* Pagination */}
          {pagination && (pagination.total_pages ?? 1) > 1 && (
            <div className="flex items-center justify-between px-4 py-3 border-t text-xs text-muted-foreground bg-muted/20">
              <div>
                Showing page <span className="font-medium text-foreground">{page}</span> of{' '}
                <span className="font-medium text-foreground">{pagination.total_pages}</span> ({pagination.total_records ?? 0} total reviews)
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => setPage((p) => Math.max(p - 1, 1))}
                  disabled={page <= 1}
                >
                  Previous
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => setPage((p) => Math.min(p + 1, pagination.total_pages ?? 1))}
                  disabled={page >= (pagination.total_pages ?? 1)}
                >
                  Next
                </Button>
              </div>
            </div>
          )}
        </div>
      </Main>

      {/* Review Detail Modal */}
      <ReviewDetailModal
        open={detailModalOpen}
        onOpenChange={setDetailModalOpen}
        review={detailReview}
        onOpenReply={(r) => {
          setReplyReview(r)
          setReplyDialogOpen(true)
        }}
        onOpenReports={(r) => {
          setReportsReview(r)
          setReportsDialogOpen(true)
        }}
      />

      {/* Reply Dialog */}
      <ReviewReplyDialog
        open={replyDialogOpen}
        onOpenChange={setReplyDialogOpen}
        review={replyReview}
      />

      {/* Reports Investigation Dialog */}
      <ReviewReportsDialog
        open={reportsDialogOpen}
        onOpenChange={setReportsDialogOpen}
        review={reportsReview}
        onRefresh={refetch}
      />

      {/* Permanent Delete Confirmation Dialog */}
      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title="Permanently Delete Review"
        desc={`Are you sure you want to permanently delete the review by "${deleteReview?.reviewer_name}"? This will delete the review and all uploaded media files.`}
        confirmText="Delete"
        destructive
        handleConfirm={handleConfirmDelete}
        isLoading={forceDeleteReview.isPending}
      />
    </>
  )
}
