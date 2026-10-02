import { useEffect, useState } from 'react'
import { Cross2Icon } from '@radix-ui/react-icons'
import { type Table } from '@tanstack/react-table'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { DataTableFacetedFilter } from './faceted-filter'
import { DataTableViewOptions } from './view-options'

type DataTableToolbarProps<TData> = {
  table: Table<TData>
  searchPlaceholder?: string
  searchKey?: string
  debounceMs?: number
  filters?: {
    columnId: string
    title: string
    options: {
      label: string
      value: string
      icon?: React.ComponentType<{ className?: string }>
    }[]
  }[]
}

export function DataTableToolbar<TData>({
  table,
  searchPlaceholder = 'Filter...',
  searchKey,
  debounceMs = 350,
  filters = [],
}: DataTableToolbarProps<TData>) {
  const isFiltered =
    table.getState().columnFilters.length > 0 || Boolean(table.getState().globalFilter)

  const externalValue = searchKey
    ? ((table.getColumn(searchKey)?.getFilterValue() as string) ?? '')
    : ((table.getState().globalFilter as string) ?? '')

  const [searchValue, setSearchValue] = useState(externalValue)

  // Sync with external state changes (e.g., reset button or URL updates)
  useEffect(() => {
    setSearchValue(externalValue)
  }, [externalValue])

  // Debounce pushing value to table state
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchValue !== externalValue) {
        if (searchKey) {
          table.getColumn(searchKey)?.setFilterValue(searchValue || undefined)
        } else {
          table.setGlobalFilter(searchValue || undefined)
        }
      }
    }, debounceMs)

    return () => clearTimeout(timer)
  }, [searchValue, externalValue, searchKey, table, debounceMs])

  const handleReset = () => {
    setSearchValue('')
    table.resetColumnFilters()
    table.setGlobalFilter('')
  }

  return (
    <div className='flex items-center justify-between'>
      <div className='flex flex-1 flex-col-reverse items-start gap-y-2 sm:flex-row sm:items-center sm:space-x-2'>
        <Input
          placeholder={searchPlaceholder}
          value={searchValue}
          onChange={(event) => setSearchValue(event.target.value)}
          className='h-8 w-37.5 lg:w-62.5'
        />
        <div className='flex gap-x-2'>
          {filters.map((filter) => {
            const column = table.getColumn(filter.columnId)
            if (!column) return null
            return (
              <DataTableFacetedFilter
                key={filter.columnId}
                column={column}
                title={filter.title}
                options={filter.options}
              />
            )
          })}
        </div>
        {isFiltered && (
          <Button
            variant='ghost'
            onClick={handleReset}
            className='h-8 px-2 lg:px-3'
          >
            Reset
            <Cross2Icon className='ms-2 h-4 w-4' />
          </Button>
        )}
      </div>
      <DataTableViewOptions table={table} />
    </div>
  )
}

