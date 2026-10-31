import React from 'react'
import type { Metadata } from 'next'
import Link from 'next/link'
import {
  Store,
  ShieldCheck,
  Zap,
  Sparkles,
  Award,
  Users,
  Package,
  HeartHandshake,
  ArrowRight,
  TrendingUp,
} from 'lucide-react'

export const metadata: Metadata = {
  title: 'About Us — GOshop',
  description:
    'Learn about GOshop, our mission to deliver top-tier authentic gaming gear, boss plushies, premium tees, and exceptional shopping experiences.',
}

export default function AboutPage() {
  return (
    <div className="bg-[#FAFBFC] min-h-screen py-12">
      {/* Hero Section */}
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        <div className="relative rounded-3xl bg-gradient-to-br from-[#0D47A1] via-[#1565C0] to-[#00ADD8] p-8 sm:p-14 text-white overflow-hidden shadow-xl mb-16">
          {/* Subtle glow background */}
          <div className="absolute -top-24 -right-24 w-96 h-96 rounded-full bg-white/10 blur-3xl pointer-events-none" />
          <div className="absolute -bottom-24 -left-24 w-80 h-80 rounded-full bg-[#00ADD8]/20 blur-2xl pointer-events-none" />

          <div className="relative z-10 max-w-2xl space-y-4">
            <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-white/15 backdrop-blur-xs text-xs font-semibold text-white border border-white/20">
              <Sparkles className="w-3.5 h-3.5 text-[#90CAF9]" />
              <span>Our Story & Mission</span>
            </div>
            <h1 className="text-3xl sm:text-5xl font-black tracking-tight leading-tight">
              Fueling Your Passion with Authentic Quality.
            </h1>
            <p className="text-sm sm:text-base text-[#E3F2FD] leading-relaxed">
              At GOshop, we believe every fan, gamer, and enthusiast deserves authentic,
              premium gear that reflects their lifestyle. From collectible boss plushies to
              tailored graphic streetwear, we craft moments of excitement with every unboxing.
            </p>
          </div>
        </div>

        {/* Stats Grid */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 sm:gap-6 mb-16">
          {[
            { label: 'Happy Customers', value: '50,000+', icon: Users },
            { label: 'Products Delivered', value: '120,000+', icon: Package },
            { label: 'Authentic Partners', value: '100+', icon: Award },
            { label: 'Customer Satisfaction', value: '99.4%', icon: TrendingUp },
          ].map((stat) => {
            const Icon = stat.icon
            return (
              <div
                key={stat.label}
                className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs flex flex-col items-center text-center space-y-2"
              >
                <div className="w-12 h-12 rounded-xl bg-[#E3F2FD] text-[#0D47A1] flex items-center justify-center">
                  <Icon className="w-6 h-6" />
                </div>
                <div className="text-2xl sm:text-3xl font-black text-[#0D1B2A] tracking-tight">
                  {stat.value}
                </div>
                <div className="text-xs font-medium text-slate-500">{stat.label}</div>
              </div>
            )
          })}
        </div>

        {/* Mission & Vision */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-8 mb-16">
          <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-4">
            <div className="w-12 h-12 rounded-2xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center">
              <Store className="w-6 h-6" />
            </div>
            <h2 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
              Our Vision
            </h2>
            <p className="text-sm text-slate-600 leading-relaxed">
              To be the premier lifestyle e-commerce destination in Southeast Asia, connecting
              creators, brands, and fans with verified genuine merchandise, seamless one-click
              checkout, and express doorstep delivery.
            </p>
          </div>

          <div className="p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-4">
            <div className="w-12 h-12 rounded-2xl bg-emerald-50 text-emerald-600 flex items-center justify-center">
              <ShieldCheck className="w-6 h-6" />
            </div>
            <h2 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
              Our Quality Guarantee
            </h2>
            <p className="text-sm text-slate-600 leading-relaxed">
              Every item featured on GOshop is sourced directly from licensed manufacturers.
              We rigorously inspect materials, print durability, and stitching to guarantee 100%
              authenticity and satisfaction.
            </p>
          </div>
        </div>

        {/* Core Values */}
        <div className="space-y-8 mb-16">
          <div className="text-center max-w-xl mx-auto space-y-2">
            <h2 className="text-2xl sm:text-3xl font-black text-[#0D1B2A] tracking-tight">
              Core Principles We Live By
            </h2>
            <p className="text-xs sm:text-sm text-slate-500">
              The foundational values guiding our daily work, customer support, and technology decisions.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {[
              {
                title: 'Customer Obsession',
                desc: 'We place our community at the center of everything, providing 24/7 dedicated support and easy returns.',
                icon: HeartHandshake,
              },
              {
                title: 'Speed & Reliability',
                desc: 'Powered by GHN Express and optimized fulfillment to deliver your orders rapidly across all 63 provinces.',
                icon: Zap,
              },
              {
                title: 'Security & Transparency',
                desc: 'Military-grade AES-256 encryption, safe JWT sessions, and trusted MoMo sandbox integrations.',
                icon: ShieldCheck,
              },
            ].map((val) => {
              const Icon = val.icon
              return (
                <div
                  key={val.title}
                  className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-3"
                >
                  <div className="w-10 h-10 rounded-xl bg-slate-100 text-[#0D47A1] flex items-center justify-center">
                    <Icon className="w-5 h-5" />
                  </div>
                  <h3 className="text-base font-bold text-[#0D1B2A]">{val.title}</h3>
                  <p className="text-xs text-slate-500 leading-relaxed">{val.desc}</p>
                </div>
              )
            })}
          </div>
        </div>

        {/* CTA Banner */}
        <div className="rounded-3xl bg-[#0D1B2A] p-8 sm:p-12 text-center text-white space-y-5 shadow-lg">
          <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight">
            Ready to Explore the Collection?
          </h2>
          <p className="text-xs sm:text-sm text-slate-300 max-w-md mx-auto leading-relaxed">
            Discover our curated catalog of boss plushies, limited streetwear, accessories, and special coupons.
          </p>
          <div className="pt-2">
            <Link
              href="/products"
              className="inline-flex items-center gap-2 px-7 py-3 rounded-2xl bg-[#2196F3] text-white text-sm font-bold hover:bg-[#00ADD8] transition-all shadow-md active:scale-98"
            >
              <span>Start Shopping</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
          </div>
        </div>
      </div>
    </div>
  )
}
