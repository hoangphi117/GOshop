import { useState, useEffect, useRef } from 'react'
import { Link } from '@tanstack/react-router'
import { Plus, Search as SearchIcon, Eye, Trash2, Package, FileSpreadsheet, Sparkles, Loader2, ArrowRight } from 'lucide-react'
import { toast } from 'sonner'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { getImageUrl } from '@/lib/utils'
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
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useCategories } from '@/features/categories/hooks/use-categories'
import { useProducts, useProductMutations, useProductSuggestions } from './hooks/use-products'
import { ProductDialog } from './components/product-dialog'
import { CSVImportDialog } from './components/csv-import-dialog'
import type { Product } from './types'

export function Products() {
  const [page, setPage] = useState(1)
  const [searchTerm, setSearchTerm] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [selectedCategoryUuid, setSelectedCategoryUuid] = useState<string>('all')
  const [isDropdownOpen, setIsDropdownOpen] = useState(false)

  const searchContainerRef = useRef<HTMLDivElement>(null)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [importDialogOpen, setImportDialogOpen] = useState(false)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [productToDelete, setProductToDelete] = useState<Product | null>(null)

  // Debounce search term to trigger Meilisearch search on the main table
  useEffect(() => {
    const timer = setTimeout(() => {
      setActiveSearch(searchTerm.trim())
      setPage(1)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchTerm])

  // Close dropdown on click outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (searchContainerRef.current && !searchContainerRef.current.contains(e.target as Node)) {
        setIsDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const { data: suggestions = [], isFetching: isSearchingSuggestions } = useProductSuggestions(
    searchTerm,
    6,
  )

  const { data: categoriesData } = useCategories({ limit: 100 })
  const categories = (categoriesData?.data as Array<{ uuid?: string; category_name?: string }>) || []

  const { data: productsData, isLoading } = useProducts({
    page,
    limit: 10,
    search: activeSearch,
    category_uuid: selectedCategoryUuid === 'all' ? undefined : selectedCategoryUuid,
  })

  const { deleteProduct } = useProductMutations()

  const products = (productsData?.data as Product[]) || []
  const pagination = productsData?.pagination

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setActiveSearch(searchTerm.trim())
    setIsDropdownOpen(false)
    setPage(1)
  }

  const handleSelectSuggestion = (productName: string) => {
    setSearchTerm(productName)
    setActiveSearch(productName)
    setIsDropdownOpen(false)
    setPage(1)
  }

  const handleOpenCreate = () => {
    setDialogOpen(true)
  }

  const handleOpenDelete = (p: Product) => {
    setProductToDelete(p)
    setDeleteConfirmOpen(true)
  }

  const handleConfirmDelete = async () => {
    if (!productToDelete?.uuid) return
    try {
      await deleteProduct.mutateAsync(productToDelete.uuid)
      toast.success(`Deleted product "${productToDelete.name}"`)
      setDeleteConfirmOpen(false)
      setProductToDelete(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete product'
      toast.error(msg)
    }
  }

  const formatPrice = (price?: number) => {
    if (price === undefined || price === null) return '0 ₫'
    return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND' }).format(price)
  }

  return (
    <>
      <DashboardHeader fixed />

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-2'>
          <div>
            <h2 className='text-2xl font-bold tracking-tight'>Products</h2>
            <p className='text-muted-foreground'>
              Manage your product catalog, pricing, inventory and images.
            </p>
          </div>
          <div className='flex items-center gap-2'>
            <Button
              variant='outline'
              onClick={() => setImportDialogOpen(true)}
              className='gap-2'
            >
              <FileSpreadsheet className='size-4' /> Import CSV
            </Button>
            <Button onClick={handleOpenCreate} className='gap-2'>
              <Plus className='size-4' /> Add Product
            </Button>
          </div>
        </div>

        {/* Filter Bar */}
        <div className='flex flex-wrap items-center gap-3'>
          <div ref={searchContainerRef} className='relative max-w-sm flex-1'>
            <form onSubmit={handleSearchSubmit} className='flex items-center gap-2'>
              <div className='relative flex-1'>
                <SearchIcon className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
                <Input
                  placeholder='Search by name, SKU...'
                  className='pl-9 pr-8'
                  value={searchTerm}
                  onFocus={() => {
                    if (searchTerm.trim().length >= 1) {
                      setIsDropdownOpen(true)
                    }
                  }}
                  onChange={(e) => {
                    setSearchTerm(e.target.value)
                    if (e.target.value.trim().length >= 1) {
                      setIsDropdownOpen(true)
                    } else {
                      setIsDropdownOpen(false)
                    }
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Escape') {
                      setIsDropdownOpen(false)
                    }
                  }}
                />
                {isSearchingSuggestions && (
                  <Loader2 className='absolute right-2.5 top-1/2 size-4 -translate-y-1/2 animate-spin text-muted-foreground' />
                )}
              </div>
              <Button type='submit' variant='secondary' size='sm'>
                Search
              </Button>
            </form>

            {/* Meilisearch Suggestions Dropdown */}
            {isDropdownOpen && searchTerm.trim().length >= 1 && (
              <div className='absolute top-full left-0 mt-1.5 w-full sm:w-[420px] bg-popover text-popover-foreground border rounded-lg shadow-xl z-50 overflow-hidden animate-in fade-in-50 zoom-in-95'>
                <div className='px-3 py-2 bg-muted/50 border-b flex items-center justify-between text-xs font-medium text-muted-foreground'>
                  <span className='flex items-center gap-1.5'>
                    <Sparkles className='size-3.5 text-primary' /> Instant Meilisearch Suggestions
                  </span>
                  <span>{suggestions.length} found</span>
                </div>

                <div className='max-h-[300px] overflow-y-auto divide-y divide-border/40'>
                  {suggestions.length > 0 ? (
                    suggestions.map((item) => (
                      <div
                        key={item.uuid}
                        onClick={() => handleSelectSuggestion(item.name)}
                        className='px-3 py-2.5 flex items-center justify-between gap-3 hover:bg-muted/60 cursor-pointer transition-colors group'
                      >
                        <div className='flex items-center gap-2.5 min-w-0'>
                          {item.image_url ? (
                            <img
                              src={getImageUrl(item.image_url)}
                              alt={item.name}
                              className='size-9 rounded-md object-cover border shrink-0 bg-muted'
                            />
                          ) : (
                            <div className='size-9 rounded-md bg-muted flex items-center justify-center shrink-0 border'>
                              <Package className='size-4 text-muted-foreground' />
                            </div>
                          )}
                          <div className='min-w-0'>
                            <p className='text-sm font-medium text-foreground truncate group-hover:text-primary transition-colors'>
                              {item.name}
                            </p>
                            <div className='flex items-center gap-2 text-xs text-muted-foreground'>
                              {item.category_name && (
                                <span className='truncate'>{item.category_name}</span>
                              )}
                              <span>•</span>
                              <span className='font-semibold text-primary/90'>
                                {formatPrice(item.price)}
                              </span>
                            </div>
                          </div>
                        </div>

                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-7 shrink-0 text-muted-foreground hover:text-foreground opacity-0 group-hover:opacity-100 transition-opacity'
                          asChild
                          onClick={(e) => e.stopPropagation()}
                          title='View details'
                        >
                          <Link
                            to='/products/$uuid'
                            params={{ uuid: item.uuid }}
                            preload='intent'
                          >
                            <Eye className='size-3.5' />
                          </Link>
                        </Button>
                      </div>
                    ))
                  ) : !isSearchingSuggestions ? (
                    <div className='py-6 text-center text-sm text-muted-foreground'>
                      No products found matching &ldquo;{searchTerm}&rdquo;
                    </div>
                  ) : (
                    <div className='py-6 flex items-center justify-center gap-2 text-sm text-muted-foreground'>
                      <Loader2 className='size-4 animate-spin' /> Searching products...
                    </div>
                  )}
                </div>

                <div
                  onClick={() => {
                    setActiveSearch(searchTerm.trim())
                    setIsDropdownOpen(false)
                    setPage(1)
                  }}
                  className='px-3 py-2 bg-muted/30 border-t flex items-center justify-between text-xs text-primary font-medium hover:bg-muted/70 cursor-pointer transition-colors'
                >
                  <span>Filter all table results for &ldquo;{searchTerm}&rdquo;</span>
                  <ArrowRight className='size-3.5' />
                </div>
              </div>
            )}
          </div>

          <Select
            value={selectedCategoryUuid}
            onValueChange={(val) => {
              setSelectedCategoryUuid(val)
              setPage(1)
            }}
          >
            <SelectTrigger className='w-[200px]'>
              <SelectValue placeholder='Filter by category' />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='all'>All Categories</SelectItem>
              {categories.map((c) => (
                <SelectItem key={c.uuid} value={c.uuid || ''}>
                  {c.category_name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {(activeSearch || selectedCategoryUuid !== 'all') && (
            <Button
              type='button'
              variant='ghost'
              size='sm'
              onClick={() => {
                setSearchTerm('')
                setActiveSearch('')
                setSelectedCategoryUuid('all')
                setIsDropdownOpen(false)
                setPage(1)
              }}
            >
              Reset Filters
            </Button>
          )}
        </div>

        {/* Products Table */}
        <div className='rounded-md border bg-card'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className='w-[80px]'>Image</TableHead>
                <TableHead>Product Name</TableHead>
                <TableHead>Category</TableHead>
                <TableHead>SKU</TableHead>
                <TableHead>Price</TableHead>
                <TableHead>Stock</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    Loading products...
                  </TableCell>
                </TableRow>
              ) : products.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className='h-32 text-center text-muted-foreground'>
                    No products found.
                  </TableCell>
                </TableRow>
              ) : (
                products.map((p) => (
                  <TableRow key={p.uuid}>
                    <TableCell>
                      <Link
                        to='/products/$uuid'
                        params={{ uuid: p.uuid! }}
                        preload='intent'
                        className='block shrink-0 transition-opacity hover:opacity-80'
                      >
                        {p.image_url ? (
                          <img
                            src={getImageUrl(p.image_url)}
                            alt={p.name}
                            className='size-12 rounded object-cover border'
                            onError={(e) => {
                              ;(e.currentTarget as HTMLImageElement).src =
                                'https://placehold.co/48x48?text=Product'
                            }}
                          />
                        ) : (
                          <div className='size-12 rounded border bg-muted flex items-center justify-center text-muted-foreground'>
                            <Package className='size-5' />
                          </div>
                        )}
                      </Link>
                    </TableCell>
                    <TableCell className='font-medium max-w-[240px] truncate'>
                      <Link
                        to='/products/$uuid'
                        params={{ uuid: p.uuid! }}
                        preload='intent'
                        className='hover:underline hover:text-primary transition-colors'
                      >
                        {p.name}
                      </Link>
                    </TableCell>
                    <TableCell className='text-muted-foreground text-sm'>
                      {p.category_name || '—'}
                    </TableCell>
                    <TableCell className='font-mono text-xs text-muted-foreground'>
                      {p.sku || '—'}
                    </TableCell>
                    <TableCell className='font-semibold text-primary'>
                      {formatPrice(p.price)}
                    </TableCell>
                    <TableCell>
                      <span
                        className={`text-sm font-medium ${
                          (p.stock ?? 0) <= 5
                            ? 'text-destructive font-bold'
                            : 'text-foreground'
                        }`}
                      >
                        {p.stock ?? 0}
                      </span>
                    </TableCell>
                    <TableCell>
                      {(() => {
                        const s = p.status?.toString().toLowerCase()
                        if (s === 'active' || s === '1') {
                          return (
                            <Badge
                              variant='outline'
                              className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                            >
                              Active
                            </Badge>
                          )
                        }
                        if (s === 'out of stock' || s === '3') {
                          return (
                            <Badge
                              variant='outline'
                              className='bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
                            >
                              Out of Stock
                            </Badge>
                          )
                        }
                        return <Badge variant='secondary'>Inactive</Badge>
                      })()}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8'
                          asChild
                          title='View details'
                        >
                          <Link
                            to='/products/$uuid'
                            params={{ uuid: p.uuid! }}
                            preload='intent'
                          >
                            <Eye className='size-4' />
                          </Link>
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8 text-destructive hover:bg-destructive/10'
                          onClick={() => handleOpenDelete(p)}
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
              Showing page {pagination.page} of {pagination.total_pages} (Total {pagination.total_records} products)
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

      <ProductDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
      />

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title='Delete Product'
        desc={`Are you sure you want to delete product "${productToDelete?.name}"?`}
        confirmText='Delete'
        destructive
        isLoading={deleteProduct.isPending}
        handleConfirm={handleConfirmDelete}
      />

      <CSVImportDialog
        open={importDialogOpen}
        onOpenChange={setImportDialogOpen}
      />
    </>
  )
}
