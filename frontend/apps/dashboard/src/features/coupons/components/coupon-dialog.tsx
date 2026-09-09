import { useEffect } from 'react'
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
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { useCouponMutations } from '../hooks/use-coupons'
import type { Coupon, CouponFormData } from '../types'

const couponSchema = z.object({
  code: z
    .string()
    .min(3, 'Coupon code must be at least 3 characters')
    .max(50, 'Coupon code cannot exceed 50 characters'),
  name: z.string().min(2, 'Campaign name must be at least 2 characters').max(100),
  description: z.string().max(500).optional(),
  type: z.union([z.literal(1), z.literal(2)]),
  discount_value: z.number().min(1, 'Discount value must be greater than 0'),
  min_order_amount: z.number().min(0).optional(),
  max_discount_amount: z.number().min(0).optional(),
  usage_limit: z.number().int().min(1).optional(),
  start_date: z.string().min(1, 'Please select start date'),
  end_date: z.string().min(1, 'Please select end date'),
  is_active: z.boolean(),
})

type CouponFormValues = z.infer<typeof couponSchema>

interface CouponDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  coupon?: Coupon | null
}

export function CouponDialog({ open, onOpenChange, coupon }: CouponDialogProps) {
  const isEdit = Boolean(coupon?.uuid)
  const { createCoupon, updateCoupon } = useCouponMutations()

  const form = useForm<CouponFormValues>({
    resolver: zodResolver(couponSchema),
    defaultValues: {
      code: '',
      name: '',
      description: '',
      type: 1,
      discount_value: 10,
      min_order_amount: 0,
      max_discount_amount: 0,
      usage_limit: 100,
      start_date: '',
      end_date: '',
      is_active: true,
    },
  })

  useEffect(() => {
    const now = new Date()
    const today = now.toISOString().split('T')[0]
    const in30Days = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000)
      .toISOString()
      .split('T')[0]

    if (coupon) {
      form.reset({
        code: coupon.code || '',
        name: coupon.name || '',
        description: coupon.description || '',
        type: (coupon.type as 1 | 2) || 1,
        discount_value: coupon.discount_value ?? 10,
        min_order_amount: coupon.min_order_amount ?? 0,
        max_discount_amount: coupon.max_discount_amount ?? 0,
        usage_limit: coupon.usage_limit ?? 100,
        start_date: coupon.start_date
          ? new Date(coupon.start_date).toISOString().split('T')[0]
          : today,
        end_date: coupon.end_date
          ? new Date(coupon.end_date).toISOString().split('T')[0]
          : in30Days,
        is_active: coupon.is_active ?? true,
      })
    } else {
      form.reset({
        code: '',
        name: '',
        description: '',
        type: 1,
        discount_value: 10,
        min_order_amount: 0,
        max_discount_amount: 0,
        usage_limit: 100,
        start_date: today,
        end_date: in30Days,
        is_active: true,
      })
    }
  }, [coupon, form])

  const onSubmit = async (values: CouponFormValues) => {
    try {
      const payload: CouponFormData = {
        code: values.code.toUpperCase().trim(),
        name: values.name,
        description: values.description || '',
        type: values.type,
        discount_value: Number(values.discount_value),
        min_order_amount: values.min_order_amount ? Number(values.min_order_amount) : undefined,
        max_discount_amount: values.max_discount_amount ? Number(values.max_discount_amount) : undefined,
        usage_limit: values.usage_limit ? Number(values.usage_limit) : undefined,
        start_date: new Date(values.start_date).toISOString(),
        end_date: new Date(values.end_date).toISOString(),
        is_active: values.is_active,
      }

      if (isEdit && coupon?.uuid) {
        await updateCoupon.mutateAsync({
          uuid: coupon.uuid,
          payload,
        })
        toast.success('Coupon updated successfully!')
      } else {
        await createCoupon.mutateAsync(payload)
        toast.success('Coupon created successfully!')
      }
      onOpenChange(false)
      form.reset()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Operation failed'
      toast.error(msg)
    }
  }

  const isSubmitting = createCoupon.isPending || updateCoupon.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[560px] max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Coupon' : 'Create Coupon'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update discount value, rules, and validity period.'
              : 'Create promotional coupon code for customers.'}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 py-2'>
            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='code'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Coupon Code *</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='e.g. GOSHOP10, SUMMER2026'
                        className='uppercase font-mono'
                        disabled={isEdit}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Campaign Name *</FormLabel>
                    <FormControl>
                      <Input placeholder='e.g. Summer Promo 2026' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Discount Type *</FormLabel>
                    <Select
                      onValueChange={(val) => field.onChange(Number(val) as 1 | 2)}
                      value={String(field.value)}
                    >
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder='Select type' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='1'>Percentage (%)</SelectItem>
                        <SelectItem value='2'>Fixed Amount (VND)</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='discount_value'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Discount Value *</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
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
                name='min_order_amount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Min Order Amount (VND)</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder='0'
                        {...field}
                        onChange={(e) =>
                          field.onChange(
                            e.target.value === '' ? undefined : Number(e.target.value)
                          )
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='max_discount_amount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Max Discount (VND)</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        placeholder='0'
                        {...field}
                        onChange={(e) =>
                          field.onChange(
                            e.target.value === '' ? undefined : Number(e.target.value)
                          )
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='usage_limit'
                render={({ field }) => (
                  <FormItem className='sm:col-span-2'>
                    <FormLabel>Usage Limit</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        placeholder='100'
                        {...field}
                        onChange={(e) =>
                          field.onChange(
                            e.target.value === '' ? undefined : Number(e.target.value)
                          )
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='start_date'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Start Date *</FormLabel>
                    <FormControl>
                      <Input type='date' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='end_date'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>End Date *</FormLabel>
                    <FormControl>
                      <Input type='date' {...field} />
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
                    <FormLabel>Description & Terms</FormLabel>
                    <FormControl>
                      <Textarea
                        placeholder='Applies to orders above 200,000 VND...'
                        rows={2}
                        value={field.value || ''}
                        onChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='is_active'
                render={({ field }) => (
                  <FormItem className='sm:col-span-2 flex items-center justify-between rounded-lg border p-3'>
                    <div className='space-y-0.5'>
                      <FormLabel>Active Status</FormLabel>
                      <p className='text-xs text-muted-foreground'>
                        Allow customers to apply this coupon at checkout
                      </p>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
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
                {isSubmitting ? 'Saving...' : isEdit ? 'Save Changes' : 'Create Coupon'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
