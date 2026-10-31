import { useNavigate, useRouter } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'

export function ForbiddenError() {
  const navigate = useNavigate()
  const { history } = useRouter()
  const { auth } = useAuthStore()

  const handleSignOut = () => {
    auth.reset()
    navigate({ to: '/sign-in', replace: true })
  }

  return (
    <div className='h-svh'>
      <div className='m-auto flex h-full w-full flex-col items-center justify-center gap-2'>
        <h1 className='text-[7rem] leading-tight font-bold'>403</h1>
        <span className='font-medium text-lg'>Access Forbidden</span>
        <p className='text-center text-muted-foreground max-w-md'>
          You do not have administrative or staff privileges to access this dashboard.
        </p>
        <div className='mt-6 flex gap-4'>
          <Button variant='outline' onClick={() => history.go(-1)}>
            Go Back
          </Button>
          <Button onClick={handleSignOut}>
            Sign in with another account
          </Button>
        </div>
      </div>
    </div>
  )
}
