import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useProducts } from '@/features/products/hooks/use-products'
import { useAdjustStock } from '../hooks/use-stock-movements'
import { Loader2 } from 'lucide-react'

interface AdjustStockDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  initialProductUUID?: string
}

export function AdjustStockDialog({
  open,
  onOpenChange,
  initialProductUUID,
}: AdjustStockDialogProps) {
  const [productUUID, setProductUUID] = useState(initialProductUUID || '')
  const [changeType, setChangeType] = useState<'RESTOCK' | 'MANUAL_ADJUSTMENT' | 'DAMAGED_DISCARD'>('RESTOCK')
  const [quantity, setQuantity] = useState<number>(10)
  const [isDeduct, setIsDeduct] = useState(false)
  const [note, setNote] = useState('')

  const { data: productsData } = useProducts({ limit: 100 })
  const products = (productsData?.data as unknown as Array<{ uuid: string; name: string; stock?: number; sku?: string }>) || []

  const adjustStockMutation = useAdjustStock()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!productUUID) return

    const qtyChange = isDeduct || changeType === 'DAMAGED_DISCARD' ? -Math.abs(quantity) : Math.abs(quantity)

    adjustStockMutation.mutate(
      {
        product_uuid: productUUID,
        quantity_change: qtyChange,
        change_type: changeType,
        note: note.trim() || undefined,
      },
      {
        onSuccess: () => {
          onOpenChange(false)
          setQuantity(10)
          setNote('')
        },
      }
    )
  }

  const handleTypeChange = (val: 'RESTOCK' | 'MANUAL_ADJUSTMENT' | 'DAMAGED_DISCARD') => {
    setChangeType(val)
    if (val === 'DAMAGED_DISCARD') {
      setIsDeduct(true)
    } else if (val === 'RESTOCK') {
      setIsDeduct(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[480px]'>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Adjust Product Stock</DialogTitle>
            <DialogDescription>
              Record an inventory adjustment, restocking event, or damaged loss write-off.
            </DialogDescription>
          </DialogHeader>

          <div className='grid gap-4 py-4'>
            {/* Product Select */}
            <div className='grid gap-2'>
              <Label htmlFor='product-select' className='text-xs font-semibold'>
                Product <span className='text-destructive'>*</span>
              </Label>
              <Select value={productUUID} onValueChange={setProductUUID}>
                <SelectTrigger id='product-select' className='text-xs h-9'>
                  <SelectValue placeholder='Select a product...' />
                </SelectTrigger>
                <SelectContent className='max-h-60'>
                  {products.map((p) => (
                    <SelectItem key={p.uuid} value={p.uuid} className='text-xs'>
                      {p.name} {p.sku ? `(${p.sku})` : ''} - Stock: {p.stock ?? 0}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Change Type */}
            <div className='grid gap-2'>
              <Label htmlFor='change-type' className='text-xs font-semibold'>
                Adjustment Reason <span className='text-destructive'>*</span>
              </Label>
              <Select
                value={changeType}
                onValueChange={(val) => handleTypeChange(val as 'RESTOCK' | 'MANUAL_ADJUSTMENT' | 'DAMAGED_DISCARD')}
              >
                <SelectTrigger id='change-type' className='text-xs h-9'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='RESTOCK' className='text-xs'>
                    Restock / Purchase Receipt (+)
                  </SelectItem>
                  <SelectItem value='MANUAL_ADJUSTMENT' className='text-xs'>
                    Manual Count Adjustment (+/-)
                  </SelectItem>
                  <SelectItem value='DAMAGED_DISCARD' className='text-xs'>
                    Damaged / Broken / Write-off (-)
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Operation Type (Add / Deduct) if Manual */}
            {changeType === 'MANUAL_ADJUSTMENT' && (
              <div className='grid gap-2'>
                <Label className='text-xs font-semibold'>Direction</Label>
                <div className='flex gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    variant={!isDeduct ? 'default' : 'outline'}
                    className='flex-1 text-xs'
                    onClick={() => setIsDeduct(false)}
                  >
                    + Add to Stock
                  </Button>
                  <Button
                    type='button'
                    size='sm'
                    variant={isDeduct ? 'destructive' : 'outline'}
                    className='flex-1 text-xs'
                    onClick={() => setIsDeduct(true)}
                  >
                    - Deduct from Stock
                  </Button>
                </div>
              </div>
            )}

            {/* Quantity */}
            <div className='grid gap-2'>
              <Label htmlFor='quantity' className='text-xs font-semibold'>
                Quantity <span className='text-destructive'>*</span>
              </Label>
              <Input
                id='quantity'
                type='number'
                min='1'
                value={quantity}
                onChange={(e) => setQuantity(Math.max(1, parseInt(e.target.value) || 1))}
                className='text-xs h-9'
                required
              />
            </div>

            {/* Audit Note */}
            <div className='grid gap-2'>
              <Label htmlFor='note' className='text-xs font-semibold'>
                Audit Note / Reason (Optional)
              </Label>
              <Textarea
                id='note'
                placeholder='e.g., Supplier PO #8829 received, shelf count audit...'
                value={note}
                onChange={(e) => setNote(e.target.value)}
                className='text-xs'
                rows={2}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={adjustStockMutation.isPending}
              onClick={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type='submit'
              size='sm'
              disabled={!productUUID || adjustStockMutation.isPending}
            >
              {adjustStockMutation.isPending ? (
                <Loader2 className='size-3.5 animate-spin mr-1.5' />
              ) : null}
              Save Adjustment
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
