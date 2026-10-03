import { useLocation } from '@tanstack/react-router'
import { Header } from '@/components/layout/header'
import { TopNav } from '@/components/layout/top-nav'
import { Search } from '@/components/search'
import { NotificationBell } from '@/features/notifications/components/notification-bell'
import { ThemeSwitch } from '@/components/theme-switch'
import { ConfigDrawer } from '@/components/config-drawer'
import { ProfileDropdown } from '@/components/profile-dropdown'

interface DashboardHeaderProps {
  fixed?: boolean
  className?: string
  children?: React.ReactNode
}

export function DashboardHeader({
  fixed = true,
  className,
  children,
}: DashboardHeaderProps) {
  const { pathname } = useLocation()

  const topNav = [
    {
      title: 'Overview',
      href: '/',
      isActive: pathname === '/' || pathname === '/dashboard/overview',
      disabled: false,
    },
    {
      title: 'Customers',
      href: '/users',
      isActive: pathname.startsWith('/users'),
      disabled: false,
    },
    {
      title: 'Products',
      href: '/products',
      isActive: pathname.startsWith('/products'),
      disabled: false,
    },
    {
      title: 'Settings',
      href: '/settings',
      isActive: pathname.startsWith('/settings'),
      disabled: false,
    },
  ]

  return (
    <Header fixed={fixed} className={className}>
      <TopNav links={topNav} className='me-auto' />
      <Search />
      <NotificationBell />
      <ThemeSwitch />
      <ConfigDrawer />
      <ProfileDropdown />
      {children}
    </Header>
  )
}
