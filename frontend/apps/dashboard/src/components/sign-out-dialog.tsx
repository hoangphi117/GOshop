import { useNavigate, useLocation } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { API_KEY, API_VERSIONED_BASE, CLIENT_HEADERS } from '@/lib/api'
import { sanitizeRedirectTarget } from '@/lib/auth-redirect'
import { ConfirmDialog } from '@/components/confirm-dialog'

interface SignOutDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SignOutDialog({ open, onOpenChange }: SignOutDialogProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const { auth } = useAuthStore()

  const handleSignOut = async () => {
    try {
      // Refresh token is an HttpOnly cookie; logout reads it server-side.
      await fetch(`${API_VERSIONED_BASE}/auth/logout`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(API_KEY ? { 'X-API-KEY': API_KEY } : {}),
          ...CLIENT_HEADERS,
        },
        credentials: 'include',
        body: JSON.stringify({}),
      })
    } catch {
      // ignore
    } finally {
      auth.reset()
      const target = sanitizeRedirectTarget(location.href)
      navigate({
        to: '/sign-in',
        search: target ? { redirect: target } : undefined,
        replace: true,
      })
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title='Sign out'
      desc='Are you sure you want to sign out? You will need to sign in again to access your account.'
      confirmText='Sign out'
      destructive
      handleConfirm={handleSignOut}
      className='sm:max-w-sm'
    />
  )
}
