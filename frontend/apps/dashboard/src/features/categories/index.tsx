import { useState } from 'react'
import { Plus, Search as SearchIcon, Edit2, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
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
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useCategories, useCategoryMutations } from './hooks/use-categories'
import { CategoryDialog } from './components/category-dialog'
import type { Category } from './types'

export function Categories() {
  const [page, setPage] = useState(1)
  const [searchTerm, setSearchTerm] = useState('')
  const [activeSearch, setActiveSearch] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [selectedCategory, setSelectedCategory] = useState<Category | null>(null)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [categoryToDelete, setCategoryToDelete] = useState<Category | null>(null)

  const { data: categoriesData, isLoading } = useCategories({
    page,
    limit: 10,
    search: activeSearch,
  })

  const { deleteCategory } = useCategoryMutations()

  const categories = (categoriesData?.data as Category[]) || []
  const pagination = categoriesData?.pagination

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setActiveSearch(searchTerm.trim())
    setPage(1)
  }

  const handleOpenCreate = () => {
    setSelectedCategory(null)
    setDialogOpen(true)
  }

  const handleOpenEdit = (cat: Category) => {
    setSelectedCategory(cat)
    setDialogOpen(true)
  }

  const handleOpenDelete = (cat: Category) => {
    setCategoryToDelete(cat)
    setDeleteConfirmOpen(true)
  }

  const handleConfirmDelete = async () => {
    if (!categoryToDelete?.uuid) return
    try {
      await deleteCategory.mutateAsync(categoryToDelete.uuid)
      toast.success(`Deleted category "${categoryToDelete.category_name}"`)
      setDeleteConfirmOpen(false)
      setCategoryToDelete(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete category'
      toast.error(msg)
    }
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
            <h2 className='text-2xl font-bold tracking-tight'>Categories</h2>
            <p className='text-muted-foreground'>
              Manage product categories, structure and display states.
            </p>
          </div>
          <Button onClick={handleOpenCreate} className='gap-2'>
            <Plus className='size-4' /> Add Category
          </Button>
        </div>

        {/* Filter bar */}
        <form onSubmit={handleSearchSubmit} className='flex items-center gap-2 max-w-sm'>
          <div className='relative flex-1'>
            <SearchIcon className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
            <Input
              placeholder='Search categories...'
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

        {/* Categories Table */}
        <div className='rounded-md border bg-card'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Category Name</TableHead>
                <TableHead>Slug</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Created At</TableHead>
                <TableHead className='text-right'>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {isLoading ? (
                <TableRow>
                  <TableCell colSpan={6} className='h-32 text-center text-muted-foreground'>
                    Loading categories...
                  </TableCell>
                </TableRow>
              ) : categories.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className='h-32 text-center text-muted-foreground'>
                    No categories found.
                  </TableCell>
                </TableRow>
              ) : (
                categories.map((cat) => (
                  <TableRow key={cat.uuid}>
                    <TableCell className='font-medium'>{cat.category_name}</TableCell>
                    <TableCell className='text-muted-foreground font-mono text-xs'>
                      {cat.slug}
                    </TableCell>
                    <TableCell className='max-w-[280px] truncate text-muted-foreground'>
                      {cat.description || '—'}
                    </TableCell>
                    <TableCell>
                      {cat.status?.toLowerCase() === 'active' || cat.status === '1' ? (
                        <Badge variant='outline' className='bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'>
                          Active
                        </Badge>
                      ) : (
                        <Badge variant='secondary'>Inactive</Badge>
                      )}
                    </TableCell>
                    <TableCell className='text-muted-foreground text-xs'>
                      {cat.created_at ? new Date(cat.created_at).toLocaleDateString('en-US') : '—'}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8'
                          onClick={() => handleOpenEdit(cat)}
                          title='Edit'
                        >
                          <Edit2 className='size-4' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='size-8 text-destructive hover:bg-destructive/10'
                          onClick={() => handleOpenDelete(cat)}
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
              Showing page {pagination.page} of {pagination.total_pages} (Total {pagination.total_records} categories)
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

      <CategoryDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        category={selectedCategory}
      />

      <ConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title='Delete Category'
        desc={`Are you sure you want to delete category "${categoryToDelete?.category_name}"? This action will move it to trash.`}
        confirmText='Delete'
        destructive
        isLoading={deleteCategory.isPending}
        handleConfirm={handleConfirmDelete}
      />
    </>
  )
}
