import React from 'react'
import Link from 'next/link'
import {
  getBanners,
  getCategories,
  getProducts,
} from '@/lib/api-server'
import { HeroBanner } from '@/components/home/hero-banner'
import { TrustBadges } from '@/components/home/trust-badges'
import { CategoryShowcase } from '@/components/home/category-showcase'
import { ProductGrid } from '@/components/product/product-grid'
import { Sparkles, ArrowRight, Flame } from 'lucide-react'

// Revalidate home page every 60 seconds (ISR)
export const revalidate = 60

export default async function HomePage() {
  // Fetch real data concurrently from Go backend
  const [banners, categories, productsData] = await Promise.all([
    getBanners(),
    getCategories(),
    getProducts({ limit: 8 }),
  ])

  const products = productsData.products || []

  return (
    <div className="flex flex-col min-h-screen">
      {/* 1. Hero Banner with Gradient & Floating Glass */}
      <HeroBanner banners={banners} />

      {/* 2. Trust Badges in Soft Ice Blue Canvas */}
      <TrustBadges />

      {/* 3. Categories Showcase */}
      <CategoryShowcase categories={categories} />

      {/* 4. Flash Deals & Featured Products Section */}
      <section id="deals" className="py-16 bg-white border-y border-slate-100">
        <div className="max-w-6xl mx-auto px-4 sm:px-6">
          <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-8 gap-4">
            <div>
              <div className="inline-flex items-center gap-1.5 text-xs font-bold text-rose-500 uppercase tracking-wider mb-2">
                <Flame className="w-4 h-4 fill-rose-500 text-rose-500" />
                <span>Trending Highlights</span>
              </div>
              <h2 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
                Featured Products
              </h2>
            </div>
            <Link
              href="/products"
              className="inline-flex items-center gap-1.5 text-sm font-semibold text-[#0D47A1] hover:text-[#2196F3] transition-colors group"
            >
              <span>Explore All Products</span>
              <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-1" />
            </Link>
          </div>

          {/* Product Grid */}
          <ProductGrid products={products} />
        </div>
      </section>

      {/* 5. Promotional Callout Banner */}
      <section className="py-14 bg-gradient-to-r from-[#0D47A1] to-[#1565C0] text-white">
        <div className="max-w-6xl mx-auto px-4 sm:px-6">
          <div className="rounded-3xl bg-white/10 backdrop-blur-md border border-white/20 p-8 sm:p-12 flex flex-col md:flex-row items-center justify-between gap-8 shadow-xl">
            <div className="space-y-3 text-center md:text-left max-w-xl">
              <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-[#2196F3] text-white text-xs font-bold tracking-wide">
                <Sparkles className="w-3.5 h-3.5" />
                <span>Limited Time Offer</span>
              </div>
              <h3 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-white">
                Get 10% Off Your First Order
              </h3>
              <p className="text-sm text-[#E3F2FD]/85 leading-relaxed">
                Join our club today and apply code <strong className="text-white font-bold tracking-wider">WELCOME10</strong> at checkout to unlock instant discounts.
              </p>
            </div>
            <Link
              href="/products"
              className="px-8 py-3.5 rounded-full bg-white text-[#0D47A1] font-bold text-sm hover:bg-[#E3F2FD] hover:scale-105 transition-all shadow-lg shrink-0"
            >
              Claim Discount Now
            </Link>
          </div>
        </div>
      </section>
    </div>
  )
}
