import { useCallback, useMemo, useState } from 'react'
import { CheckCircle2, EyeOff, Loader2, Trash2, X } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

export interface BatchResult {
  succeeded: number
  failed: number
}

/** Shows a consistent English summary toast for bulk operations. */
export function notifyBatchResult(actionLabel: string, result: BatchResult) {
  if (result.succeeded > 0 && result.failed === 0) {
    toast.success(`${actionLabel} ${result.succeeded} item(s)`)
  } else if (result.succeeded > 0 && result.failed > 0) {
    toast.warning(
      `${actionLabel}: ${result.succeeded} succeeded, ${result.failed} failed`
    )
  } else if (result.failed > 0) {
    toast.error(`Failed to ${actionLabel.toLowerCase()} ${result.failed} item(s)`)
  }
}

/** Manages row UUID selection across paginated catalog tables. */
export function useRowSelection<T extends { uuid?: string }>(items: T[]) {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set())

  const itemIds = useMemo(
    () => items.map((i) => i.uuid).filter((id): id is string => Boolean(id)),
    [items]
  )

  const selectedItems = useMemo(
    () => items.filter((i) => i.uuid && selectedIds.has(i.uuid)),
    [items, selectedIds]
  )

  const allSelected =
    itemIds.length > 0 && itemIds.every((id) => selectedIds.has(id))
  const someSelected =
    !allSelected && itemIds.some((id) => selectedIds.has(id))

  const toggleOne = useCallback((uuid: string, checked: boolean) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (checked) next.add(uuid)
      else next.delete(uuid)
      return next
    })
  }, [])

  const toggleAll = useCallback(
    (checked: boolean) => {
      setSelectedIds((prev) => {
        const next = new Set(prev)
        for (const id of itemIds) {
          if (checked) next.add(id)
          else next.delete(id)
        }
        return next
      })
    },
    [itemIds]
  )

  const clear = useCallback(() => setSelectedIds(new Set()), [])

  return {
    selectedIds,
    selectedItems,
    selectedCount: selectedItems.length,
    allSelected,
    someSelected,
    toggleOne,
    toggleAll,
    clear,
  }
}

/** Runs an async mutation across all items concurrently and tallies outcomes. */
export async function runBatch<T>(
  items: T[],
  fn: (item: T) => Promise<unknown>
): Promise<BatchResult> {
  const results = await Promise.allSettled(items.map(fn))
  let succeeded = 0
  let failed = 0
  for (const res of results) {
    if (res.status === 'fulfilled') {
      succeeded++
    } else {
      failed++
    }
  }
  return { succeeded, failed }
}

interface CatalogBulkBarProps {
  selectedCount: number
  entityLabel: string
  isPending: boolean
  onActivate: () => void
  onDeactivate: () => void
  onDelete: () => void
  onClear: () => void
}

/** Floating/inline action bar for bulk status updates and deletion in catalog tables. */
export function CatalogBulkBar({
  selectedCount,
  entityLabel,
  isPending,
  onActivate,
  onDeactivate,
  onDelete,
  onClear,
}: CatalogBulkBarProps) {
  if (selectedCount === 0) return null

  return (
    <div className='flex flex-wrap items-center justify-between gap-3 rounded-lg border border-primary/25 bg-primary/5 px-4 py-2.5'>
      <div className='flex items-center gap-2 text-sm font-medium'>
        <Badge variant='default' className='font-semibold'>
          {selectedCount}
        </Badge>
        <span>
          {entityLabel} selected
        </span>
      </div>

      <div className='flex flex-wrap items-center gap-2'>
        <Button
          size='sm'
          variant='outline'
          disabled={isPending}
          onClick={onActivate}
          className='h-8 gap-1.5 bg-background'
        >
          {isPending ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <CheckCircle2 className='size-3.5 text-emerald-600' />
          )}
          Activate
        </Button>

        <Button
          size='sm'
          variant='outline'
          disabled={isPending}
          onClick={onDeactivate}
          className='h-8 gap-1.5 bg-background'
        >
          {isPending ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <EyeOff className='size-3.5 text-amber-600' />
          )}
          Set Inactive
        </Button>

        <Button
          size='sm'
          variant='destructive'
          disabled={isPending}
          onClick={onDelete}
          className='h-8 gap-1.5'
        >
          {isPending ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <Trash2 className='size-3.5' />
          )}
          Delete
        </Button>

        <Button
          size='sm'
          variant='ghost'
          disabled={isPending}
          onClick={onClear}
          className='h-8 gap-1 text-muted-foreground'
        >
          <X className='size-3.5' />
          Clear
        </Button>
      </div>
    </div>
  )
}
