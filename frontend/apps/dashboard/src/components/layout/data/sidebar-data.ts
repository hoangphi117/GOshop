import {
  LayoutDashboard,
  Palette,
  Settings,
  Users,
  Layers,
  ShoppingBag,
  ShoppingCart,
  Image,
  TicketPercent,
  RotateCcw,
  MessageSquareQuote,
  MessageSquare,
  Boxes,
} from 'lucide-react'
import { type SidebarData } from '../types'

// ponytail: one static team until multi-tenant exists; the signed-in user is
// injected at render time by AppSidebar from the auth store.
export const sidebarData: Omit<SidebarData, 'user'> = {
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
          title: 'Chats',
          url: '/chats',
          icon: MessageSquare,
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
          title: 'Stock Movements',
          url: '/inventory',
          icon: Boxes,
        },
        {
          title: 'Orders',
          url: '/orders',
          icon: ShoppingCart,
        },
        {
          title: 'Returns',
          url: '/returns',
          icon: RotateCcw,
        },
        {
          title: 'Reviews',
          url: '/reviews',
          icon: MessageSquareQuote,
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
              title: 'Appearance',
              url: '/settings/appearance',
              icon: Palette,
            },
          ],
        },
      ],
    },
  ],
}
