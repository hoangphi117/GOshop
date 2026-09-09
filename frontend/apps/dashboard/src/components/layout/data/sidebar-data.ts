import {
  LayoutDashboard,
  Monitor,
  HelpCircle,
  Bell,
  Palette,
  Settings,
  Wrench,
  UserCog,
  Users,
  Layers,
  ShoppingBag,
  ShoppingCart,
  Image,
  TicketPercent,
} from 'lucide-react'
import { type SidebarData } from '../types'

export const sidebarData: SidebarData = {
  user: {
    name: 'Admin GOshop',
    email: 'admin@goshop.vn',
    avatar: '/avatars/shadcn.jpg',
  },
  teams: [
    {
      name: 'GOshop Admin',
      logo: ShoppingBag,
      plan: 'Enterprise',
    },
  ],
  navGroups: [
    {
      title: 'General',
      items: [
        {
          title: 'Dashboard',
          url: '/',
          icon: LayoutDashboard,
        },
        {
          title: 'Users',
          url: '/users',
          icon: Users,
        },
      ],
    },
    {
      title: 'E-Commerce',
      items: [
        {
          title: 'Categories',
          url: '/categories',
          icon: Layers,
        },
        {
          title: 'Products',
          url: '/products',
          icon: ShoppingBag,
        },
        {
          title: 'Orders',
          url: '/orders',
          icon: ShoppingCart,
        },
        {
          title: 'Banners',
          url: '/banners',
          icon: Image,
        },
        {
          title: 'Coupons',
          url: '/coupons',
          icon: TicketPercent,
        },
      ],
    },
    {
      title: 'Other',
      items: [
        {
          title: 'Settings',
          icon: Settings,
          items: [
            {
              title: 'Profile',
              url: '/settings',
              icon: UserCog,
            },
            {
              title: 'Account',
              url: '/settings/account',
              icon: Wrench,
            },
            {
              title: 'Appearance',
              url: '/settings/appearance',
              icon: Palette,
            },
            {
              title: 'Notifications',
              url: '/settings/notifications',
              icon: Bell,
            },
            {
              title: 'Display',
              url: '/settings/display',
              icon: Monitor,
            },
          ],
        },
        {
          title: 'Help Center',
          url: '/help-center',
          icon: HelpCircle,
        },
      ],
    },
  ],
}
