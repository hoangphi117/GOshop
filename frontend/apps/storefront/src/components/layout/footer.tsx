import React from 'react'
import Link from 'next/link'
import {
  Store,
  ShieldCheck,
  Truck,
  CreditCard,
  Headphones,
  Mail,
  Phone,
  MapPin,
} from 'lucide-react'

export function Footer() {
  return (
    <footer className="mt-auto bg-[#0D47A1] text-[#E3F2FD] border-t border-[#1565C0]">
      {/* Upper Footer: Value Props */}
      <div className="max-w-6xl mx-auto px-4 sm:px-6 py-10 border-b border-white/10">
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-6">
          <div className="flex items-center gap-3">
            <div className="w-11 h-11 rounded-xl bg-white/10 flex items-center justify-center text-[#90CAF9] shrink-0">
              <Truck className="w-5 h-5" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-white">Express Delivery</h4>
              <p className="text-xs text-[#90CAF9] mt-0.5">Nationwide fast delivery</p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="w-11 h-11 rounded-xl bg-white/10 flex items-center justify-center text-[#90CAF9] shrink-0">
              <ShieldCheck className="w-5 h-5" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-white">100% Authentic</h4>
              <p className="text-xs text-[#90CAF9] mt-0.5">Direct genuine guarantee</p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="w-11 h-11 rounded-xl bg-white/10 flex items-center justify-center text-[#90CAF9] shrink-0">
              <CreditCard className="w-5 h-5" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-white">Secure Payments</h4>
              <p className="text-xs text-[#90CAF9] mt-0.5">MoMo & Cash on delivery</p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="w-11 h-11 rounded-xl bg-white/10 flex items-center justify-center text-[#90CAF9] shrink-0">
              <Headphones className="w-5 h-5" />
            </div>
            <div>
              <h4 className="text-sm font-semibold text-white">24/7 Support</h4>
              <p className="text-xs text-[#90CAF9] mt-0.5">Ready to assist anytime</p>
            </div>
          </div>
        </div>
      </div>

      {/* Main Footer Links */}
      <div className="max-w-6xl mx-auto px-4 sm:px-6 py-12">
        <div className="grid grid-cols-1 md:grid-cols-5 gap-8">
          {/* Brand Info */}
          <div className="md:col-span-2 space-y-4">
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-lg bg-[#2196F3] flex items-center justify-center text-white">
                <Store className="w-4 h-4" />
              </div>
              <span className="text-xl font-extrabold tracking-tight text-white">
                GO<span className="text-[#90CAF9]">shop</span>
              </span>
            </div>
            <p className="text-xs text-[#90CAF9] leading-relaxed max-w-sm">
              GOshop is a state-of-the-art e-commerce platform delivering high quality
              electronics, gaming gear, fashion, and lifestyle essentials with blazing fast checkout.
            </p>
            <div className="space-y-2 text-xs text-[#90CAF9]">
              <div className="flex items-center gap-2">
                <MapPin className="w-3.5 h-3.5 shrink-0 text-[#2196F3]" />
                <span>District 1, Ho Chi Minh City, Vietnam</span>
              </div>
              <div className="flex items-center gap-2">
                <Mail className="w-3.5 h-3.5 shrink-0 text-[#2196F3]" />
                <span>support@goshop.com</span>
              </div>
              <div className="flex items-center gap-2">
                <Phone className="w-3.5 h-3.5 shrink-0 text-[#2196F3]" />
                <span>1900-6868 (Mon - Sun, 8:00 - 21:00)</span>
              </div>
            </div>
          </div>

          {/* Quick Links */}
          <div>
            <h5 className="text-xs font-bold uppercase tracking-wider text-white mb-3">
              Explore
            </h5>
            <ul className="space-y-2 text-xs text-[#90CAF9]">
              <li>
                <Link href="/" className="hover:text-white transition-colors">
                  Home
                </Link>
              </li>
              <li>
                <Link href="/products" className="hover:text-white transition-colors">
                  All Products
                </Link>
              </li>
              <li>
                <Link href="/about" className="hover:text-white transition-colors">
                  About Us
                </Link>
              </li>
              <li>
                <Link href="/contact" className="hover:text-white transition-colors">
                  Contact Us
                </Link>
              </li>
            </ul>
          </div>

          {/* Customer Service */}
          <div>
            <h5 className="text-xs font-bold uppercase tracking-wider text-white mb-3">
              Customer Care
            </h5>
            <ul className="space-y-2 text-xs text-[#90CAF9]">
              <li>
                <Link href="/cart" className="hover:text-white transition-colors">
                  My Shopping Cart
                </Link>
              </li>
              <li>
                <Link href="/account" className="hover:text-white transition-colors">
                  Order Tracking
                </Link>
              </li>
              <li>
                <Link href="/faq" className="hover:text-white transition-colors">
                  Help Center & FAQ
                </Link>
              </li>
              <li>
                <Link href="/shipping-policy" className="hover:text-white transition-colors">
                  Shipping Policy
                </Link>
              </li>
              <li>
                <Link href="/return-policy" className="hover:text-white transition-colors">
                  Return & Refund Policy
                </Link>
              </li>
              <li>
                <Link href="/privacy" className="hover:text-white transition-colors">
                  Privacy Policy
                </Link>
              </li>
            </ul>
          </div>

          {/* Payment Partners */}
          <div>
            <h5 className="text-xs font-bold uppercase tracking-wider text-white mb-3">
              Payments & Shipping
            </h5>
            <div className="flex flex-wrap gap-2 text-xs">
              <span className="px-2.5 py-1 rounded-md bg-white/10 text-white font-medium">
                MoMo E-Wallet
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white/10 text-white font-medium">
                COD
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white/10 text-white font-medium">
                Visa / MC
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white/10 text-white font-medium">
                GHN Express
              </span>
            </div>
          </div>
        </div>

        {/* Bottom Copyright */}
        <div className="mt-12 pt-6 border-t border-white/10 flex flex-col sm:flex-row items-center justify-between text-xs text-[#90CAF9] gap-4">
          <p>© {new Date().getFullYear()} GOshop. All rights reserved.</p>
          <div className="flex items-center gap-4">
            <Link href="/terms" className="hover:text-white transition-colors">
              Terms of Service
            </Link>
            <span>•</span>
            <Link href="/privacy" className="hover:text-white transition-colors">
              Privacy Policy
            </Link>
            <span>•</span>
            <Link href="/shipping-policy" className="hover:text-white transition-colors">
              Shipping & Delivery
            </Link>
            <span>•</span>
            <Link href="/faq" className="hover:text-white transition-colors">
              FAQ
            </Link>
          </div>
        </div>
      </div>
    </footer>
  )
}
