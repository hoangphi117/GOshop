"use client";

import React from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth-store";
import { useMounted } from "@/hooks/use-mounted";
import { authFetch } from "@/lib/api";
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
  Bell,
} from "lucide-react";
import { toast } from "sonner";
import { useNotificationStore } from "@/stores/notification-store";

export default function AccountLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, accessToken, reset } = useAuthStore();
  const { unreadCount } = useNotificationStore();
  const mounted = useMounted();

  if (!mounted) {
    return (
      <div className="min-h-[70vh] flex items-center justify-center">
        <div className="w-8 h-8 rounded-md border-2 border-[#007D9C] border-t-transparent" />
      </div>
    );
  }

  if (!user || !accessToken) {
    return (
      <div className="min-h-[70vh] bg-[#F8FAFC] py-16 flex items-center justify-center font-body">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-16 h-16 rounded-md bg-[#E6F7FB] text-[#007D9C] flex items-center justify-center mx-auto">
            <LogIn className="w-8 h-8" />
          </div>
          <h2 className="text-xl font-bold text-[#0F172A] font-heading">
            Sign in to access your account
          </h2>
          <p className="text-xs text-slate-500 max-w-sm mx-auto">
            View orders, saved addresses, manage reviews, and customize your
            profile.
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              href="/login"
              className="w-full sm:w-auto px-6 py-2.5 rounded-md bg-[#007D9C] text-white text-xs font-semibold hover:bg-[#006782] transition-colors shadow-xs"
            >
              <span>Sign In</span>
            </Link>
            <Link
              href="/register"
              className="w-full sm:w-auto px-6 py-2.5 rounded-md border border-[#E2E8F0] bg-white text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-colors"
            >
              <span>Create Account</span>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const handleLogout = async () => {
    try {
      const refreshToken = useAuthStore.getState().refreshToken;
      await authFetch("/api/v1/auth/logout", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(refreshToken ? { refresh_token: refreshToken } : {}),
      });
    } catch {
      // Ignore
    }
    reset();
    router.push("/login");
  };

  const navItems = [
    {
      name: "Personal Profile",
      href: "/account/profile",
      icon: User,
    },
    {
      name: "My Orders",
      href: "/account/orders",
      icon: ShoppingBag,
    },
    {
      name: "Notifications",
      href: "/account/notifications",
      icon: Bell,
      badge: unreadCount > 0 ? unreadCount : undefined,
    },
    {
      name: "Returns & Refunds",
      href: "/account/returns",
      icon: RotateCcw,
    },
    {
      name: "My Reviews",
      href: "/account/reviews",
      icon: Star,
    },
    {
      name: "Shipping Addresses",
      href: "/account/addresses",
      icon: MapPin,
    },
    {
      name: "Password & Security",
      href: "/account/security",
      icon: Shield,
    },
  ];

  const displayName = user?.fullName || user?.email?.split("@")[0] || "Member";

  return (
    <div className="bg-[#F8FAFC] min-h-screen py-6 font-body">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        {/* Breadcrumbs */}
        <nav className="flex items-center gap-2 text-xs text-slate-500 mb-6">
          <Link href="/" className="hover:text-[#007D9C] transition-colors">
            Home
          </Link>
          <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
          <span className="font-semibold text-[#0F172A]">My Account</span>
        </nav>

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 lg:gap-8">
          {/* Sidebar (3 cols) */}
          <aside className="lg:col-span-3 space-y-6">
            {/* User Profile Summary Card */}
            <div className="p-4 rounded-md bg-white border border-[#E2E8F0] shadow-xs flex items-center gap-3.5">
              <div className="w-12 h-12 rounded-md bg-[#007D9C] text-white flex items-center justify-center text-lg font-bold shrink-0 font-heading">
                {displayName[0].toUpperCase()}
              </div>
              <div className="min-w-0 flex-1">
                <h2 className="text-sm font-bold text-[#0F172A] truncate font-heading">
                  {displayName}
                </h2>
                <p className="text-[11px] text-slate-500 truncate">
                  {user?.email}
                </p>
              </div>
            </div>

            {/* Navigation Menu */}
            <div className="p-2 rounded-md bg-white border border-[#E2E8F0] shadow-xs space-y-1">
              {navItems.map((item) => {
                const Icon = item.icon;
                const isActive =
                  pathname === item.href ||
                  (item.href === "/account/profile" && pathname === "/account");

                return (
                  <Link
                    key={item.name}
                    href={item.href}
                    className={`flex items-center justify-between px-3 py-2.5 rounded-md text-xs font-semibold transition-colors ${
                      isActive
                        ? "bg-[#E6F7FB] text-[#007D9C] font-bold"
                        : "text-slate-600 hover:bg-slate-50 hover:text-[#0F172A]"
                    }`}
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <Icon
                        className={`w-4 h-4 shrink-0 ${
                          isActive ? "text-[#007D9C]" : "text-slate-400"
                        }`}
                      />
                      <span className="truncate">{item.name}</span>
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {item.badge !== undefined && (
                        <span className="px-1.5 py-0.5 rounded-md bg-rose-500 text-white text-[10px] font-bold leading-none">
                          {item.badge > 99 ? "99+" : item.badge}
                        </span>
                      )}
                      <ChevronRight
                        className={`w-4 h-4 ${
                          isActive ? "text-[#007D9C]" : "text-slate-300"
                        }`}
                      />
                    </div>
                  </Link>
                );
              })}

              <div className="pt-2 border-t border-[#E2E8F0] mt-2">
                <button
                  onClick={handleLogout}
                  className="w-full flex items-center gap-3 px-3 py-2.5 rounded-md text-xs font-semibold text-rose-600 hover:bg-rose-50 transition-colors"
                >
                  <LogOut className="w-4 h-4 text-rose-600" />
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
  );
}
