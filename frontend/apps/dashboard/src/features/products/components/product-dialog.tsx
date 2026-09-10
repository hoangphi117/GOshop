import { useEffect, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
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

export function ProductDialog({ open, onOpenChange, product }: ProductDialogProps) {
  const isEdit = Boolean(product?.uuid)
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
    }
  }, [product, categories, form])

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
