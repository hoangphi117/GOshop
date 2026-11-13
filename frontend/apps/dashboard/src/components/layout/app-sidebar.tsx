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
import { useAuthStore } from '@/stores/auth-store'

export function AppSidebar() {
  const { collapsible, variant } = useLayout()
  const { data: unreadCount = 0 } = useAdminChatUnreadCount()
  const authUser = useAuthStore((state) => state.auth.user)
  const user = {
    name: authUser?.fullName || authUser?.email || 'GOshop Admin',
    email: authUser?.email || '',
    // No avatar storage yet: NavUser falls back to initials.
    avatar: '',
  }

  const navGroups = useMemo(() => {
    return sidebarData.navGroups.map((group) => ({
      ...group,
      items: group.items.map((item) => {
        if ('url' in item && item.url === '/chats') {
          return {
            ...item,
            badge:
              unreadCount > 0
                ? unreadCount > 99
                  ? '99+'
                  : String(unreadCount)
                : undefined,
          }
        }
        return item
      }),
    }))
  }, [unreadCount])

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
