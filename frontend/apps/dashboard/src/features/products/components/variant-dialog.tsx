import { useState, useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { Plus, Trash2, Loader2 } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { useAppDispatch } from '@/stores/hooks'
import { createVariant, updateVariant } from '../store/variant-slice'
import { ImageUpload } from './image-upload'
import type { ProductVariant, CreateVariantInput } from '../types'

const variantFormSchema = z.object({
  sku: z.string().min(1, 'SKU is required').max(100),
  title: z.string().min(1, 'Title is required').max(255),
  price: z.number().min(0.01, 'Price must be greater than 0'),
  compare_at_price: z.number().optional(),
  image_url: z.string().optional(),
  position: z.number().int().min(0),
  status: z.union([z.literal(1), z.literal(2)]),
})

type VariantFormValues = z.infer<typeof variantFormSchema>

interface VariantDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  productUuid: string
  variant?: ProductVariant | null
}

interface AttributePair {
  key: string
  value: string
}

export function VariantDialog({
  open,
  onOpenChange,
  productUuid,
  variant,
}: VariantDialogProps) {
  const dispatch = useAppDispatch()
  const isEdit = Boolean(variant)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [attributes, setAttributes] = useState<AttributePair[]>([])

  const form = useForm<VariantFormValues>({
    resolver: zodResolver(variantFormSchema),
    defaultValues: {
      sku: '',
      title: '',
      price: 0,
      compare_at_price: undefined,
      image_url: '',
      position: 0,
      status: 1,
    },
  })

  useEffect(() => {
    if (variant) {
      form.reset({
        sku: variant.sku,
        title: variant.title,
        price: variant.price,
        compare_at_price: variant.compare_at_price ?? undefined,
        image_url: variant.image_url || '',
        position: variant.position,
        status: variant.status === 'Active' ? 1 : 2,
      })

      const pairs = Object.entries(variant.attributes || {}).map(([key, value]) => ({
        key,
        value,
      }))
      setAttributes(pairs.length > 0 ? pairs : [{ key: '', value: '' }])
    } else {
      form.reset({
        sku: '',
        title: '',
        price: 0,
        compare_at_price: undefined,
        image_url: '',
        position: 0,
        status: 1,
      })
      setAttributes([{ key: '', value: '' }])
    }
  }, [variant, form, open])

  const addAttribute = () => {
    setAttributes((prev) => [...prev, { key: '', value: '' }])
  }

  const removeAttribute = (index: number) => {
    setAttributes((prev) => prev.filter((_, i) => i !== index))
  }

  const updateAttribute = (index: number, field: 'key' | 'value', val: string) => {
    setAttributes((prev) =>
      prev.map((item, i) => (i === index ? { ...item, [field]: val } : item))
    )
  }

  const onSubmit = async (values: VariantFormValues) => {
    setIsSubmitting(true)
    try {
      const attrMap: Record<string, string> = {}
      for (const pair of attributes) {
        if (pair.key.trim() && pair.value.trim()) {
          attrMap[pair.key.trim()] = pair.value.trim()
        }
      }

      const payload: CreateVariantInput = {
        sku: values.sku.trim(),
        title: values.title.trim(),
        price: Number(values.price),
        compare_at_price: values.compare_at_price ? Number(values.compare_at_price) : undefined,
        image_url: values.image_url?.trim() || undefined,
        attributes: attrMap,
        position: Number(values.position),
        status: values.status,
      }

      if (isEdit && variant) {
        await dispatch(
          updateVariant({
            productUuid,
            variantUuid: variant.uuid,
            data: payload,
          })
        ).unwrap()
        toast.success('Variant updated successfully')
      } else {
        await dispatch(
          createVariant({
            productUuid,
            data: payload,
          })
        ).unwrap()
        toast.success('Variant created successfully')
      }
      onOpenChange(false)
    } catch (err) {
      toast.error(typeof err === 'string' ? err : 'Failed to save variant')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-xl'>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Product Variant' : 'Add New Variant'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update the SKU, pricing, attributes, or image for this variant.'
              : 'Add an SKU-level variant (e.g. Color, Size, Storage) to this product.'}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 py-2'>
            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='title'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Variant Title *</FormLabel>
                    <FormControl>
                      <Input placeholder='e.g. Black / 256GB' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='sku'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>SKU *</FormLabel>
                    <FormControl>
                      <Input placeholder='e.g. PROD-BLK-256' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='price'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Price (VND) *</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        step='1000'
                        placeholder='150000'
                        {...field}
                        onChange={(e) => field.onChange(parseFloat(e.target.value) || 0)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />


            </div>

            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='position'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Display Position</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        placeholder='0'
                        {...field}
                        onChange={(e) => field.onChange(parseInt(e.target.value, 10) || 0)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='status'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Status</FormLabel>
                    <Select
                      value={String(field.value)}
                      onValueChange={(val) => field.onChange(parseInt(val, 10))}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder='Select status' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='1'>Active</SelectItem>
                        <SelectItem value='2'>Inactive</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            {/* Custom Attributes Section */}
            <div className='space-y-2 pt-2 border-t'>
              <div className='flex items-center justify-between'>
                <FormLabel className='text-sm font-semibold'>Attributes (Option Values)</FormLabel>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={addAttribute}
                  className='h-7 text-xs gap-1'
                >
                  <Plus className='h-3 w-3' />
                  Add Option
                </Button>
              </div>
              <p className='text-xs text-muted-foreground'>
                Key-value pairs representing options like Color, Size, Storage.
              </p>

              <div className='space-y-2'>
                {attributes.map((attr, index) => (
                  <div key={index} className='flex items-center gap-2'>
                    <Input
                      placeholder='Option Name (e.g. Color)'
                      value={attr.key}
                      onChange={(e) => updateAttribute(index, 'key', e.target.value)}
                      className='h-8 text-xs flex-1'
                    />
                    <Input
                      placeholder='Option Value (e.g. Midnight Blue)'
                      value={attr.value}
                      onChange={(e) => updateAttribute(index, 'value', e.target.value)}
                      className='h-8 text-xs flex-1'
                    />
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='h-8 w-8 text-muted-foreground hover:text-red-500'
                      onClick={() => removeAttribute(index)}
                      disabled={attributes.length === 1 && !attr.key && !attr.value}
                    >
                      <Trash2 className='h-3.5 w-3.5' />
                    </Button>
                  </div>
                ))}
              </div>
            </div>

            {/* Variant Image */}
            <FormField
              control={form.control}
              name='image_url'
              render={({ field }) => (
                <FormItem className='pt-2 border-t'>
                  <FormLabel>Variant Image</FormLabel>
                  <FormControl>
                    <ImageUpload
                      value={field.value}
                      onChange={field.onChange}
                      folder='products'
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter className='pt-4 border-t'>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={isSubmitting}
              >
                Cancel
              </Button>
              <Button type='submit' disabled={isSubmitting}>
                {isSubmitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
                {isEdit ? 'Save Changes' : 'Create Variant'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
