import { useState, useEffect } from 'react'
import { Plus, Edit, Trash2, Layers, AlertCircle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { useAppDispatch, useAppSelector } from '@/stores/hooks'
import {
  fetchVariants,
  deleteVariant,
  setSelectedVariant,
} from '../store/variant-slice'
import { VariantDialog } from './variant-dialog'
import type { ProductVariant } from '../types'

interface VariantManagerProps {
  productUuid: string
}

function formatPrice(price?: number) {
  if (price === undefined || price === null) return '0 ₫'
  return new Intl.NumberFormat('vi-VN', {
    style: 'currency',
    currency: 'VND',
  }).format(price)
}

export function VariantManager({ productUuid }: VariantManagerProps) {
  const dispatch = useAppDispatch()
  const { variants, isLoading, error, selectedVariant } = useAppSelector(
    (state) => state.variants
  )

  const [dialogOpen, setDialogOpen] = useState(false)
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [variantToDelete, setVariantToDelete] = useState<ProductVariant | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  useEffect(() => {
    if (productUuid) {
      dispatch(fetchVariants(productUuid))
    }
  }, [productUuid, dispatch])

  const handleOpenCreate = () => {
    dispatch(setSelectedVariant(null))
    setDialogOpen(true)
  }

  const handleOpenEdit = (v: ProductVariant) => {
    dispatch(setSelectedVariant(v))
    setDialogOpen(true)
  }

  const handleOpenDelete = (v: ProductVariant) => {
    setVariantToDelete(v)
    setDeleteDialogOpen(true)
  }

  const handleConfirmDelete = async () => {
    if (!variantToDelete) return
    setIsDeleting(true)
    try {
      await dispatch(
        deleteVariant({
          productUuid,
          variantUuid: variantToDelete.uuid,
        })
      ).unwrap()
      toast.success('Variant removed successfully')
      setDeleteDialogOpen(false)
    } catch (err) {
      toast.error(typeof err === 'string' ? err : 'Failed to delete variant')
    } finally {
      setIsDeleting(false)
      setVariantToDelete(null)
    }
  }

  return (
    <Card className='shadow-xs'>
      <CardHeader className='flex flex-row items-center justify-between pb-3'>
        <div className='space-y-0.5'>
          <CardTitle className='text-base font-semibold flex items-center gap-2'>
            <Layers className='h-4 w-4 text-primary' />
            Product Variants (SKUs)
          </CardTitle>
          <CardDescription className='text-xs'>
            Manage options, individual SKUs, prices, and imagery for this product.
          </CardDescription>
        </div>
        <Button size='sm' onClick={handleOpenCreate} className='gap-1.5 h-8 text-xs'>
          <Plus className='h-3.5 w-3.5' />
          Add Variant
        </Button>
      </CardHeader>

      <CardContent>
        {error && (
          <div className='mb-4 flex items-center gap-2 rounded-md bg-red-50 p-3 text-xs text-red-600 dark:bg-red-950/40'>
            <AlertCircle className='h-4 w-4 shrink-0' />
            <span>{error}</span>
          </div>
        )}

        {isLoading && variants.length === 0 ? (
          <div className='flex items-center justify-center py-10 text-muted-foreground'>
            <Loader2 className='h-5 w-5 animate-spin mr-2' />
            <span className='text-sm'>Loading variants...</span>
          </div>
        ) : variants.length === 0 ? (
          <div className='flex flex-col items-center justify-center py-12 text-center'>
            <div className='flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-3 text-muted-foreground'>
              <Layers className='h-6 w-6' />
            </div>
            <h3 className='text-sm font-semibold mb-1'>No variants created yet</h3>
            <p className='text-xs text-muted-foreground max-w-sm mb-4'>
              This product currently uses single-item stock. Add variants to allow customers to choose by size, color, or storage options.
            </p>
            <Button size='sm' variant='outline' onClick={handleOpenCreate} className='gap-1.5 text-xs'>
              <Plus className='h-3.5 w-3.5' />
              Add First Variant
            </Button>
          </div>
        ) : (
          <div className='rounded-md border overflow-hidden'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-[60px]'>Image</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>SKU</TableHead>
                  <TableHead>Attributes</TableHead>
                  <TableHead className='text-right'>Price</TableHead>
                  <TableHead className='w-[80px] text-center'>Status</TableHead>
                  <TableHead className='w-[90px] text-right'>Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {variants.map((v) => (
                  <TableRow key={v.uuid}>
                    <TableCell>
                      {v.image_url ? (
                        <img
                          src={v.image_url}
                          alt={v.title}
                          className='h-10 w-10 rounded-md object-cover border bg-muted'
                        />
                      ) : (
                        <div className='h-10 w-10 rounded-md bg-muted flex items-center justify-center text-[10px] text-muted-foreground border'>
                          No img
                        </div>
                      )}
                    </TableCell>
                    <TableCell className='font-medium text-xs'>
                      <div>{v.title}</div>
                      {v.position > 0 && (
                        <span className='text-[10px] text-muted-foreground'>
                          Pos: {v.position}
                        </span>
                      )}
                    </TableCell>
                    <TableCell className='text-xs font-mono text-muted-foreground'>
                      {v.sku}
                    </TableCell>
                    <TableCell>
                      <div className='flex flex-wrap gap-1'>
                        {Object.entries(v.attributes || {}).map(([key, val]) => (
                          <Badge
                            key={key}
                            variant='secondary'
                            className='text-[10px] py-0 px-1.5'
                          >
                            <span className='font-normal text-muted-foreground'>{key}:</span>{' '}
                            {val}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell className='text-right text-xs font-medium'>
                      <div>{formatPrice(v.price)}</div>
                    </TableCell>
                    <TableCell className='text-center'>
                      <Badge
                        variant='outline'
                        className={`text-[10px] py-0 px-1.5 ${
                          v.status === 'Active'
                            ? 'bg-emerald-500/10 text-emerald-600 border-emerald-200/80 dark:border-emerald-900/50'
                            : 'bg-slate-500/10 text-slate-600 border-slate-200/80 dark:border-slate-800'
                        }`}
                      >
                        {v.status}
                      </Badge>
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='h-7 w-7'
                          onClick={() => handleOpenEdit(v)}
                          title='Edit Variant'
                        >
                          <Edit className='h-3.5 w-3.5' />
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='h-7 w-7 text-muted-foreground hover:text-red-500'
                          onClick={() => handleOpenDelete(v)}
                          title='Delete Variant'
                        >
                          <Trash2 className='h-3.5 w-3.5' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <VariantDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        productUuid={productUuid}
        variant={selectedVariant}
      />

      <AlertDialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Variant</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to permanently delete variant &quot;{variantToDelete?.title}&quot; (SKU: {variantToDelete?.sku})? This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isDeleting}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmDelete}
              disabled={isDeleting}
              className='bg-red-600 hover:bg-red-700 text-white'
            >
              {isDeleting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
