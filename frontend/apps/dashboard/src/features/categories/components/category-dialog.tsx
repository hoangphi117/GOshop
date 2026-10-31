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
import { Button } from '@/components/ui/button'
import type { Category } from '../types'
import { useCategoryMutations } from '../hooks/use-categories'

const categorySchema = z.object({
  category_name: z
    .string()
    .min(2, 'Category name must be at least 2 characters')
    .max(100, 'Category name must not exceed 100 characters'),
  description: z.string().max(500, 'Description must not exceed 500 characters').optional(),
  status: z.union([z.literal(1), z.literal(2)]),
})

type CategoryFormValues = z.infer<typeof categorySchema>

interface CategoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  category?: Category | null
}

export function CategoryDialog({ open, onOpenChange, category }: CategoryDialogProps) {
  const isEdit = Boolean(category?.uuid)
  const { createCategory, updateCategory } = useCategoryMutations()

  const form = useForm<CategoryFormValues>({
    resolver: zodResolver(categorySchema),
    defaultValues: {
      category_name: '',
      description: '',
      status: 1,
    },
  })

  useEffect(() => {
    if (category) {
      form.reset({
        category_name: category.category_name || '',
        description: category.description || '',
        status:
          category.status?.toLowerCase() === 'active' || category.status === '1'
            ? 1
            : 2,
      })
    } else {
      form.reset({
        category_name: '',
        description: '',
        status: 1,
      })
    }
  }, [category, form])

  const onSubmit = async (values: CategoryFormValues) => {
    try {
      if (isEdit && category?.uuid) {
        await updateCategory.mutateAsync({
          uuid: category.uuid,
          payload: {
            category_name: values.category_name,
            description: values.description || '',
            status: values.status,
          },
        })
        toast.success('Category updated successfully!')
      } else {
        await createCategory.mutateAsync({
          category_name: values.category_name,
          description: values.description || '',
          status: values.status,
        })
        toast.success('Category created successfully!')
      }
      onOpenChange(false)
      form.reset()
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Operation failed'
      toast.error(msg)
    }
  }

  const isSubmitting = createCategory.isPending || updateCategory.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[480px]'>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Category' : 'Create Category'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Update the details for this product category.'
              : 'Fill in the information below to create a new category.'}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4 py-2'>
            <FormField
              control={form.control}
              name='category_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Category Name *</FormLabel>
                  <FormControl>
                    <Input placeholder='e.g., Electronics, Fashion...' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Description</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder='Brief overview of this category...'
                      rows={3}
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
              name='status'
              render={({ field }) => (
                <FormItem>
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
                {isSubmitting ? 'Saving...' : isEdit ? 'Save Changes' : 'Create Category'}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
