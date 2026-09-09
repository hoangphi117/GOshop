import {
  ChevronLeftIcon,
  ChevronRightIcon,
  DoubleArrowLeftIcon,
  DoubleArrowRightIcon,
} from '@radix-ui/react-icons'
import { type Table } from '@tanstack/react-table'
import { cn, getPageNumbers } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

type DataTablePaginationProps<TData> = {
  className?: string
} & (
  | {
      table: Table<TData>
      page?: never
      pageSize?: never
      totalPages?: never
      onPageChange?: never
      onPageSizeChange?: never
    }
  | {
      table?: never
      page: number
      pageSize: number
      totalPages: number
      onPageChange: (page: number) => void
      onPageSizeChange?: (pageSize: number) => void
    }
)

export function DataTablePagination<TData>({
  table,
  className,
  page,
  pageSize,
  totalPages: propTotalPages,
  onPageChange,
  onPageSizeChange,
}: DataTablePaginationProps<TData>) {
  const isTable = Boolean(table)

  const currentPage = isTable
    ? table!.getState().pagination.pageIndex + 1
    : (page ?? 1)
  const totalPages = isTable
    ? table!.getPageCount()
    : Math.max(1, propTotalPages ?? 1)
  const currentPageSize = isTable
    ? table!.getState().pagination.pageSize
    : (pageSize ?? 10)

  const canPrevious = isTable ? table!.getCanPreviousPage() : currentPage > 1
  const canNext = isTable ? table!.getCanNextPage() : currentPage < totalPages

  const handleSetPage = (targetPage: number) => {
    if (isTable) {
      table!.setPageIndex(targetPage - 1)
    } else {
      onPageChange?.(targetPage)
    }
  }

  const handleSetPageSize = (size: number) => {
    if (isTable) {
      table!.setPageSize(size)
    } else {
      onPageSizeChange?.(size)
    }
  }

  const handleFirstPage = () => {
    if (isTable) {
      table!.setPageIndex(0)
    } else {
      handleSetPage(1)
    }
  }

  const handlePreviousPage = () => {
    if (isTable) {
      table!.previousPage()
    } else {
      handleSetPage(Math.max(1, currentPage - 1))
    }
  }

  const handlePageClick = (p: number) => {
    if (isTable) {
      table!.setPageIndex(p - 1)
    } else {
      handleSetPage(p)
    }
  }

  const handleNextPage = () => {
    if (isTable) {
      table!.nextPage()
    } else {
      handleSetPage(Math.min(totalPages, currentPage + 1))
    }
  }

  const handleLastPage = () => {
    if (isTable) {
      table!.setPageIndex(table!.getPageCount() - 1)
    } else {
      handleSetPage(totalPages)
    }
  }

  const pageNumbers = getPageNumbers(currentPage, totalPages)

  return (
    <div
      className={cn(
        'flex items-center justify-between overflow-clip px-2',
        '@max-2xl/content:flex-col-reverse @max-2xl/content:gap-4',
        className
      )}
      style={{ overflowClipMargin: 1 }}
    >
      <div className='flex w-full items-center justify-between'>
        <div className='flex w-25 items-center justify-center text-sm font-medium @2xl/content:hidden'>
          Page {currentPage} of {totalPages}
        </div>
        <div className='flex items-center gap-2 @max-2xl/content:flex-row-reverse'>
          <Select
            value={`${currentPageSize}`}
            onValueChange={(value) => {
              handleSetPageSize(Number(value))
            }}
          >
            <SelectTrigger className='h-8 w-17.5'>
              <SelectValue placeholder={currentPageSize} />
            </SelectTrigger>
            <SelectContent side='top'>
              {[10, 20, 30, 40, 50].map((size) => (
                <SelectItem key={size} value={`${size}`}>
                  {size}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className='hidden text-sm font-medium sm:block'>Rows per page</p>
        </div>
      </div>

      <div className='flex items-center sm:space-x-6 lg:space-x-8'>
        <div className='flex w-25 items-center justify-center text-sm font-medium @max-3xl/content:hidden'>
          Page {currentPage} of {totalPages}
        </div>
        <div className='flex items-center space-x-2'>
          <Button
            variant='outline'
            className='size-8 p-0 @max-md/content:hidden'
            onClick={handleFirstPage}
            disabled={!canPrevious}
          >
            <span className='sr-only'>Go to first page</span>
            <DoubleArrowLeftIcon className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            className='size-8 p-0'
            onClick={handlePreviousPage}
            disabled={!canPrevious}
          >
            <span className='sr-only'>Go to previous page</span>
            <ChevronLeftIcon className='h-4 w-4' />
          </Button>

          {/* Page number buttons */}
          {pageNumbers.map((pageNumber, index) => (
            <div key={`${pageNumber}-${index}`} className='flex items-center'>
              {pageNumber === '...' ? (
                <span className='px-1 text-sm text-muted-foreground'>...</span>
              ) : (
                <Button
                  variant={currentPage === pageNumber ? 'default' : 'outline'}
                  className='h-8 min-w-8 px-2'
                  onClick={() => handlePageClick(pageNumber as number)}
                >
                  <span className='sr-only'>Go to page {pageNumber}</span>
                  {pageNumber}
                </Button>
              )}
            </div>
          ))}

          <Button
            variant='outline'
            className='size-8 p-0'
            onClick={handleNextPage}
            disabled={!canNext}
          >
            <span className='sr-only'>Go to next page</span>
            <ChevronRightIcon className='h-4 w-4' />
          </Button>
          <Button
            variant='outline'
            className='size-8 p-0 @max-md/content:hidden'
            onClick={handleLastPage}
            disabled={!canNext}
          >
            <span className='sr-only'>Go to last page</span>
            <DoubleArrowRightIcon className='h-4 w-4' />
          </Button>
        </div>
      </div>
    </div>
  )
}
