import { useSearch } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardHeader,
} from '@/components/ui/card'
import { sanitizeRedirectTarget } from '@/lib/auth-redirect'
import { AuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  const { redirect } = useSearch({ from: '/(auth)/sign-in' })

  return (
    <AuthLayout>
      <Card className='w-full sm:w-[380px] shadow-sm'>
        <CardHeader className='pb-4 text-center'>
          <div className='mx-auto  flex items-center justify-center'>
            <img src='/Logo.svg' alt='GOshop' className='h-10 w-auto' />
          </div>
        </CardHeader>
        <CardContent>
          <UserAuthForm redirectTo={sanitizeRedirectTarget(redirect)} />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
