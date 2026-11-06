import { useEffect, useRef } from 'react'
import { Outlet, useLocation, useNavigate } from '@tanstack/react-router'
import { getCookie } from '@/lib/cookies'
import { tryRefreshSession } from '@/lib/api'
import { sanitizeRedirectTarget } from '@/lib/auth-redirect'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { LayoutProvider } from '@/context/layout-provider'
import { SearchProvider } from '@/context/search-provider'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { AppSidebar } from '@/components/layout/app-sidebar'
import { SkipToMain } from '@/components/skip-to-main'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout({ children }: AuthenticatedLayoutProps) {
  const defaultOpen = getCookie('sidebar_state') !== 'false'
  const accessToken = useAuthStore((s) => s.auth.accessToken)
  const user = useAuthStore((s) => s.auth.user)
  const navigate = useNavigate()
  const location = useLocation()

  // The route guard's beforeLoad only runs on navigation, but the session can
  // die while this layout is already mounted (token expired mid-use, refresh
  // failed -> the API client calls auth.reset()). React to that and send the
  // user to sign-in instead of leaving them staring at failing queries.
  // Try one silent refresh first so short-lived access tokens do not bounce
  // the admin to sign-in on every expiry.
  const refreshing = useRef(false)
  useEffect(() => {
    if (accessToken || user || refreshing.current) return
    refreshing.current = true
    void tryRefreshSession().then((ok) => {
      refreshing.current = false
      if (!ok) {
        const target = sanitizeRedirectTarget(location.href)
        navigate({
          to: '/sign-in',
          search: target ? { redirect: target } : undefined,
          replace: true,
        })
      }
    })
  }, [accessToken, user, navigate, location.href])

  return (
    <SearchProvider>
      <LayoutProvider>
        <SidebarProvider defaultOpen={defaultOpen}>
          <SkipToMain />
          <AppSidebar />
          <SidebarInset
            className={cn(
              // Set content container, so we can use container queries
              '@container/content',

              // If layout is fixed, set the height
              // to 100svh to prevent overflow
              'has-data-[layout=fixed]:h-svh',

              // If layout is fixed and sidebar is inset,
              // set the height to 100svh - spacing (total margins) to prevent overflow
              'peer-data-[variant=inset]:has-data-[layout=fixed]:h-[calc(100svh-(var(--spacing)*4))]'
            )}
          >
            <div className='flex min-h-0 flex-1 flex-col'>
              {children ?? <Outlet />}
            </div>
          </SidebarInset>
        </SidebarProvider>
      </LayoutProvider>
    </SearchProvider>
  )
}
