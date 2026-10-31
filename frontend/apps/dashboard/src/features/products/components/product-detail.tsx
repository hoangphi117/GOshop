import { useEffect, useMemo } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  ArrowLeft,
  Save,
  RotateCcw,
  ExternalLink,
  Star,
  CheckCircle2,
  Clock,
  XCircle,
  AlertCircle,
  Loader2,
} from 'lucide-react'
import { format, parseISO } from 'date-fns'
import { toast } from 'sonner'
import { DashboardHeader } from '@/components/layout/dashboard-header'
import { Main } from '@/components/layout/main'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormDescription,
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
import { Skeleton } from '@/components/ui/skeleton'
import { useCategories } from '@/features/categories/hooks/use-categories'
import { useProduct, useProductMutations } from '../hooks/use-products'
import { ImageUpload } from './image-upload'
import type { ProductFormData } from '../types'

interface ProductDetailProps {
  uuid: string
}

const productFormSchema = z.object({
  name: z.string().min(2, 'Product name must be at least 2 characters').max(200),
  category_uuid: z.string().min(1, 'Please select a category'),
  price: z.number().min(0.01, 'Price must be at least 0.01'),
  stock: z.number().int().min(0, 'Stock must be non-negative'),
  sku: z.string().max(50).optional(),
  image_url: z.string().optional(),
  description: z.string().max(2000).optional(),
  status: z.union([z.literal(1), z.literal(2)]),
})

type ProductFormValues = z.infer<typeof productFormSchema>

function formatPrice(price?: number) {
  if (price === undefined || price === null) return '0 ₫'
  return new Intl.NumberFormat('vi-VN', {
    style: 'currency',
    currency: 'VND',
  }).format(price)
}

function formatDate(dateStr?: string) {
  if (!dateStr) return 'N/A'
  try {
    return format(parseISO(dateStr), 'MMM dd, yyyy · HH:mm')
  } catch {
    return dateStr
  }
}

export function ProductDetail({ uuid }: ProductDetailProps) {
  const navigate = useNavigate()
  const { data: product, isLoading, isError, error } = useProduct(uuid)
  const { updateProduct } = useProductMutations()
  const { data: categoriesData } = useCategories({ limit: 100 })

  const categories = useMemo(() => {
    return (categoriesData?.data as Array<{ uuid?: string; category_name?: string }>) || []
  }, [categoriesData?.data])

  const form = useForm<ProductFormValues>({
    resolver: zodResolver(productFormSchema),
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

  // Synchronize form when product loads
  useEffect(() => {
    if (product) {
      const rawStatus = String(product.status || '').toLowerCase()
      const statusValue = rawStatus === 'active' || rawStatus === '1' ? 1 : 2
      form.reset({
        name: product.name || '',
        category_uuid: product.category_uuid || '',
        price: product.price ?? 0,
        stock: product.stock ?? 0,
        sku: product.sku || '',
        image_url: product.image_url || '',
        description: product.description || '',
        status: statusValue,
      })
    }
  }, [product, form])

  // Watch fields for live preview
  const watchedPrice = form.watch('price')
  const watchedStock = form.watch('stock')
  const watchedStatus = form.watch('status')

  const statusBadge = useMemo(() => {
    if (watchedStock <= 0) {
      return {
        label: 'Out of Stock',
        className: 'bg-red-500/10 text-red-600 border-red-200/80 dark:border-red-900/50',
        icon: XCircle,
      }
    }
    if (watchedStatus === 1) {
      return {
        label: 'Active',
        className: 'bg-emerald-500/10 text-emerald-600 border-emerald-200/80 dark:border-emerald-900/50',
        icon: CheckCircle2,
      }
    }
    return {
      label: 'Inactive',
      className: 'bg-slate-500/10 text-slate-600 border-slate-200/80 dark:border-slate-800',
      icon: Clock,
    }
  }, [watchedStock, watchedStatus])

  const storefrontUrl = useMemo(() => {
    if (!product?.slug) return null
    return `http://localhost:3001/products/${product.slug}`
  }, [product?.slug])

  const handleReset = () => {
    if (product) {
      const rawStatus = String(product.status || '').toLowerCase()
      const statusValue = rawStatus === 'active' || rawStatus === '1' ? 1 : 2
      form.reset({
        name: product.name || '',
        category_uuid: product.category_uuid || '',
        price: product.price ?? 0,
        stock: product.stock ?? 0,
        sku: product.sku || '',
        image_url: product.image_url || '',
        description: product.description || '',
        status: statusValue,
      })
      toast.info('Form reverted to saved state')
    }
  }

  const onSubmit = async (values: ProductFormValues) => {
    if (!uuid) return
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

      await updateProduct.mutateAsync({
        uuid,
        payload,
      })
      toast.success('Product updated successfully!')
      form.reset(values) // mark as not dirty with new values
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to update product'
      toast.error(msg)
    }
  }

  if (isLoading) {
    return (
      <>
        <DashboardHeader fixed />
        <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
          <div className='flex items-center gap-3'>
            <Skeleton className='h-9 w-9 rounded-md' />
            <div className='space-y-1.5'>
              <Skeleton className='h-6 w-48' />
              <Skeleton className='h-4 w-32' />
            </div>
          </div>
          <div className='grid grid-cols-1 gap-6 lg:grid-cols-3'>
            <div className='space-y-6 lg:col-span-1'>
              <Skeleton className='aspect-square w-full rounded-xl' />
              <Skeleton className='h-32 w-full rounded-xl' />
            </div>
            <div className='space-y-6 lg:col-span-2'>
              <Skeleton className='h-64 w-full rounded-xl' />
              <Skeleton className='h-48 w-full rounded-xl' />
            </div>
          </div>
        </Main>
      </>
    )
  }

  if (isError || !product) {
    return (
      <>
        <DashboardHeader fixed />
        <Main className='flex flex-1 flex-col items-center justify-center gap-4 py-16 text-center'>
          <div className='flex h-12 w-12 items-center justify-center rounded-full bg-red-100 dark:bg-red-950/40 text-red-600 mb-2'>
            <AlertCircle className='h-6 w-6' />
          </div>
          <h2 className='text-lg font-semibold'>Product Not Found</h2>
          <p className='text-sm text-muted-foreground max-w-sm'>
            {error instanceof Error ? error.message : 'The requested product could not be found or has been removed.'}
          </p>
          <Button
            variant='outline'
            onClick={() => navigate({ to: '/products' })}
          >
            Back to Products
          </Button>
        </Main>
      </>
    )
  }

  const isSaving = updateProduct.isPending
  const isDirty = form.formState.isDirty

  return (
    <>
      <DashboardHeader fixed />

      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
            {/* Page Header */}
            <div className='flex flex-wrap items-center justify-between gap-3'>
              <div className='flex items-center gap-3 min-w-0'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='size-9 shrink-0'
                  onClick={() => navigate({ to: '/products' })}
                  title='Back to Products'
                >
                  <ArrowLeft className='size-4' />
                </Button>
                <div className='min-w-0'>
                  <div className='flex items-center gap-2.5 flex-wrap'>
                    <h1 className='text-2xl font-bold tracking-tight text-foreground truncate'>
                      {form.watch('name') || product.name || 'Untitled Product'}
                    </h1>
                    <Badge
                      variant='outline'
                      className={`text-[11px] font-medium shrink-0 flex items-center gap-1 py-0.5 px-2 ${statusBadge.className}`}
                    >
                      <statusBadge.icon className='h-3 w-3' />
                      {statusBadge.label}
                    </Badge>
                  </div>
                  <p className='text-xs text-muted-foreground font-mono mt-0.5'>
                    UUID: {product.uuid}
                  </p>
                </div>
              </div>

              {/* Action Buttons */}
              <div className='flex items-center gap-2 shrink-0'>
                {storefrontUrl && (
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='h-9 gap-1.5 text-xs'
                    asChild
                  >
                    <a href={storefrontUrl} target='_blank' rel='noreferrer'>
                      <ExternalLink className='h-3.5 w-3.5' />
                      <span className='hidden sm:inline'>View in Store</span>
                    </a>
                  </Button>
                )}

                {isDirty && (
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-9 gap-1.5 text-xs'
                    onClick={handleReset}
                    disabled={isSaving}
                  >
                    <RotateCcw className='h-3.5 w-3.5' />
                    <span>Discard</span>
                  </Button>
                )}

                <Button
                  type='submit'
                  size='sm'
                  className='h-9 gap-1.5 text-xs'
                  disabled={isSaving || !isDirty}
                >
                  {isSaving ? (
                    <Loader2 className='h-3.5 w-3.5 animate-spin' />
                  ) : (
                    <Save className='h-3.5 w-3.5' />
                  )}
                  <span>{isSaving ? 'Saving...' : 'Save Changes'}</span>
                </Button>
              </div>
            </div>

            {/* Form Content Grid */}
            <div className='grid grid-cols-1 gap-6 lg:grid-cols-3'>
              {/* Left Column: Media & Organization */}
              <div className='space-y-6 lg:col-span-1'>
                {/* Media Card */}
                <Card className='shadow-xs'>
                  <CardHeader className='pb-3'>
                    <CardTitle className='text-base font-semibold'>Product Image</CardTitle>
                    <CardDescription className='text-xs'>
                      Upload or drag & drop thumbnail image
                    </CardDescription>
                  </CardHeader>
                  <CardContent className='pt-0'>
                    <FormField
                      control={form.control}
                      name='image_url'
                      render={({ field }) => (
                        <FormItem>
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
                  </CardContent>
                </Card>

                {/* Status & Category */}
                <Card className='shadow-xs'>
                  <CardHeader className='pb-3'>
                    <CardTitle className='text-base font-semibold'>Status & Category</CardTitle>
                  </CardHeader>
                  <CardContent className='space-y-4 pt-0'>
                    {/* Status Select */}
                    <FormField
                      control={form.control}
                      name='status'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className='text-xs'>Status</FormLabel>
                          <Select
                            value={String(field.value)}
                            onValueChange={(val) => field.onChange(Number(val) as 1 | 2)}
                          >
                            <FormControl>
                              <SelectTrigger className='w-full'>
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

                    {/* Category Select */}
                    <FormField
                      control={form.control}
                      name='category_uuid'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className='text-xs'>Category</FormLabel>
                          <Select
                            value={field.value}
                            onValueChange={field.onChange}
                          >
                            <FormControl>
                              <SelectTrigger className='w-full'>
                                <SelectValue placeholder='Select category' />
                              </SelectTrigger>
                            </FormControl>
                            <SelectContent>
                              {categories.map((c) => (
                                <SelectItem key={c.uuid} value={c.uuid || ''}>
                                  {c.category_name || 'Unnamed Category'}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </CardContent>
                </Card>

                {/* Metadata & Stats */}
                <Card className='shadow-xs bg-muted/20'>
                  <CardHeader className='pb-3'>
                    <CardTitle className='text-sm font-semibold text-muted-foreground'>
                      Catalog Information
                    </CardTitle>
                  </CardHeader>
                  <CardContent className='space-y-3 pt-0 text-xs'>
                    <div>
                      <span className='text-muted-foreground'>Slug URL</span>
                      <p className='font-mono font-medium text-foreground break-all mt-0.5'>
                        {product.slug || '—'}
                      </p>
                    </div>
                    <div>
                      <span className='text-muted-foreground'>Created Date</span>
                      <p className='font-medium text-foreground mt-0.5'>
                        {formatDate(product.created_at)}
                      </p>
                    </div>
                    {product.rating !== undefined && product.rating > 0 && (
                      <div className='flex items-center justify-between pt-1 border-t'>
                        <span className='text-muted-foreground'>Rating</span>
                        <div className='flex items-center gap-1 font-semibold text-amber-500'>
                          <Star className='h-3.5 w-3.5 fill-current' />
                          <span>{product.rating.toFixed(1)}</span>
                          {product.reviews_count !== undefined && (
                            <span className='text-muted-foreground font-normal'>
                              ({product.reviews_count} reviews)
                            </span>
                          )}
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>

              {/* Right Column: Pricing, Inventory & Details */}
              <div className='space-y-6 lg:col-span-2'>
                {/* General Information Card */}
                <Card className='shadow-xs'>
                  <CardHeader className='pb-4'>
                    <CardTitle className='text-base font-semibold'>General Information</CardTitle>
                    <CardDescription className='text-xs'>
                      Update the primary details for this product
                    </CardDescription>
                  </CardHeader>
                  <CardContent className='space-y-4 pt-0'>
                    {/* Name Field */}
                    <FormField
                      control={form.control}
                      name='name'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className='text-xs'>Product Name</FormLabel>
                          <FormControl>
                            <Input placeholder='e.g. Classic Cotton T-Shirt' {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    {/* Pricing & Stock Row */}
                    <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
                      {/* Price Field */}
                      <FormField
                        control={form.control}
                        name='price'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel className='text-xs'>Price (VND)</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min='0'
                                step='1000'
                                placeholder='100000'
                                {...field}
                                onChange={(e) =>
                                  field.onChange(
                                    e.target.value === '' ? 0 : Number(e.target.value)
                                  )
                                }
                              />
                            </FormControl>
                            <FormDescription className='text-[11px] font-medium text-primary'>
                              Formatted: {formatPrice(watchedPrice)}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      {/* Stock Field */}
                      <FormField
                        control={form.control}
                        name='stock'
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel className='text-xs'>Stock Quantity</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min='0'
                                step='1'
                                placeholder='50'
                                {...field}
                                onChange={(e) =>
                                  field.onChange(
                                    e.target.value === '' ? 0 : Number(e.target.value)
                                  )
                                }
                              />
                            </FormControl>
                            <FormDescription className={`text-[11px] font-medium ${watchedStock <= 0 ? 'text-destructive' : 'text-muted-foreground'}`}>
                              {watchedStock <= 0 ? 'Out of stock warning' : `${watchedStock} units in inventory`}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>

                    {/* SKU Field */}
                    <FormField
                      control={form.control}
                      name='sku'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className='text-xs'>SKU (Stock Keeping Unit)</FormLabel>
                          <FormControl>
                            <Input
                              placeholder='e.g. TSHIRT-COTTON-001'
                              className='font-mono'
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </CardContent>
                </Card>

                {/* Description Card */}
                <Card className='shadow-xs'>
                  <CardHeader className='pb-3'>
                    <CardTitle className='text-base font-semibold'>Description</CardTitle>
                    <CardDescription className='text-xs'>
                      Full description and specifications of the product
                    </CardDescription>
                  </CardHeader>
                  <CardContent className='pt-0'>
                    <FormField
                      control={form.control}
                      name='description'
                      render={({ field }) => (
                        <FormItem>
                          <FormControl>
                            <Textarea
                              placeholder='Write a detailed product description...'
                              className='min-h-[180px] leading-relaxed resize-y'
                              {...field}
                            />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </CardContent>
                </Card>
              </div>
            </div>
          </form>
        </Form>
      </Main>
    </>
  )
}
