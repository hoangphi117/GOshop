'use client'

import React from 'react'
import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { useAuthStore } from '@/stores/auth-store'
import { useMounted } from '@/hooks/use-mounted'
import { authFetch } from '@/lib/api'
import {
  User,
  ShoppingBag,
  RotateCcw,
  Star,
  MapPin,
  Shield,
  LogOut,
  ChevronRight,
  LogIn,
} from 'lucide-react'
import { toast } from 'sonner'

export default function AccountLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const pathname = usePathname()
  const router = useRouter()
  const { user, accessToken, reset } = useAuthStore()
  const mounted = useMounted()

  if (!mounted) {
    return (
      <div className="min-h-[70vh] flex items-center justify-center">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    )
  }

  if (!user || !accessToken) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-16 h-16 rounded-full bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto">
            <LogIn className="w-8 h-8" />
          </div>
          <h2 className="text-xl font-bold text-[#0D1B2A]">
            Sign in to access your account
          </h2>
          <p className="text-xs text-slate-500 max-w-sm mx-auto">
            View orders, saved addresses, manage reviews, and customize your profile.
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              href="/login"
              className="w-full sm:w-auto px-6 py-2.5 rounded-xl bg-[#2196F3] text-white text-xs font-semibold hover:bg-[#0D47A1] transition-all shadow-xs"
            >
              <span>Sign In</span>
            </Link>
            <Link
              href="/register"
              className="w-full sm:w-auto px-6 py-2.5 rounded-xl border border-slate-200 bg-white text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-all"
            >
              <span>Create Account</span>
            </Link>
          </div>
        </div>
      </div>
    )
  }

  const handleLogout = async () => {
    try {
      await authFetch('/api/v1/auth/logout', { method: 'POST' })
    } catch {
      // Ignore
    }
    reset()
    toast.success('Logged out successfully.')
    router.push('/')
  }

  const navItems = [
    {
      name: 'Personal Profile',
      href: '/account/profile',
      icon: User,
    },
    {
      name: 'My Orders',
      href: '/account/orders',
      icon: ShoppingBag,
    },
    {
      name: 'Returns & Refunds',
      href: '/account/returns',
      icon: RotateCcw,
    },
    {
      name: 'My Reviews',
      href: '/account/reviews',
      icon: Star,
    },
    {
      name: 'Shipping Addresses',
      href: '/account/addresses',
      icon: MapPin,
    },
    {
      name: 'Password & Security',
      href: '/account/security',
      icon: Shield,
    },
  ]

  const displayName = user?.fullName || user?.email?.split('@')[0] || 'Member'

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-10">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Breadcrumbs */}
        <nav className="flex items-center gap-2 text-xs text-slate-500 mb-6">
          <Link href="/" className="hover:text-[#2196F3] transition-colors">
            Home
          </Link>
          <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
          <span className="font-semibold text-[#0D1B2A]">My Account</span>
        </nav>

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 lg:gap-8">
          {/* Sidebar (3 cols) */}
          <aside className="lg:col-span-3 space-y-6">
            {/* User Profile Summary Card */}
            <div className="p-5 rounded-3xl bg-white border border-slate-200/80 shadow-xs flex items-center gap-3.5">
              <div className="w-12 h-12 rounded-2xl bg-gradient-to-tr from-[#0D47A1] to-[#2196F3] text-white flex items-center justify-center text-lg font-black shadow-md shrink-0">
                {displayName[0].toUpperCase()}
              </div>
              <div className="min-w-0 flex-1">
                <h2 className="text-sm font-bold text-[#0D1B2A] truncate">
                  {displayName}
                </h2>
                <p className="text-[11px] text-slate-500 truncate">{user?.email}</p>
                <div className="mt-1 inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-[10px] font-bold">
                  <span>Verified Member</span>
                </div>
              </div>
            </div>

            {/* Navigation Menu */}
            <div className="p-3 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-1">
              {navItems.map((item) => {
                const Icon = item.icon
                const isActive =
                  pathname === item.href ||
                  (item.href === '/account/profile' && pathname === '/account')

                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    className={`flex items-center justify-between px-4 py-3 rounded-2xl text-xs font-semibold transition-all ${
                      isActive
                        ? 'bg-[#E3F2FD] text-[#0D47A1] font-bold shadow-xs'
                        : 'text-slate-600 hover:bg-slate-50 hover:text-[#0D1B2A]'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <Icon
                        className={`w-4 h-4 ${
                          isActive ? 'text-[#2196F3]' : 'text-slate-400'
                        }`}
                      />
                      <span>{item.name}</span>
                    </div>
                    <ChevronRight
                      className={`w-4 h-4 ${
                        isActive ? 'text-[#0D47A1]' : 'text-slate-300'
                      }`}
                    />
                  </Link>
                )
              })}

              <div className="pt-2 border-t border-slate-100 mt-2">
                <button
                  onClick={handleLogout}
                  className="w-full flex items-center gap-3 px-4 py-3 rounded-2xl text-xs font-semibold text-rose-500 hover:bg-rose-50 transition-colors"
                >
                  <LogOut className="w-4 h-4 text-rose-500" />
                  <span>Sign Out</span>
                </button>
              </div>
            </div>
          </aside>

          {/* Content Area (9 cols) */}
          <main className="lg:col-span-9">{children}</main>
        </div>
      </div>
    </div>
  )
}
