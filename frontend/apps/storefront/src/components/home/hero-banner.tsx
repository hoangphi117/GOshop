"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import Image from "next/image";
import type { Banner } from "@/lib/api-server";
import { ArrowRight, Flame, Star, Box, Sparkles } from "lucide-react";

interface HeroBannerProps {
  banners?: Banner[];
  subBanner?: Banner | null;
}

function normalizeBannerUrl(url?: string): string {
  if (!url) return "/products";
  if (url.startsWith("/category/")) {
    const slug = url.replace("/category/", "").trim();
    return `/products?category=${slug}`;
  }
  return url;
}

export function HeroBanner({ banners = [], subBanner }: HeroBannerProps) {
  const [currentSlide, setCurrentSlide] = useState(0);

  // Auto rotate banner if multiple exist
  useEffect(() => {
    if (banners.length <= 1) return;
    const timer = setInterval(() => {
      setCurrentSlide((prev) => (prev + 1) % banners.length);
    }, 6000);
    return () => clearInterval(timer);
  }, [banners.length]);

  const activeBanner = banners[currentSlide] || null;
  const bannerLink = normalizeBannerUrl(activeBanner?.link_url);

  return (
    <section
      id="hero-banner"
      aria-label="Hero Showcase"
      className="relative overflow-hidden bg-[#F8FAFC] py-8 sm:py-12 lg:py-16"
    >
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        <div className="bg-white rounded-3xl border border-slate-200/90 p-6 sm:p-10 lg:p-12 shadow-sm overflow-hidden relative">
          {/* Ambient radial glow backdrop */}
          <div className="absolute -right-20 -top-20 w-96 h-96 bg-[#00ADD8]/15 rounded-full blur-3xl pointer-events-none" />
          <div className="absolute right-1/4 -bottom-20 w-80 h-80 bg-[#007D9C]/10 rounded-full blur-3xl pointer-events-none" />
          <div className="absolute -left-20 -bottom-20 w-80 h-80 bg-[#F26522]/5 rounded-full blur-3xl pointer-events-none" />

          <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 lg:gap-12 items-center relative z-10">
            {/* Left Column Content */}
            <div className="lg:col-span-7 space-y-6">
              {/* Release Badge */}
              <div className="inline-flex items-center gap-2 bg-[#E0F7FC] text-[#007D9C] px-3.5 py-1.5 rounded-full text-xs font-bold border border-[#BAEAFF] shadow-2xs">
                <span>
                  {activeBanner?.title
                    ? "Official Promotion"
                    : "Official Release • High Performance Tech"}
                </span>
              </div>

              {/* Main Headline */}
              <h1 className="text-3xl sm:text-4xl lg:text-5xl font-extrabold text-slate-900 tracking-tight leading-[1.15] font-heading">
                {activeBanner?.title ? (
                  activeBanner.title
                ) : (
                  <>
                    Level Up Your Setup with Official{" "}
                    <span className="text-transparent bg-clip-text bg-gradient-to-r from-[#007D9C] via-[#00ADD8] to-[#005F73]">
                      GoLang
                    </span>{" "}
                    & Gaming Drops
                  </>
                )}
              </h1>

              {/* Subtext */}
              <p className="text-sm sm:text-base text-slate-600 max-w-xl leading-relaxed font-body">
                Curated high-performance hardware, mechanical keyboards,
                audiophile sound, and collectible merch designed specifically
                for developers, creators, and enthusiasts.
              </p>

              {/* CTAs */}
              <div className="flex flex-wrap items-center gap-3.5 pt-2">
                <Link
                  href={bannerLink}
                  className="bg-[#007D9C] hover:bg-[#00637C] text-white px-7 py-3.5 rounded-xl font-bold text-sm shadow-md hover:shadow-[#00ADD8]/25 transition-all flex items-center gap-2 group active:scale-95 font-heading cursor-pointer"
                >
                  <span>Explore Catalog</span>
                  <ArrowRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />
                </Link>

                <Link
                  href="/products?sort=newest"
                  className="bg-white hover:bg-slate-50 text-[#007D9C] border-2 border-[#007D9C]/80 hover:border-[#007D9C] px-6 py-3.5 rounded-xl font-bold text-sm flex items-center gap-2 font-heading cursor-pointer shadow-2xs"
                >
                  <Sparkles className="w-4 h-4 text-[#00ADD8]" />
                  <span>New Arrivals</span>
                </Link>
              </div>

              {/* Social Proof Bar */}
              <div className="pt-6 border-t border-slate-100 flex flex-wrap items-center gap-4">
                <div className="flex -space-x-2 overflow-hidden">
                  <div className="inline-block h-8 w-8 rounded-full ring-2 ring-white bg-[#BAEAFF] text-[#004D62] font-black flex items-center justify-center text-[10px]">
                    GO
                  </div>
                  <div className="inline-block h-8 w-8 rounded-full ring-2 ring-white bg-[#D0EDF5] text-[#007D9C] font-black flex items-center justify-center text-[10px]">
                    JS
                  </div>
                  <div className="inline-block h-8 w-8 rounded-full ring-2 ring-white bg-[#FFE2D5] text-[#F26522] font-black flex items-center justify-center text-[10px]">
                    RS
                  </div>
                  <div className="inline-block h-8 w-8 rounded-full ring-2 ring-white bg-slate-100 text-slate-700 font-bold flex items-center justify-center text-[10px]">
                    +10k
                  </div>
                </div>

                <div className="flex flex-col">
                  <div className="flex items-center gap-1 text-amber-500">
                    <Star className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                    <Star className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                    <Star className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                    <Star className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                    <Star className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                    <span className="text-xs font-bold text-slate-900 ml-1 font-heading">
                      4.9 / 5
                    </span>
                  </div>
                  <span className="text-[11px] text-slate-500 font-body">
                    Verified ratings across 10,000+ orders
                  </span>
                </div>
              </div>
            </div>

            {/* Right Column Showcase Pedestal */}
            <div className="lg:col-span-5 relative">
              <div className="relative bg-gradient-to-b from-[#F0F9FB] to-slate-50 rounded-2xl p-6 border border-[#D0EDF5] flex flex-col items-center justify-center shadow-inner">
                {/* Floating stat badges */}
                <div className="absolute -top-3 left-4 bg-[#F26522] text-white px-3 py-1 rounded-full font-bold text-[11px] shadow-md flex items-center gap-1 font-heading z-20">
                  <Flame className="w-3.5 h-3.5 fill-white" />
                  <span>Featured Collection</span>
                </div>

                {/* Hero Showcase Image */}
                <Link
                  href={bannerLink}
                  className="block w-full max-w-sm group/banner"
                  aria-label={`View ${activeBanner?.title || "featured collection"}`}
                >
                  <div className="relative w-full aspect-square rounded-2xl overflow-hidden shadow-xl bg-white p-3 border border-slate-200/80">
                    <Image
                      src={
                        activeBanner?.image_url ||
                        "https://images.unsplash.com/photo-1550009158-9ebf69173e03?w=800"
                      }
                      alt={activeBanner?.title || "GOshop Featured Collection"}
                      fill
                      sizes="(max-width: 1024px) 100vw, 400px"
                      priority
                      className="object-cover rounded-xl"
                    />
                  </div>

                  {/* Card Subtitle */}
                  <div className="mt-4 text-center">
                    <div className="text-sm font-bold text-slate-900 font-heading group-hover/banner:text-[#007D9C]">
                      {activeBanner?.title || "Official Tech & Merch Collection"}
                    </div>
                    <div className="text-xs font-extrabold text-[#007D9C] font-heading mt-0.5">
                      100% Genuine Guaranteed • Fast Delivery
                    </div>
                  </div>
                </Link>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
