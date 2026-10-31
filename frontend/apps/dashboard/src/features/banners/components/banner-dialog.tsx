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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { ImageUpload } from '@/features/products/components/image-upload'
import { useBannerMutations } from '../hooks/use-banners'
import type { Banner, BannerFormData, BannerPosition } from '../types'

const bannerSchema = z.object({
  title: z.string().min(2, 'Banner title must be at least 2 characters').max(255),
  position: z.enum([
    'home_main',
    'home_sub',
    'home_middle',
    'home_popup',
    'category_top',
    'sidebar',
    'common',
  ]),
  image_url: z.string().min(1, 'Please upload a banner image'),
  link_url: z.string().optional(),
  order: z.number().int().min(0),
  is_active: z.boolean(),
})

type BannerFormValues = z.infer<typeof bannerSchema>

interface BannerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  banner?: Banner | null
}

export function BannerDialog({ open, onOpenChange, banner }: BannerDialogProps) {
  const isEdit = Boolean(banner?.uuid)
  const { createBanner, updateBanner } = useBannerMutations()

  const form = useForm<BannerFormValues>({
    resolver: zodResolver(bannerSchema),
    defaultValues: {
      title: '',
      position: 'home_main',
      image_url: '',
      link_url: '',
      order: 0,
      is_active: true,
    },
  })

  useEffect(() => {
    if (banner) {
      form.reset({
        title: banner.title || '',
        position: (banner.position as BannerPosition) || 'home_main',
        image_url: banner.image_url || '',
        link_url: banner.link_url || '',
        order: banner.order ?? 0,
        is_active: banner.is_active ?? true,
      })
    } else {
      form.reset({
        title: '',
        position: 'home_main',
        image_url: '',
        link_url: '',
        order: 0,
        is_active: true,
      })
    }
  }, [banner, form])

  const onSubmit = async (values: BannerFormValues) => {
    try {
      const payload: BannerFormData = {
        title: values.title,
        position: values.position,
        image_url: values.image_url,
        link_url: values.link_url || '',
        order: Number(values.order),
        is_active: values.is_active,
      }

      if (isEdit && banner?.uuid) {
        await updateBanner.mutateAsync({
          uuid: banner.uuid,
          payload,
        })
        toast.success('Banner updated successfully!')
      } else {
        await createBanner.mutateAsync(payload)
        toast.success('Banner created successfully!')
      }
      onOpenChange(false)
      form.reset()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Operation failed'
      toast.error(msg)
    }
  }

  const isSubmitting = createBanner.isPending || updateBanner.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[560px] max-h-[90vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Banner' : 'Create Banner'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update banner image and navigation target.'
              : 'Upload banner image and configure storefront placement.'}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 py-2'>
            <FormField
              control={form.control}
              name='title'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Banner Title *</FormLabel>
                  <FormControl>
                    <Input placeholder='e.g. Summer Mega Sale 2026' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='grid grid-cols-1 sm:grid-cols-2 gap-4'>
              <FormField
                control={form.control}
                name='position'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Placement Position *</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue placeholder='Select position' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='home_main'>Home - Main Hero</SelectItem>
                        <SelectItem value='home_sub'>Home - Sub Banner</SelectItem>
                        <SelectItem value='home_middle'>Home - Middle Section</SelectItem>
                        <SelectItem value='home_popup'>Home - Pop-up</SelectItem>
                        <SelectItem value='category_top'>Category Top</SelectItem>
                        <SelectItem value='sidebar'>Sidebar</SelectItem>
                        <SelectItem value='common'>General</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='order'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Priority Order</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
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
            </div>

            <FormField
              control={form.control}
              name='link_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Target URL</FormLabel>
                  <FormControl>
                    <Input placeholder='https://... or /products/...' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='image_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Banner Image *</FormLabel>
                  <FormControl>
                    <ImageUpload
                      value={field.value}
                      onChange={field.onChange}
                      folder='banners'
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
                <FormItem className='flex items-center justify-between rounded-lg border p-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>Active Status</FormLabel>
                    <p className='text-xs text-muted-foreground'>
                      Enable to display banner on storefront immediately
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
                {isSubmitting ? 'Saving...' : isEdit ? 'Save Changes' : 'Create Banner'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
