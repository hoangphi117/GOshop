import { useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useNavigate } from '@tanstack/react-router'
import { Loader2, LogIn } from 'lucide-react'
import { toast } from 'sonner'
import { useAuth } from '@/features/auth/hooks/use-auth'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { PasswordInput } from '@/components/password-input'

const formSchema = z.object({
  email: z.email({
    error: (iss) => (iss.input === '' ? 'Please enter your email.' : undefined),
  }),
  password: z
    .string()
    .min(1, 'Please enter your password.')
    .min(6, 'Password must be at least 6 characters long.'),
})

interface UserAuthFormProps extends React.HTMLAttributes<HTMLFormElement> {
  redirectTo?: string
}

export function UserAuthForm({
  className,
  redirectTo,
  ...props
}: UserAuthFormProps) {
  const [isLoading, setIsLoading] = useState(false)
  const navigate = useNavigate()

  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      email: '',
      password: '',
    },
  })

  const { login } = useAuth()

  async function onSubmit(data: z.infer<typeof formSchema>) {
    setIsLoading(true)
    try {
      await login({ email: data.email, password: data.password })
      toast.success('Signed in successfully!')
      // Guard stores a full URL in `redirect`; navigate() only accepts paths,
      // so strip the origin. Sign-in itself is never a valid target.
      let targetPath = '/'
      if (redirectTo) {
        try {
          const url = new URL(redirectTo, window.location.origin)
          if (url.pathname !== '/sign-in' && url.pathname !== '/admin/sign-in') {
            targetPath = `${url.pathname}${url.search}${url.hash}`
          }
        } catch {
          if (redirectTo.startsWith('/')) targetPath = redirectTo
        }
      }
      navigate({ to: targetPath, replace: true })
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Sign in failed'
      toast.error(msg)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Email</FormLabel>
              <FormControl>
                <Input placeholder='admin@goshop.com' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel>Password</FormLabel>
              <FormControl>
                <PasswordInput placeholder='••••••••' {...field} />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />
        <Button className='mt-2 w-full' disabled={isLoading}>
          {isLoading ? <Loader2 className='h-4 w-4 animate-spin' /> : <LogIn className='h-4 w-4' />}
          Sign in
        </Button>
      </form>
    </Form>
  )
}
