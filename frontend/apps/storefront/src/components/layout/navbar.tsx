"use client";

import React, { useState, useEffect } from "react";
import Image from "next/image";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { ShoppingCart, Search, User, Menu, X, Store, Bell } from "lucide-react";
import { useMounted } from "@/hooks/use-mounted";
import { NotificationPopover } from "@/components/layout/notification-popover";

export function Navbar() {
  const pathname = usePathname();
  const router = useRouter();
  const { getTotalItems, openDrawer } = useCartStore();
  const { user, accessToken } = useAuthStore();

  const mounted = useMounted();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [showSearch, setShowSearch] = useState(false);
  const [isScrolled, setIsScrolled] = useState(false);

  const isHome = pathname === "/";
  const isTransparent = isHome && !isScrolled;

  useEffect(() => {
    if (!isHome) {
      setIsScrolled(true);
      return;
    }

    const checkScroll = () => {
      const heroEl = document.getElementById("hero-banner");
      if (heroEl) {
        const rect = heroEl.getBoundingClientRect();
        // Scrolled past hero banner when the bottom of hero reaches the header area
        setIsScrolled(rect.bottom <= 80);
      } else {
        setIsScrolled(window.scrollY > 450);
      }
    };

    checkScroll();
    window.addEventListener("scroll", checkScroll, { passive: true });
    return () => window.removeEventListener("scroll", checkScroll);
  }, [isHome]);

  const navLinks = [
    { name: "Home", href: "/" },
    { name: "Catalog", href: "/products" },
    { name: "Flash Deals", href: "/#deals" },
  ];

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      router.push(`/products?search=${encodeURIComponent(searchQuery.trim())}`);
      setShowSearch(false);
      setMobileMenuOpen(false);
    }
  };

  const isAuthenticated = Boolean(user && accessToken);
  const totalCartItems = mounted && isAuthenticated ? getTotalItems() : 0;

  const handleCartClick = () => {
    if (!isAuthenticated) {
      router.push("/login?redirect=/cart");
      return;
    }
    openDrawer();
  };

  return (
    <header className="fixed top-4 left-0 right-0 z-40 px-4 sm:px-6 w-full pointer-events-none">
      <div
        className={`max-w-6xl mx-auto rounded-full px-5 py-1 flex items-center justify-between pointer-events-auto transition-all duration-300 ${
          isTransparent
            ? "bg-white/15 backdrop-blur-md border border-white/20 shadow-[0_8px_30px_rgba(0,0,0,0.15)]"
            : "bg-white/95 backdrop-blur-md border border-slate-200/70 shadow-[0_8px_30px_rgb(0,0,0,0.06)]"
        }`}
      >
        {/* Brand Logo */}
        <Link href="/" className="flex items-center ">
          <Image
            src="/Logo.png"
            alt="GOshop"
            width={200}
            height={50}
            priority
            className="h-7 w-auto object-contain"
          />
        </Link>

        {/* Desktop Navigation Links */}
        <nav className="hidden md:flex items-center gap-1 text-sm font-medium">
          {navLinks.map((link) => {
            const isActive =
              link.href === "/"
                ? pathname === "/"
                : pathname.startsWith(link.href);
            return (
              <Link
                key={link.name}
                href={link.href}
                className={`px-3.5 py-1.5 rounded-full transition-all duration-200 ${
                  isTransparent
                    ? isActive
                      ? "bg-white/20 text-white font-semibold backdrop-blur-xs border border-white/25"
                      : "text-white/80 hover:text-white hover:bg-white/10"
                    : isActive
                      ? "bg-[#E3F2FD] text-[#0D47A1] font-semibold"
                      : "text-slate-600 hover:text-[#0D1B2A] hover:bg-slate-100/80"
                }`}
              >
                {link.name}
              </Link>
            );
          })}
        </nav>

        {/* Right Section: Search, Cart, Account */}
        <div className="flex items-center gap-2">
          {/* Quick Search Toggle / Form */}
          <div className="relative">
            {showSearch ? (
              <form
                onSubmit={handleSearchSubmit}
                className="flex items-center absolute right-0 top-1/2 -translate-y-1/2 bg-white rounded-full shadow-lg border border-slate-200/80 px-3 py-1.5 w-64 z-10"
              >
                <Search className="w-4 h-4 text-slate-400 shrink-0 mr-2" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Search products..."
                  className="w-full text-xs text-[#0D1B2A] bg-transparent outline-none"
                  autoFocus
                />
                <button
                  type="button"
                  onClick={() => setShowSearch(false)}
                  className="p-0.5 text-slate-400 hover:text-slate-600 ml-1"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </form>
            ) : (
              <button
                onClick={() => setShowSearch(true)}
                className={`w-9 h-9 rounded-full flex items-center justify-center transition-colors ${
                  isTransparent
                    ? "text-white/90 hover:text-white hover:bg-white/15"
                    : "text-slate-600 hover:text-[#0D1B2A] hover:bg-slate-100"
                }`}
                aria-label="Search"
              >
                <Search className="w-4 h-4" />
              </button>
            )}
          </div>

          {/* Notifications Trigger */}
          <NotificationPopover isTransparent={isTransparent} />

          {/* Cart Trigger */}
          <button
            onClick={handleCartClick}
            className={`relative w-9 h-9 rounded-full flex items-center justify-center transition-colors cursor-pointer ${
              isTransparent
                ? "text-white hover:bg-white/15"
                : "text-slate-700 hover:bg-[#E3F2FD] hover:text-[#0D47A1]"
            }`}
            aria-label="Shopping Cart"
          >
            <ShoppingCart className="w-4 h-4" />
            {totalCartItems > 0 && (
              <span className="absolute -top-1 -right-1 min-w-[18px] h-[18px] px-1 bg-[#2196F3] text-white text-[10px] font-bold rounded-full flex items-center justify-center shadow-xs">
                {totalCartItems}
              </span>
            )}
          </button>

          {/* Auth Button */}
          {user ? (
            <Link
              href="/account"
              className={`flex items-center gap-1.5 pl-2 pr-3 py-1 rounded-full transition-colors text-xs font-semibold ${
                isTransparent
                  ? "bg-white/20 hover:bg-white/30 text-white border border-white/25 backdrop-blur-xs"
                  : "bg-slate-100 hover:bg-slate-200 text-slate-700"
              }`}
            >
              <div
                className={`w-6 h-6 rounded-full flex items-center justify-center text-[10px] ${
                  isTransparent
                    ? "bg-white text-[#0D47A1] font-bold"
                    : "bg-[#0D47A1] text-white"
                }`}
              >
                {user.fullName ? user.fullName[0].toUpperCase() : "U"}
              </div>
              <span className="hidden sm:inline max-w-[80px] truncate">
                {user.fullName || user.email.split("@")[0]}
              </span>
            </Link>
          ) : (
            <Link
              href="/login"
              className={`hidden sm:inline-flex items-center gap-1 px-3.5 py-1.5 rounded-full text-xs font-semibold transition-colors shadow-xs ${
                isTransparent
                  ? "bg-white/20 hover:bg-white/30 text-white border border-white/25 backdrop-blur-xs"
                  : "bg-[#0D47A1] text-white hover:bg-[#2196F3]"
              }`}
            >
              <User className="w-3.5 h-3.5" />
              <span>Sign In</span>
            </Link>
          )}

          {/* Mobile Menu Hamburger */}
          <button
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            className={`md:hidden w-9 h-9 rounded-full flex items-center justify-center transition-colors ${
              isTransparent
                ? "text-white hover:bg-white/15"
                : "text-slate-700 hover:bg-slate-100"
            }`}
            aria-label="Toggle navigation menu"
          >
            {mobileMenuOpen ? (
              <X className="w-5 h-5" />
            ) : (
              <Menu className="w-5 h-5" />
            )}
          </button>
        </div>
      </div>

      {/* Mobile Menu Dropdown */}
      {mobileMenuOpen && (
        <div className="md:hidden mt-2 max-w-6xl mx-auto rounded-2xl bg-white/95 backdrop-blur-md border border-slate-200/80 shadow-xl p-4 pointer-events-auto space-y-2">
          {/* Mobile Search */}
          <form onSubmit={handleSearchSubmit} className="relative mb-3">
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search products..."
              className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-100 text-sm text-[#0D1B2A] outline-none border border-transparent focus:border-[#2196F3]"
            />
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          </form>

          {/* Links */}
          <nav className="flex flex-col space-y-1">
            {navLinks.map((link) => (
              <Link
                key={link.name}
                href={link.href}
                onClick={() => setMobileMenuOpen(false)}
                className="px-3 py-2 rounded-xl text-sm font-medium text-slate-700 hover:bg-[#E3F2FD] hover:text-[#0D47A1] transition-colors"
              >
                {link.name}
              </Link>
            ))}
            {!user && (
              <Link
                href="/login"
                onClick={() => setMobileMenuOpen(false)}
                className="mt-2 w-full inline-flex items-center justify-center gap-2 py-2.5 rounded-xl bg-[#0D47A1] text-white text-sm font-semibold hover:bg-[#2196F3] transition-colors"
              >
                <User className="w-4 h-4" />
                <span>Sign In / Register</span>
              </Link>
            )}
          </nav>
        </div>
      )}
    </header>
  );
}
