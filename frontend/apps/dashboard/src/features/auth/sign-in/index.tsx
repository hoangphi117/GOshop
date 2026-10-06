import { useSearch } from '@tanstack/react-router'
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { AuthLayout } from '../auth-layout'
import { UserAuthForm } from './components/user-auth-form'

export function SignIn() {
  const { redirect } = useSearch({ from: '/(auth)/sign-in' })

  return (
    <AuthLayout>
      <Card className='w-full sm:w-[380px] shadow-sm'>
        <CardHeader className='pb-4 text-center'>
          <div className='mx-auto mb-3 flex items-center justify-center'>
            <img src='/Logo.svg' alt='GOshop' className='h-10 w-auto' />
          </div>
          <CardTitle className='text-2xl font-bold tracking-tight'>
            GOshop Admin
          </CardTitle>
        </CardHeader>
        <CardContent>
          <UserAuthForm redirectTo={redirect} />
        </CardContent>
      </Card>
    </AuthLayout>
  )
}
