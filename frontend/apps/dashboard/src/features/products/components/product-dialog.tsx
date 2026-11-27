import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { Plus, Trash2, Layers } from 'lucide-react'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { useCategories } from '@/features/categories/hooks/use-categories'
import { useProductMutations } from '../hooks/use-products'
import { ImageUpload } from './image-upload'
import type { Product, ProductFormData } from '../types'

const productSchema = z.object({
  name: z.string().min(2, 'Product name must be at least 2 characters').max(200),
  category_uuid: z.string().min(1, 'Please select a category'),
  price: z.number().min(0.01, 'Price must be at least 0.01'),
  stock: z.number().int().min(0, 'Stock must be non-negative'),
  sku: z.string().max(50).optional(),
  image_url: z.string().optional(),
  description: z.string().max(2000).optional(),
  status: z.union([z.literal(1), z.literal(2)]),
})

type ProductFormValues = z.infer<typeof productSchema>

interface ProductDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  product?: Product | null
}

interface AttributePair {
  key: string
  value: string
}

interface VariantDraft {
  id: string
  title: string
  sku: string
  price: number
  attributes: AttributePair[]
}

export function ProductDialog({ open, onOpenChange, product }: ProductDialogProps) {
  const isEdit = Boolean(product?.uuid)
  const [hasVariants, setHasVariants] = useState(false)
  const [variantDrafts, setVariantDrafts] = useState<VariantDraft[]>([])
  const { createProduct, updateProduct } = useProductMutations()
  const { data: categoriesData } = useCategories({ limit: 100 })
  const categories = useMemo(() => {
    return (categoriesData?.data as Array<{ uuid?: string; category_name?: string }>) || []
  }, [categoriesData?.data])

  const form = useForm<ProductFormValues>({
    resolver: zodResolver(productSchema),
    defaultValues: {
      name: '',
      category_uuid: '',
      price: 0,
      stock: 0,
      sku: '',
      image_url: '',
      description: '',
      status: 1,
    },
  })

  useEffect(() => {
    if (product) {
      form.reset({
        name: product.name || '',
        category_uuid: product.category_uuid || '',
        price: product.price ?? 0,
        stock: product.stock ?? 0,
        sku: product.sku || '',
        image_url: product.image_url || '',
        description: product.description || '',
        status:
          product.status?.toLowerCase() === 'active' || product.status === '1'
            ? 1
            : 2,
      })
      setHasVariants(false)
      setVariantDrafts([])
    } else {
      form.reset({
        name: '',
        category_uuid: categories[0]?.uuid || '',
        price: 0,
        stock: 0,
        sku: '',
        image_url: '',
        description: '',
        status: 1,
      })
      setHasVariants(false)
      setVariantDrafts([])
    }
  }, [product, categories, form])

  const handleAddVariant = () => {
    const parentPrice = form.getValues('price') || 0
    const parentName = form.getValues('name') || 'Item'
    const nextIdx = variantDrafts.length + 1
    const newDraft: VariantDraft = {
      id: `draft-${Date.now()}-${nextIdx}`,
      title: `${parentName} - Variant ${nextIdx}`,
      sku: `${form.getValues('sku') || 'SKU'}-V${nextIdx}`,
      price: parentPrice,
      attributes: [{ key: 'Color', value: '' }],
    }
    setVariantDrafts([...variantDrafts, newDraft])
  }

  const handleRemoveVariant = (id: string) => {
    setVariantDrafts(variantDrafts.filter((v) => v.id !== id))
  }

  const handleUpdateVariant = (id: string, field: 'title' | 'sku' | 'price', value: string | number) => {
    setVariantDrafts(
      variantDrafts.map((v) => (v.id === id ? { ...v, [field]: value } : v))
    )
  }

  const handleAddAttribute = (variantId: string) => {
    setVariantDrafts(
      variantDrafts.map((v) =>
        v.id === variantId
          ? { ...v, attributes: [...v.attributes, { key: '', value: '' }] }
          : v
      )
    )
  }

  const handleUpdateAttribute = (
    variantId: string,
    attrIdx: number,
    field: 'key' | 'value',
    val: string
  ) => {
    setVariantDrafts(
      variantDrafts.map((v) => {
        if (v.id !== variantId) return v
        const newAttrs = [...v.attributes]
        newAttrs[attrIdx] = { ...newAttrs[attrIdx], [field]: val }
        return { ...v, attributes: newAttrs }
      })
    )
  }

  const handleRemoveAttribute = (variantId: string, attrIdx: number) => {
    setVariantDrafts(
      variantDrafts.map((v) => {
        if (v.id !== variantId) return v
        return { ...v, attributes: v.attributes.filter((_, idx) => idx !== attrIdx) }
      })
    )
  }

  const onSubmit = async (values: ProductFormValues) => {
    try {
      const payload: ProductFormData = {
        name: values.name.trim(),
        category_uuid: values.category_uuid,
        price: Number(values.price),
        stock: Number(values.stock),
        sku: values.sku?.trim() || undefined,
        image_url: values.image_url?.trim() || undefined,
        description: values.description?.trim() || undefined,
        status: values.status,
      }

      if (isEdit && product?.uuid) {
        await updateProduct.mutateAsync({
          uuid: product.uuid,
          payload,
        })
        toast.success('Product updated successfully!')
      } else {
        if (hasVariants && variantDrafts.length > 0) {
          payload.variants = variantDrafts
            .filter((v) => v.sku.trim() !== '')
            .map((v, i) => ({
              sku: v.sku.trim(),
              title: v.title.trim() || v.sku.trim(),
              price: v.price > 0 ? v.price : Number(values.price),
              position: i,
              status: 1 as const,
              attributes: v.attributes.reduce<Record<string, string>>((acc, curr) => {
                if (curr.key.trim() && curr.value.trim()) {
                  acc[curr.key.trim()] = curr.value.trim()
                }
                return acc
              }, {}),
            }))
        }
        await createProduct.mutateAsync(payload)
        toast.success('Product created successfully!')
      }
      onOpenChange(false)
      form.reset()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Operation failed'
      toast.error(msg)
    }
  }

  const isSubmitting = createProduct.isPending || updateProduct.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[620px] max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Product' : 'Add Product'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update the details and pricing for this product.'
              : 'Fill in the fields below to add a new product to the store.'}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 py-2'>
            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem className='sm:col-span-2'>
                    <FormLabel>Product Name *</FormLabel>
                    <FormControl>
                      <Input placeholder='e.g., iPhone 16 Pro Max 256GB' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='category_uuid'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Category *</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder='Select category' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {categories.map((c) => (
                          <SelectItem key={c.uuid} value={c.uuid || ''}>
                            {c.category_name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='sku'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>SKU</FormLabel>
                    <FormControl>
                      <Input placeholder='e.g., IP16-256-BLK' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='price'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Price (VND) *</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder='0'
                        {...field}
                        onChange={(e) =>
                          field.onChange(e.target.value === '' ? 0 : Number(e.target.value))
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='stock'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Initial Stock *</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder='0'
                        {...field}
                        onChange={(e) =>
                          field.onChange(e.target.value === '' ? 0 : Number(e.target.value))
                        }
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
                  <FormItem className='sm:col-span-2'>
                    <FormLabel>Status</FormLabel>
                    <Select
                      onValueChange={(val) => field.onChange(Number(val) as 1 | 2)}
                      value={String(field.value)}
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

              <FormField
                control={form.control}
                name='image_url'
                render={({ field }) => (
                  <FormItem className='sm:col-span-2'>
                    <FormLabel>Product Image</FormLabel>
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

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem className='sm:col-span-2'>
                    <FormLabel>Description</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder='Highlight key features, technical specs, etc...'
                        rows={3}
                        value={field.value || ''}
                        onChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            {/* Product Variants Section (Creation only) */}
            {!isEdit && (
              <div className='rounded-lg border bg-muted/20 p-4 space-y-3'>
                <div className='flex items-center justify-between'>
                  <div className='flex items-center gap-2'>
                    <Checkbox
                      id='has-variants-toggle'
                      checked={hasVariants}
                      onCheckedChange={(checked) => {
                        const val = Boolean(checked)
                        setHasVariants(val)
                        if (val && variantDrafts.length === 0) {
                          handleAddVariant()
                        }
                      }}
                    />
                    <label
                      htmlFor='has-variants-toggle'
                      className='text-sm font-semibold cursor-pointer flex items-center gap-1.5'
                    >
                      <Layers className='h-4 w-4 text-primary' />
                      This product has multiple variants (Size, Color, etc.)
                    </label>
                  </div>
                  {hasVariants && (
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={handleAddVariant}
                      className='h-8 gap-1 text-xs'
                    >
                      <Plus className='h-3.5 w-3.5' />
                      Add Variant
                    </Button>
                  )}
                </div>

                {hasVariants && (
                  <div className='space-y-3 pt-2'>
                    {variantDrafts.map((draft, idx) => (
                      <div
                        key={draft.id}
                        className='relative rounded-md border bg-card p-3 shadow-xs space-y-3'
                      >
                        <div className='flex items-center justify-between border-b pb-2'>
                          <span className='text-xs font-semibold text-muted-foreground'>
                            Variant #{idx + 1}
                          </span>
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            onClick={() => handleRemoveVariant(draft.id)}
                            className='h-6 w-6 p-0 text-destructive hover:bg-destructive/10'
                          >
                            <Trash2 className='h-3.5 w-3.5' />
                          </Button>
                        </div>

                        <div className='grid grid-cols-1 sm:grid-cols-3 gap-2.5'>
                          <div>
                            <label className='text-xs font-medium'>Variant Title</label>
                            <Input
                              placeholder='e.g., Black / XL'
                              value={draft.title}
                              onChange={(e) => handleUpdateVariant(draft.id, 'title', e.target.value)}
                              className='h-8 text-xs'
                            />
                          </div>
                          <div>
                            <label className='text-xs font-medium'>Variant SKU *</label>
                            <Input
                              placeholder='e.g., TSHIRT-BLK-XL'
                              value={draft.sku}
                              onChange={(e) => handleUpdateVariant(draft.id, 'sku', e.target.value)}
                              className='h-8 text-xs'
                            />
                          </div>
                          <div>
                            <label className='text-xs font-medium'>Price (VND)</label>
                            <Input
                              type='number'
                              min={0}
                              placeholder='0'
                              value={draft.price}
                              onChange={(e) =>
                                handleUpdateVariant(
                                  draft.id,
                                  'price',
                                  e.target.value === '' ? 0 : Number(e.target.value)
                                )
                              }
                              className='h-8 text-xs'
                            />
                          </div>
                        </div>

                        {/* Variant Attributes (Key/Value) */}
                        <div className='space-y-1.5 pt-1'>
                          <div className='flex items-center justify-between'>
                            <span className='text-[11px] font-medium text-muted-foreground'>
                              Attributes (Options)
                            </span>
                            <button
                              type='button'
                              onClick={() => handleAddAttribute(draft.id)}
                              className='text-[11px] text-primary hover:underline flex items-center gap-0.5'
                            >
                              <Plus className='h-3 w-3' /> Add Option
                            </button>
                          </div>
                          <div className='grid grid-cols-1 sm:grid-cols-2 gap-2'>
                            {draft.attributes.map((attr, aIdx) => (
                              <div key={aIdx} className='flex items-center gap-1.5'>
                                <Input
                                  placeholder='Attribute (e.g., Size)'
                                  value={attr.key}
                                  onChange={(e) =>
                                    handleUpdateAttribute(draft.id, aIdx, 'key', e.target.value)
                                  }
                                  className='h-7 text-xs w-28 shrink-0'
                                />
                                <Input
                                  placeholder='Value (e.g., XL)'
                                  value={attr.value}
                                  onChange={(e) =>
                                    handleUpdateAttribute(draft.id, aIdx, 'value', e.target.value)
                                  }
                                  className='h-7 text-xs flex-1'
                                />
                                {draft.attributes.length > 1 && (
                                  <button
                                    type='button'
                                    onClick={() => handleRemoveAttribute(draft.id, aIdx)}
                                    className='text-muted-foreground hover:text-destructive'
                                  >
                                    <Trash2 className='h-3 w-3' />
                                  </button>
                                )}
                              </div>
                            ))}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            <DialogFooter className='pt-4'>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
                disabled={isSubmitting}
              >
                Cancel
              </Button>
              <Button type='submit' disabled={isSubmitting}>
                {isSubmitting ? 'Saving...' : isEdit ? 'Save Changes' : 'Create Product'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
