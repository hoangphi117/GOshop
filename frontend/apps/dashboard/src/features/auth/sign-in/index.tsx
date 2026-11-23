import { useSearch } from '@tanstack/react-router'
import { sanitizeRedirectTarget } from '@/lib/auth-redirect'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  const { redirect } = useSearch({ from: '/(auth)/sign-in' })

  return (
    <AuthLayout>
      <Card className='w-full shadow-sm sm:w-[380px]'>
        <CardHeader className='text-center'>
          <div className='mx-auto flex items-center justify-center'>
            <img
              src='/main_logo.png'
              alt='GOshop'
              className='h-12 w-auto object-contain sm:h-20'
            />
          </div>
        </CardHeader>
        <CardContent>
          <UserAuthForm redirectTo={sanitizeRedirectTarget(redirect)} />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
