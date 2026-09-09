'use client'

import React, { useState, useEffect } from 'react'
import Link from 'next/link'
import type { Banner } from '@/lib/api-server'
import { getImageUrl } from '@/lib/utils'
import {
  ArrowRight,
  Sparkles,
  ShieldCheck,
  Zap,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react'

interface HeroBannerProps {
  banners: Banner[]
}

export function HeroBanner({ banners }: HeroBannerProps) {
  const [currentSlide, setCurrentSlide] = useState(0)

  // Auto rotate banner if multiple exist
  useEffect(() => {
    if (banners.length <= 1) return
    const timer = setInterval(() => {
      setCurrentSlide((prev) => (prev + 1) % banners.length)
    }, 5000)
    return () => clearInterval(timer)
  }, [banners.length])

  const hasCustomBanners = banners && banners.length > 0
  const activeBanner = hasCustomBanners ? banners[currentSlide] : null

  return (
    <section className="relative overflow-hidden bg-gradient-to-br from-[#0D47A1] via-[#1565C0] to-[#2196F3] text-white pt-24 pb-16 sm:pt-32 sm:pb-24">
      {/* Decorative background glow circles */}
      <div className="absolute top-0 -left-20 w-96 h-96 bg-white/10 rounded-full blur-3xl pointer-events-none" />
      <div className="absolute -bottom-20 -right-20 w-96 h-96 bg-[#90CAF9]/20 rounded-full blur-3xl pointer-events-none" />

      <div className="max-w-6xl mx-auto px-4 sm:px-6 relative z-10">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-10 items-center">
          {/* Text Content */}
          <div className="lg:col-span-7 space-y-6">
            <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-white/15 backdrop-blur-md border border-white/20 text-xs font-semibold text-[#E3F2FD]">
              <Sparkles className="w-3.5 h-3.5 text-[#90CAF9]" />
              <span>Official Mega Sale 2026 • Up to 50% Off</span>
            </div>

            <h1 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold tracking-tight text-white leading-[1.15]">
              Shop Smart. <br />
              <span className="text-transparent bg-clip-text bg-gradient-to-r from-white via-[#E3F2FD] to-[#90CAF9]">
                Live Better.
              </span>
            </h1>

            <p className="text-base sm:text-lg text-[#E3F2FD]/90 max-w-xl font-normal leading-relaxed">
              Explore authentic high-tech electronics, trending accessories, and premium
              lifestyle goods with lightning fast delivery and guaranteed satisfaction.
            </p>

            <div className="flex flex-wrap items-center gap-3 pt-2">
              <Link
                href="/products"
                className="inline-flex items-center gap-2 px-6 py-3.5 rounded-full bg-white text-[#0D47A1] font-bold text-sm hover:bg-[#E3F2FD] transition-all shadow-lg hover:shadow-xl hover:scale-[1.02] active:scale-[0.98]"
              >
                <span>Shop All Products</span>
                <ArrowRight className="w-4 h-4 text-[#2196F3]" />
              </Link>

              <Link
                href="/#deals"
                className="inline-flex items-center gap-2 px-6 py-3.5 rounded-full bg-white/15 backdrop-blur-md border border-white/25 text-white font-semibold text-sm hover:bg-white/25 transition-all"
              >
                <Zap className="w-4 h-4 text-[#90CAF9]" />
                <span>View Flash Deals</span>
              </Link>
            </div>

            {/* Micro Trust Stats */}
            <div className="pt-6 grid grid-cols-3 gap-4 border-t border-white/15">
              <div>
                <p className="text-xl sm:text-2xl font-black text-white">100%</p>
                <p className="text-xs text-[#90CAF9] mt-0.5">Authentic Products</p>
              </div>
              <div>
                <p className="text-xl sm:text-2xl font-black text-white">Fast</p>
                <p className="text-xs text-[#90CAF9] mt-0.5">GHN Express Shipping</p>
              </div>
              <div>
                <p className="text-xl sm:text-2xl font-black text-white">24/7</p>
                <p className="text-xs text-[#90CAF9] mt-0.5">Dedicated Support</p>
              </div>
            </div>
          </div>

          {/* Right Visual / Banner Slide */}
          <div className="lg:col-span-5 relative">
            <div className="relative mx-auto max-w-md lg:max-w-none rounded-3xl overflow-hidden bg-white/10 backdrop-blur-md border border-white/20 shadow-2xl p-2 sm:p-3 group">
              {hasCustomBanners && activeBanner ? (
                <div className="relative aspect-4/3 rounded-2xl overflow-hidden bg-slate-900">
                  <img
                    src={getImageUrl(activeBanner.image_url)}
                    alt={activeBanner.title || 'GOshop Promotion'}
                    className="w-full h-full object-cover transition-transform duration-500 group-hover:scale-105"
                  />
                  {activeBanner.title && (
                    <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 via-black/40 to-transparent p-4">
                      <p className="text-sm font-bold text-white">
                        {activeBanner.title}
                      </p>
                    </div>
                  )}

                  {/* Slider controls if multiple banners */}
                  {banners.length > 1 && (
                    <div className="absolute bottom-3 right-3 flex items-center gap-1.5 z-10">
                      <button
                        onClick={() =>
                          setCurrentSlide(
                            (prev) =>
                              (prev - 1 + banners.length) % banners.length
                          )
                        }
                        className="w-7 h-7 rounded-full bg-black/50 text-white flex items-center justify-center hover:bg-black/80 transition-colors"
                        aria-label="Previous slide"
                      >
                        <ChevronLeft className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() =>
                          setCurrentSlide((prev) => (prev + 1) % banners.length)
                        }
                        className="w-7 h-7 rounded-full bg-black/50 text-white flex items-center justify-center hover:bg-black/80 transition-colors"
                        aria-label="Next slide"
                      >
                        <ChevronRight className="w-4 h-4" />
                      </button>
                    </div>
                  )}
                </div>
              ) : (
                /* Fallback Hero Card Visual */
                <div className="relative aspect-4/3 rounded-2xl overflow-hidden bg-gradient-to-tr from-[#0D47A1] to-[#1976D2] p-6 flex flex-col justify-between border border-white/20">
                  <div className="flex justify-between items-start">
                    <span className="px-3 py-1 rounded-full bg-white/20 text-xs font-bold tracking-wide uppercase text-white">
                      Featured Tech
                    </span>
                    <ShieldCheck className="w-6 h-6 text-[#90CAF9]" />
                  </div>

                  <div className="space-y-2">
                    <p className="text-xs font-semibold text-[#90CAF9] uppercase tracking-wider">
                      Special Edition
                    </p>
                    <h3 className="text-2xl font-bold text-white">
                      Next-Gen Electronics & Gaming Hub
                    </h3>
                    <p className="text-xs text-[#E3F2FD]/80">
                      High performance devices with verified warranties.
                    </p>
                  </div>

                  <div className="pt-2">
                    <Link
                      href="/products"
                      className="inline-flex items-center gap-1.5 text-xs font-bold text-white underline underline-offset-4 hover:text-[#90CAF9] transition-colors"
                    >
                      Browse collection <ArrowRight className="w-3.5 h-3.5" />
                    </Link>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
