"use client";

import React, { useState, useEffect, useRef } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { formatCurrency } from "@/lib/format";
import {
  ShoppingCart,
  User,
  Menu,
  X,
  Sparkles,
  Flame,
  Truck,
  Phone,
  HelpCircle,
  Package,
  LogOut,
  ChevronDown,
  RotateCcw,
  Zap,
  Ghost,
  Shirt,
  Gamepad2,
} from "lucide-react";
import { useMounted } from "@/hooks/use-mounted";
import { NotificationPopover } from "@/components/layout/notification-popover";
import { SearchAutocomplete } from "@/components/layout/search-autocomplete";
import { Button } from "@/components/ui/button";

export function Navbar() {
  const pathname = usePathname();
  const router = useRouter();
  const { items, getTotalItems, openDrawer } = useCartStore();
  const { user, accessToken, reset } = useAuthStore();

  const mounted = useMounted();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [userDropdownOpen, setUserDropdownOpen] = useState(false);
  const [isScrolled, setIsScrolled] = useState(false);
  const userMenuRef = useRef<HTMLDivElement>(null);

  const isHome = pathname === "/";
  const isAuthenticated = Boolean(user && accessToken);
  const totalCartItems = mounted && isAuthenticated ? getTotalItems() : 0;

  // Calculate cart subtotal for instant header display
  const cartSubtotal =
    mounted && isAuthenticated
      ? items.reduce((sum, item) => sum + item.price * item.quantity, 0)
      : 0;

  // Detect window scroll to toggle compact sticky header style
  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 30);
    };

    handleScroll();
    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  // Close dropdown on outside click
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        userMenuRef.current &&
        !userMenuRef.current.contains(e.target as Node)
      ) {
        setUserDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleCartClick = () => {
    if (!isAuthenticated) {
      router.push("/login?redirect=/cart");
      return;
    }
    openDrawer();
  };

  const handleLogout = () => {
    reset();
    setUserDropdownOpen(false);
    router.push("/login");
  };

  const [apiCategories, setApiCategories] = useState<
    { name: string; href: string; badge?: string }[]
  >([]);

  useEffect(() => {
    fetch(
      `${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}/api/v1/categories`,
    )
      .then((res) => (res.ok ? res.json() : null))
      .then((json) => {
        if (json?.data && Array.isArray(json.data)) {
          const cats = json.data.slice(0, 5).map((c: any, idx: number) => ({
            name: c.category_name,
            href: `/products?category=${c.slug}`,
            badge: idx === 0 ? "Popular" : undefined,
          }));
          setApiCategories(cats);
        }
      })
      .catch(() => {});
  }, []);

  const defaultNavCategories: {
    name: string;
    href: string;
    icon: React.ElementType;
    badge?: string;
    isHighlight?: boolean;
  }[] = [
    {
      name: "Smartphones",
      href: "/products?category=smartphones-tablets",
      icon: Ghost,
      badge: "Popular",
      isHighlight: false,
    },
    {
      name: "Laptops",
      href: "/products?category=laptops-computers",
      icon: Shirt,
      isHighlight: false,
    },
    {
      name: "Audio & Sound",
      href: "/products?category=audio-sound",
      icon: Package,
      isHighlight: false,
    },
    {
      name: "Gaming Gear",
      href: "/products?category=gaming-consoles",
      icon: Gamepad2,
      isHighlight: false,
    },
  ];

  const dynamicCategories: {
    name: string;
    href: string;
    icon: React.ElementType;
    badge?: string;
    isHighlight?: boolean;
  }[] =
    apiCategories.length > 0
      ? apiCategories.map((cat, idx) => ({
          ...cat,
          icon:
            idx === 0
              ? Ghost
              : idx === 1
                ? Shirt
                : idx === 2
                  ? Package
                  : Gamepad2,
          isHighlight: false,
        }))
      : defaultNavCategories;

  const navCategories: {
    name: string;
    href: string;
    icon: React.ElementType;
    badge?: string;
    isHighlight?: boolean;
  }[] = [
    ...dynamicCategories,
    { name: "Flash Deals", href: "/#deals", icon: Flame, isHighlight: true },
    {
      name: "New Drops",
      href: "/products?sort=newest",
      icon: Sparkles,
      isHighlight: false,
    },
  ];

  return (
    <header className="sticky top-0 left-0 right-0 z-40 w-full transition-all duration-200">
      {/* 1. Top Utility & Announcement Bar */}
      <div className="bg-[#071F2C] text-[#B0CDD9] text-xs border-b border-[#00384D] py-1.5 px-4 sm:px-6">
        <div className="max-w-7xl mx-auto flex items-center justify-between gap-4 font-body">
          {/* Left: Dynamic Announcement Ticker */}
          <div className="flex items-center gap-2 overflow-hidden truncate">
            <span className="inline-flex items-center gap-1 bg-[#F26522] text-white px-2.5 py-0.5 rounded-full text-[10px] font-extrabold uppercase tracking-wider shrink-0 shadow-xs">
              <Flame className="w-3 h-3 fill-white" />
              <span>GOPHER DROP 04</span>
            </span>
            <p className="truncate text-[#E0F7FC] text-xs">
              Free Nationwide Express Delivery on all orders over 500,000₫ •
              100% Authentic Merch Guarantee
            </p>
          </div>

          {/* Right: Quick Utility Links */}
          <div className="hidden lg:flex items-center gap-4 text-[#B0CDD9] shrink-0 text-xs font-medium">
            <Link
              href="/account/orders"
              className="hover:text-[#E0F7FC] transition-colors flex items-center gap-1.5"
            >
              <Package className="w-3.5 h-3.5" />
              <span>Track Order</span>
            </Link>
            <span className="text-[#004D66]">|</span>
            <Link
              href="/faq"
              className="hover:text-[#E0F7FC] transition-colors flex items-center gap-1.5"
            >
              <HelpCircle className="w-3.5 h-3.5" />
              <span>Help Center</span>
            </Link>
            <span className="text-[#004D66]">|</span>
            <div className="flex items-center gap-1.5 text-[#E0F7FC]">
              <Phone className="w-3.5 h-3.5 text-[#00ADD8]" />
              <span className="font-bold">Hotline: 1900-6868</span>
            </div>
          </div>
        </div>
      </div>

      {/* 2. Main Navigation Bar Tier (GoLang Blue) */}
      <div
        className={`w-full bg-[#007D9C] border-b border-[#00637C] shadow-md transition-all duration-200 ${
          isScrolled ? "py-2.5 shadow-lg" : "py-3"
        }`}
      >
        <div className="max-w-7xl mx-auto px-4 sm:px-6 flex items-center justify-between gap-4 lg:gap-8">
          {/* Brand Logo & Tag */}
          <div className="flex items-center gap-3 shrink-0">
            <Link
              href="/"
              className="flex items-center gap-2 group bg-white/95 hover:bg-white rounded-xl px-2.5 py-1 shadow-xs"
            >
              <Image
                src="/Logo.svg"
                alt="GOshop — Official E-commerce Store for Tech Collectors and Developers"
                width={150}
                height={38}
                priority
                className="h-7 sm:h-8 w-auto object-contain"
              />
            </Link>
          </div>

          {/* Center: Omnichannel Search Bar (Desktop) */}
          <div className="hidden md:flex flex-1 max-w-xl mx-auto">
            <SearchAutocomplete />
          </div>

          {/* Right Action Hub: Notifications, Cart, Account */}
          <div className="flex items-center gap-2 sm:gap-3">
            {/* Real-time Notifications */}
            {isAuthenticated && <NotificationPopover isTransparent={false} />}

            {/* Cart Trigger with Live Counter & Subtotal */}
            <button
              type="button"
              onClick={handleCartClick}
              className="flex items-center gap-2.5 p-2 rounded-full text-white hover:text-[#007D9C] hover:bg-[#E6F7FB] cursor-pointer shadow-xs"
              aria-label={`Shopping cart with ${totalCartItems} items`}
            >
              <div className="relative flex items-center justify-center">
                <ShoppingCart className="w-5 h-5" />
                {totalCartItems > 0 && (
                  <span className="absolute -top-3 -right-2 min-w-[18px] h-[18px] px-1 bg-[#F26522] text-white text-[10px] font-black rounded-full flex items-center justify-center ring-2 ring-[#007D9C] shadow-xs">
                    {totalCartItems > 99 ? "99+" : totalCartItems}
                  </span>
                )}
              </div>
            </button>

            {/* Account / User Menu */}
            {isAuthenticated ? (
              <div ref={userMenuRef} className="relative">
                <button
                  type="button"
                  onClick={() => setUserDropdownOpen(!userDropdownOpen)}
                  className="flex items-center gap-2 p-1 sm:px-2.5 sm:py-1 rounded-full bg-white/10 hover:bg-white/20 border border-white/20 text-white transition-all cursor-pointer"
                  aria-label="User Account Menu"
                >
                  <div className="w-7 h-7 rounded-full bg-[#BAEAFF] border border-[#5CD4FF] text-[#004D62] font-bold flex items-center justify-center text-xs shadow-xs">
                    {user?.fullName ? user.fullName[0].toUpperCase() : "U"}
                  </div>
                  <div className="hidden sm:flex flex-col text-left">
                    <span className="text-xs font-bold text-white max-w-[90px] truncate leading-tight font-heading">
                      {user?.fullName || user?.email?.split("@")[0]}
                    </span>
                  </div>
                  <ChevronDown className="w-3.5 h-3.5 text-[#BAEAFF] hidden sm:inline" />
                </button>

                {/* Dropdown Menu */}
                {userDropdownOpen && (
                  <div className="absolute right-0 top-full mt-2 w-56 bg-white rounded-2xl shadow-2xl border border-slate-200/90 py-2 z-50 animate-in fade-in slide-in-from-top-2 duration-150 text-slate-900">
                    <div className="px-4 py-2 border-b border-slate-100">
                      <p className="text-xs font-bold text-slate-900 truncate font-heading">
                        {user?.fullName || "Valued Customer"}
                      </p>
                      <p className="text-[11px] text-slate-400 truncate font-body">
                        {user?.email}
                      </p>
                    </div>

                    <div className="py-1">
                      <Link
                        href="/account"
                        onClick={() => setUserDropdownOpen(false)}
                        className="flex items-center gap-2.5 px-4 py-2 text-xs font-semibold text-slate-700 hover:bg-[#E6F7FB] hover:text-[#007D9C] transition-colors"
                      >
                        <User className="w-4 h-4" />
                        <span>My Account Profile</span>
                      </Link>
                      <Link
                        href="/account/orders"
                        onClick={() => setUserDropdownOpen(false)}
                        className="flex items-center gap-2.5 px-4 py-2 text-xs font-semibold text-slate-700 hover:bg-[#E6F7FB] hover:text-[#007D9C] transition-colors"
                      >
                        <Package className="w-4 h-4" />
                        <span>My Orders</span>
                      </Link>
                      <Link
                        href="/account/returns"
                        onClick={() => setUserDropdownOpen(false)}
                        className="flex items-center gap-2.5 px-4 py-2 text-xs font-semibold text-slate-700 hover:bg-[#E6F7FB] hover:text-[#007D9C] transition-colors"
                      >
                        <RotateCcw className="w-4 h-4" />
                        <span>Return Requests</span>
                      </Link>
                    </div>

                    <div className="pt-1 border-t border-slate-100">
                      <button
                        type="button"
                        onClick={handleLogout}
                        className="w-full flex items-center gap-2.5 px-4 py-2 text-xs font-semibold text-red-600 hover:bg-red-50 transition-colors text-left cursor-pointer"
                      >
                        <LogOut className="w-4 h-4" />
                        <span>Sign Out</span>
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <Button
                  asChild
                  variant="secondary"
                  size="sm"
                  className="rounded-full bg-white text-[#007D9C] hover:bg-[#E6F7FB] font-bold px-4 border-0 shadow-xs"
                >
                  <Link href="/login">
                    <User className="w-3.5 h-3.5 text-[#007D9C]" />
                    <span>Sign In</span>
                  </Link>
                </Button>
                <Button
                  asChild
                  variant="outline"
                  size="sm"
                  className="hidden sm:inline-flex rounded-full bg-transparent text-white border-white/40 hover:bg-white/10 hover:text-white"
                >
                  <Link href="/register">Register</Link>
                </Button>
              </div>
            )}

            {/* Mobile Hamburger Toggle */}
            <button
              type="button"
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="md:hidden w-9 h-9 rounded-xl flex items-center justify-center text-white hover:bg-white/15 transition-colors cursor-pointer"
              aria-label="Open mobile navigation"
            >
              {mobileMenuOpen ? (
                <X className="w-5 h-5" />
              ) : (
                <Menu className="w-5 h-5" />
              )}
            </button>
          </div>
        </div>
      </div>

      {/* 3. Category Sub-Navigation Bar (Desktop: Soft Ice Cyan) */}
      <div className="hidden md:block bg-[#F0F9FB] border-b border-[#D0EDF5] py-2 px-4 sm:px-6 shadow-2xs">
        <div className="max-w-7xl mx-auto flex items-center justify-between">
          <nav className="flex items-center gap-2 lg:gap-3 overflow-x-auto custom-scrollbar">
            <Link
              href="/products"
              className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-full text-xs font-bold bg-[#007D9C] text-white hover:bg-[#00637C] shadow-xs transition-all font-heading shrink-0"
            >
              <span>All Collections</span>
            </Link>

            {navCategories.map((cat) => {
              const Icon = cat.icon;
              return (
                <Link
                  key={cat.name}
                  href={cat.href}
                  className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-semibold transition-all font-body shrink-0 ${
                    cat.isHighlight
                      ? "text-[#F26522] hover:bg-orange-50 font-bold"
                      : "text-[#005F73] hover:text-[#007D9C] hover:bg-white/80"
                  }`}
                >
                  <Icon
                    className={`w-3.5 h-3.5 ${
                      cat.isHighlight ? "text-[#F26522]" : "text-[#007D9C]"
                    }`}
                  />
                  <span>{cat.name}</span>
                  {cat.badge && (
                    <span className="text-[9px] font-bold bg-[#BAEAFF] text-[#004D62] px-2 py-0.5 rounded-full">
                      {cat.badge}
                    </span>
                  )}
                </Link>
              );
            })}
          </nav>
        </div>
      </div>

      {/* 4. Mobile Navigation Drawer */}
      {mobileMenuOpen && (
        <div className="md:hidden fixed inset-x-0 top-[110px] bottom-0 bg-slate-950/60 backdrop-blur-xs z-50 animate-in fade-in-0">
          <div className="bg-white h-full max-h-[85vh] overflow-y-auto p-5 space-y-4 shadow-2xl border-t border-slate-200">
            {/* Mobile Search */}
            <SearchAutocomplete
              isMobile
              onClose={() => setMobileMenuOpen(false)}
            />

            {/* Category Navigation */}
            <div>
              <p className="text-[11px] font-bold text-slate-400 uppercase tracking-wider mb-2 font-heading">
                Shop Collections
              </p>
              <div className="grid grid-cols-2 gap-2">
                {navCategories.map((cat) => {
                  const Icon = cat.icon;
                  return (
                    <Link
                      key={cat.name}
                      href={cat.href}
                      onClick={() => setMobileMenuOpen(false)}
                      className="p-3 rounded-xl bg-slate-50 hover:bg-[#E6F7FB] border border-slate-200/80 flex items-center gap-2.5 text-xs font-bold text-slate-800"
                    >
                      <Icon className="w-4 h-4 text-[#00ADD8]" />
                      <span className="truncate">{cat.name}</span>
                    </Link>
                  );
                })}
              </div>
            </div>

            {/* Quick Links */}
            <div className="pt-2 border-t border-slate-100 space-y-1">
              <Link
                href="/account/orders"
                onClick={() => setMobileMenuOpen(false)}
                className="flex items-center justify-between p-2.5 rounded-xl hover:bg-slate-50 text-xs font-semibold text-slate-700"
              >
                <span className="flex items-center gap-2">
                  <Package className="w-4 h-4 text-slate-400" />
                  <span>Track My Order</span>
                </span>
                <span className="text-slate-400 text-[10px]">GHN Express</span>
              </Link>
              <Link
                href="/faq"
                onClick={() => setMobileMenuOpen(false)}
                className="flex items-center justify-between p-2.5 rounded-xl hover:bg-slate-50 text-xs font-semibold text-slate-700"
              >
                <span className="flex items-center gap-2">
                  <HelpCircle className="w-4 h-4 text-slate-400" />
                  <span>Help Center & FAQ</span>
                </span>
              </Link>
            </div>

            {/* Mobile Auth Action */}
            {!isAuthenticated && (
              <div className="pt-2 border-t border-slate-100 grid grid-cols-2 gap-2">
                <Button
                  asChild
                  variant="default"
                  size="default"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  <Link href="/login">Sign In</Link>
                </Button>
                <Button
                  asChild
                  variant="secondary"
                  size="default"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  <Link href="/register">Create Account</Link>
                </Button>
              </div>
            )}
          </div>
        </div>
      )}
    </header>
  );
}
