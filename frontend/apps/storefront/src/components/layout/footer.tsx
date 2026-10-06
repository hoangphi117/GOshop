"use client";

import React, { useState } from "react";
import Image from "next/image";
import Link from "next/link";
import {
  Store,
  ShieldCheck,
  Truck,
  CreditCard,
  Headphones,
  Mail,
  Phone,
  MapPin,
  ArrowRight,
  Sparkles,
  Gift,
  CheckCircle2,
  Lock,
  Globe,
  Flame,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";

export function Footer() {
  return (
    <footer className="mt-auto bg-[#071F2C] text-[#B0CDD9] border-t border-[#00384D] font-body">
      {/* 1. The 4 Trust Pillars Tier */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 py-10 border-b border-[#00384D]">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          <div className="flex items-start gap-4 p-5 rounded-2xl bg-white/[0.03] border border-white/10 hover:border-[#00ADD8]/40 transition-colors">
            <div className="w-12 h-12 rounded-xl bg-[#00ADD8]/15 border border-[#00ADD8]/30 flex items-center justify-center text-[#00ADD8] shrink-0">
              <ShieldCheck className="w-6 h-6" />
            </div>
            <div>
              <h4 className="text-sm font-bold text-white font-heading">
                100% Official Authentic
              </h4>
              <p className="text-xs text-[#B0CDD9] mt-1 leading-relaxed">
                Direct licensed merchandise & verified high-spec quality guaranteed.
              </p>
            </div>
          </div>

          <div className="flex items-start gap-4 p-5 rounded-2xl bg-white/[0.03] border border-white/10 hover:border-[#00ADD8]/40 transition-colors">
            <div className="w-12 h-12 rounded-xl bg-[#00ADD8]/15 border border-[#00ADD8]/30 flex items-center justify-center text-[#00ADD8] shrink-0">
              <Truck className="w-6 h-6" />
            </div>
            <div>
              <h4 className="text-sm font-bold text-white font-heading">
                2-Hour Express Delivery
              </h4>
              <p className="text-xs text-[#B0CDD9] mt-1 leading-relaxed">
                Rapid intra-city courier service across major metro hubs.
              </p>
            </div>
          </div>

          <div className="flex items-start gap-4 p-5 rounded-2xl bg-white/[0.03] border border-white/10 hover:border-[#00ADD8]/40 transition-colors">
            <div className="w-12 h-12 rounded-xl bg-[#F26522]/15 border border-[#F26522]/30 flex items-center justify-center text-[#F26522] shrink-0">
              <CreditCard className="w-6 h-6" />
            </div>
            <div>
              <h4 className="text-sm font-bold text-white font-heading">
                30-Day Easy Returns
              </h4>
              <p className="text-xs text-[#B0CDD9] mt-1 leading-relaxed">
                Hassle-free sizing exchanges & unconditional full refund protection.
              </p>
            </div>
          </div>

          <div className="flex items-start gap-4 p-5 rounded-2xl bg-white/[0.03] border border-white/10 hover:border-[#00ADD8]/40 transition-colors">
            <div className="w-12 h-12 rounded-xl bg-[#00ADD8]/15 border border-[#00ADD8]/30 flex items-center justify-center text-[#00ADD8] shrink-0">
              <Headphones className="w-6 h-6" />
            </div>
            <div>
              <h4 className="text-sm font-bold text-white font-heading">
                24/7 Gamer & Dev Support
              </h4>
              <p className="text-xs text-[#B0CDD9] mt-1 leading-relaxed">
                Real-time Discord & live chat support run by software engineers.
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* 2. Main Footer Directory (5-Column Layout) */}
      <div className="max-w-7xl mx-auto px-4 sm:px-6 py-14">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-12 gap-8 lg:gap-10 pb-12 border-b border-[#00384D]">
          {/* Column 1: Brand Info & Social (Col 1-5) */}
          <div className="lg:col-span-4 space-y-4">
            <Link
              href="/"
              className="inline-block bg-white/95 rounded-xl px-2.5 py-1 shadow-xs"
            >
              <Image
                src="/Logo.svg"
                alt="GOshop Logo"
                width={150}
                height={38}
                className="h-8 w-auto object-contain"
              />
            </Link>
            <p className="text-xs text-[#B0CDD9] leading-relaxed max-w-sm font-body">
              The ultimate merchandise hub for Go developers and gaming enthusiasts. Precision engineered collectibles, lifestyle apparel, and custom workspace setups.
            </p>
            {/* Social Badges */}
            <div className="flex items-center gap-2.5 pt-2">
              <a
                href="https://github.com"
                target="_blank"
                rel="noreferrer"
                className="w-8 h-8 rounded-lg bg-white/10 hover:bg-[#00ADD8] hover:text-[#071F2C] flex items-center justify-center text-white text-xs font-bold transition-all"
                aria-label="GitHub"
              >
                GH
              </a>
              <a
                href="https://discord.com"
                target="_blank"
                rel="noreferrer"
                className="w-8 h-8 rounded-lg bg-white/10 hover:bg-[#00ADD8] hover:text-[#071F2C] flex items-center justify-center text-white text-xs font-bold transition-all"
                aria-label="Discord"
              >
                DC
              </a>
              <a
                href="https://x.com"
                target="_blank"
                rel="noreferrer"
                className="w-8 h-8 rounded-lg bg-white/10 hover:bg-[#00ADD8] hover:text-[#071F2C] flex items-center justify-center text-white text-xs font-bold transition-all"
                aria-label="Twitter X"
              >
                X
              </a>
              <a
                href="https://youtube.com"
                target="_blank"
                rel="noreferrer"
                className="w-8 h-8 rounded-lg bg-white/10 hover:bg-[#00ADD8] hover:text-[#071F2C] flex items-center justify-center text-white text-xs font-bold transition-all"
                aria-label="YouTube"
              >
                YT
              </a>
            </div>
            <div className="space-y-2 text-xs text-[#B0CDD9] pt-2">
              <div className="flex items-center gap-2">
                <MapPin className="w-3.5 h-3.5 text-[#00ADD8] shrink-0" />
                <span>District 1, Ho Chi Minh City, Vietnam</span>
              </div>
              <div className="flex items-center gap-2">
                <Mail className="w-3.5 h-3.5 text-[#00ADD8] shrink-0" />
                <span>support@goshop.com</span>
              </div>
              <div className="flex items-center gap-2">
                <Phone className="w-3.5 h-3.5 text-[#00ADD8] shrink-0" />
                <span>1900-6868 (8:00 - 21:00 Everyday)</span>
              </div>
            </div>
          </div>

          {/* Column 2: Shop Categories */}
          <div className="lg:col-span-2 space-y-3">
            <h5 className="text-xs font-bold uppercase tracking-wider text-white font-heading">
              Shop Categories
            </h5>
            <ul className="space-y-2 text-xs text-[#B0CDD9]">
              <li>
                <Link
                  href="/products?category=plushies"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Boss Plushies
                </Link>
              </li>
              <li>
                <Link
                  href="/products?category=apparel"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Developer & Graphic Tees
                </Link>
              </li>
              <li>
                <Link
                  href="/products?category=figures"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Gaming Figurines & Mecha
                </Link>
              </li>
              <li>
                <Link
                  href="/products?category=accessories"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Desk Mats & Keycaps
                </Link>
              </li>
              <li>
                <Link
                  href="/#deals"
                  className="hover:text-[#F26522] transition-colors flex items-center gap-1 font-bold text-[#F26522]"
                >
                  <Flame className="w-3 h-3 text-[#F26522]" />
                  <span>Flash Deals</span>
                </Link>
              </li>
            </ul>
          </div>

          {/* Column 3: Customer Care */}
          <div className="lg:col-span-2 space-y-3">
            <h5 className="text-xs font-bold uppercase tracking-wider text-white font-heading">
              Customer Care
            </h5>
            <ul className="space-y-2 text-xs text-[#B0CDD9]">
              <li>
                <Link
                  href="/account/orders"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Track Your Order
                </Link>
              </li>
              <li>
                <Link
                  href="/cart"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Shopping Bag
                </Link>
              </li>
              <li>
                <Link
                  href="/account/returns"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Return & Exchanges
                </Link>
              </li>
              <li>
                <Link
                  href="/shipping-policy"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Shipping Policy (GHN)
                </Link>
              </li>
              <li>
                <Link
                  href="/faq"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Help Center & FAQ
                </Link>
              </li>
            </ul>
          </div>

          {/* Column 4: Corporate & Legal */}
          <div className="lg:col-span-2 space-y-3">
            <h5 className="text-xs font-bold uppercase tracking-wider text-white font-heading">
              Corporate & Legal
            </h5>
            <ul className="space-y-2 text-xs text-[#B0CDD9]">
              <li>
                <Link
                  href="/about"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  About GOshop
                </Link>
              </li>
              <li>
                <Link
                  href="/contact"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Contact Support
                </Link>
              </li>
              <li>
                <Link
                  href="/terms"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Terms of Service
                </Link>
              </li>
              <li>
                <Link
                  href="/privacy"
                  className="hover:text-[#E0F7FC] transition-colors"
                >
                  Privacy Policy
                </Link>
              </li>
            </ul>
          </div>

          {/* Column 5: Payment Badges */}
          <div className="lg:col-span-2 space-y-3">
            <h5 className="text-xs font-bold uppercase tracking-wider text-white font-heading">
              Accepted Payments
            </h5>
            <div className="flex flex-wrap gap-1.5 text-xs">
              <span className="px-2.5 py-1 rounded-md bg-white text-[#0F172A] font-bold text-[11px] shadow-xs">
                MoMo
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white text-[#0F172A] font-bold text-[11px] shadow-xs">
                ZaloPay
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white text-[#0F172A] font-bold text-[11px] shadow-xs">
                VNPay
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white text-[#0F172A] font-bold text-[11px] shadow-xs">
                COD
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white text-[#0F172A] font-bold text-[11px] shadow-xs">
                Visa
              </span>
              <span className="px-2.5 py-1 rounded-md bg-white text-[#0F172A] font-bold text-[11px] shadow-xs">
                Mastercard
              </span>
            </div>
            <div className="pt-2 flex items-center gap-1.5 text-[11px] text-emerald-400 font-medium">
              <Lock className="w-3.5 h-3.5" />
              <span>256-Bit SSL Encrypted Checkout</span>
            </div>
          </div>
        </div>

        {/* 3. Bottom Legal & Operational Bar */}
        <div className="pt-6 flex flex-col md:flex-row items-center justify-between text-xs text-[#8BAAB7] gap-4 font-body">
          <p>© {new Date().getFullYear()} GOshop Gaming & GoLang Gear. All rights reserved. Designed for high performance.</p>
          <div className="flex items-center gap-4 text-xs">
            <span className="flex items-center gap-1.5 text-emerald-400 font-semibold">
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
              <span>All APIs Operational (99.99%)</span>
            </span>
            <span>•</span>
            <span>Latency: 14ms</span>
          </div>
        </div>
      </div>
    </footer>
  );
}
