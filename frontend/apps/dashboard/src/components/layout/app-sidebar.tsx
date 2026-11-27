import { useMemo } from 'react'
import { useLayout } from '@/context/layout-provider'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarRail,
} from '@/components/ui/sidebar'
import { sidebarData } from './data/sidebar-data'
import { NavGroup } from './nav-group'
import { NavUser } from './nav-user'
import { TeamSwitcher } from './team-switcher'
import { useAdminChatUnreadCount } from '@/features/chats/hooks/use-admin-chats'
import { useOrders } from '@/features/orders/hooks/use-orders'
import { useAdminReturns } from '@/features/orders/hooks/use-returns'
import { useAuthStore } from '@/stores/auth-store'

function formatBadge(count: number): string | undefined {
  if (count <= 0) return undefined
  return count > 99 ? '99+' : String(count)
}

export function AppSidebar() {
  const { collapsible, variant } = useLayout()
  const { data: unreadCount = 0 } = useAdminChatUnreadCount()
  const { data: pendingOrdersData } = useOrders({
    page: 1,
    limit: 1,
    status: 1,
  })
  const { data: pendingReturnsData } = useAdminReturns({
    page: 1,
    limit: 1,
    status: 1,
  })
  const pendingOrdersCount = pendingOrdersData?.pagination?.total_records ?? 0
  const pendingReturnsCount = pendingReturnsData?.total ?? 0

  const authUser = useAuthStore((state) => state.auth.user)
  const user = {
    name: authUser?.fullName || authUser?.email || 'GOshop Admin',
    email: authUser?.email || '',
    // No avatar storage yet: NavUser falls back to initials.
    avatar: '',
  }

  const navGroups = useMemo(() => {
    const badges: Record<string, string | undefined> = {
      '/chats': formatBadge(unreadCount),
      '/orders': formatBadge(pendingOrdersCount),
      '/returns': formatBadge(pendingReturnsCount),
    }

    return sidebarData.navGroups.map((group) => ({
      ...group,
      items: group.items.map((item) => {
        if ('url' in item && item.url && item.url in badges) {
          return {
            ...item,
            badge: badges[item.url],
          }
        }
        return item
      }),
    }))
  }, [unreadCount, pendingOrdersCount, pendingReturnsCount])

  return (
    <Sidebar collapsible={collapsible} variant={variant}>
      <SidebarHeader>
        <TeamSwitcher teams={sidebarData.teams} />
      </SidebarHeader>
      <SidebarContent>
        {navGroups.map((props) => (
          <NavGroup key={props.title} {...props} />
        ))}
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={user} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
