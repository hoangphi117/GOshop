"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import type { Banner } from "@/lib/api-server";
import { getImageUrl } from "@/lib/utils";
import {
  ArrowRight,
  Sparkles,
  Zap,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";

interface HeroBannerProps {
  banners: Banner[];
  subBanner?: Banner | null;
}

export function HeroBanner({ banners, subBanner }: HeroBannerProps) {
  const [currentSlide, setCurrentSlide] = useState(0);

  // Auto rotate banner if multiple exist
  useEffect(() => {
    if (banners.length <= 1) return;
    const timer = setInterval(() => {
      setCurrentSlide((prev) => (prev + 1) % banners.length);
    }, 5000);
    return () => clearInterval(timer);
  }, [banners.length]);

  const hasCustomBanners = banners && banners.length > 0;
  const activeBanner = hasCustomBanners ? banners[currentSlide] : null;
  const hasSubBannerImage = Boolean(subBanner && subBanner.image_url?.trim());

  return (
    <section
      id="hero-banner"
      className="relative overflow-hidden bg-gradient-to-br from-[#0D47A1] via-[#1565C0] to-[#2196F3] text-white pt-24 pb-16 sm:pt-32 sm:pb-24"
    >
      {/* Dynamic Banner Background if available */}
      {hasCustomBanners && activeBanner ? (
        <div className="absolute inset-0 z-0">
          <img
            key={activeBanner.uuid || currentSlide}
            src={getImageUrl(activeBanner.image_url)}
            alt={activeBanner.title || "GOshop Hero Banner"}
            className="w-full h-full object-cover transition-opacity duration-700"
          />
          {/* Moderate dark gradient overlay on the left to ensure text legibility while showing original banner colors */}
          <div className="absolute inset-0 bg-gradient-to-r from-black/80 via-black/35 to-transparent" />
        </div>
      ) : (
        /* Fallback decorative glow circles when no banner exists (default blue gradient) */
        <>
          <div className="absolute top-0 -left-20 w-96 h-96 bg-white/10 rounded-full blur-3xl pointer-events-none" />
          <div className="absolute -bottom-20 -right-20 w-96 h-96 bg-[#90CAF9]/20 rounded-full blur-3xl pointer-events-none" />
        </>
      )}

      <div className="max-w-6xl mx-auto px-4 sm:px-6 relative z-10">
        <div
          className={
            hasSubBannerImage
              ? "grid grid-cols-1 lg:grid-cols-12 gap-8 lg:gap-10 items-center"
              : "max-w-3xl"
          }
        >
          {/* Left Column: Heading, intro, actions, and stats */}
          <div
            className={
              hasSubBannerImage ? "lg:col-span-7 space-y-6" : "space-y-6"
            }
          >
            {/* Heading */}
            <h1 className="text-4xl sm:text-5xl lg:text-6xl font-extrabold tracking-tight text-white leading-[1.15]">
              Level Up Your Loot. <br />
              <span className="text-transparent bg-clip-text bg-gradient-to-r from-white via-[#E3F2FD] to-[#90CAF9]">
                Plushies, Tees & Toys.
              </span>
            </h1>

            {/* Subtitle */}
            <p className="text-base sm:text-lg text-[#E3F2FD]/90 max-w-xl font-normal leading-relaxed">
              Discover handcrafted boss & character plushies, graphic printed
              gamer tees, and collectible toys from your favorite gaming worlds
              with express shipping and guaranteed authentic quality.
            </p>

            {/* Badge */}
            <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-white/15 backdrop-blur-md border border-white/20 text-xs font-semibold text-[#E3F2FD]">
              <Sparkles className="w-3.5 h-3.5 text-[#90CAF9]" />
              <span>
                {activeBanner?.title
                  ? `Featured Collection • ${activeBanner.title}`
                  : "Official Gaming Merch 2026 • Exclusive Drops"}
              </span>
            </div>

            {/* Actions */}
            <div className="flex flex-wrap items-center gap-3 pt-2">
              <Link
                href={activeBanner?.link_url || "/products"}
                className="inline-flex items-center gap-2 px-6 py-3.5 rounded-xl bg-white text-[#0D47A1] font-bold text-sm hover:bg-[#E3F2FD] transition-all shadow-lg hover:shadow-xl hover:scale-[1.02] active:scale-[0.98] cursor-pointer"
              >
                <span>
                  {activeBanner?.link_url ? "Explore Offer" : "Shop now"}
                </span>
                <ArrowRight className="w-4 h-4 text-[#2196F3]" />
              </Link>

              <Link
                href="/#deals"
                className="inline-flex items-center gap-2 px-6 py-3.5 rounded-xl bg-white/15 backdrop-blur-md border border-white/25 text-white font-semibold text-sm hover:bg-white/25 transition-all cursor-pointer"
              >
                <Zap className="w-4 h-4 text-[#90CAF9]" />
                <span>View Flash Deals</span>
              </Link>
            </div>

            {/* Micro Trust Stats */}
            <div className="pt-6 grid grid-cols-3 gap-3 max-w-lg">
              <div className="p-3.5 rounded-xl bg-black/10 backdrop-blur-xs border border-white/20 shadow-sm transition-all hover:bg-black/50">
                <p className="text-xl sm:text-2xl font-black text-white">100%</p>
                <p className="text-xs text-[#90CAF9] mt-0.5 font-medium leading-tight">
                  Authentic Game Merch
                </p>
              </div>
              <div className="p-3.5 rounded-xl bg-black/10 backdrop-blur-xs border border-white/20 shadow-sm transition-all hover:bg-black/50">
                <p className="text-xl sm:text-2xl font-black text-white">Fast</p>
                <p className="text-xs text-[#90CAF9] mt-0.5 font-medium leading-tight">
                  GHN Express Shipping
                </p>
              </div>
              <div className="p-3.5 rounded-xl bg-black/10 backdrop-blur-xs border border-white/20 shadow-sm transition-all hover:bg-black/50">
                <p className="text-xl sm:text-2xl font-black text-white">24/7</p>
                <p className="text-xs text-[#90CAF9] mt-0.5 font-medium leading-tight">
                  Gamer Support Hub
                </p>
              </div>
            </div>
          </div>

          {/* Right Column: Sub banner product image overlay (no card, transparent background, large size) */}
          {hasSubBannerImage && subBanner && (
            <div className="lg:col-span-5 flex items-center justify-center lg:justify-end relative py-4 lg:py-0 lg:translate-x-8 xl:translate-x-12">
              {/* Atmospheric dark backdrop behind sub banner */}
              <div className="absolute inset-0 m-auto w-[90%] h-[90%] bg-black/65 rounded-full blur-3xl -z-10 pointer-events-none" />
              <div className="absolute inset-2 bg-black/40 rounded-3xl blur-2xl -z-10 pointer-events-none" />

              {subBanner.link_url ? (
                <Link
                  href={subBanner.link_url}
                  className="group relative block transition-all duration-500 hover:scale-105 active:scale-98"
                >
                  <img
                    src={getImageUrl(subBanner.image_url)}
                    alt={subBanner.title || "Featured Sub Banner"}
                    className="w-auto max-w-full max-h-[360px] sm:max-h-[440px] lg:max-h-[500px] object-contain drop-shadow-[0_20px_40px_rgba(0,0,0,0.5)] transition-all duration-300 group-hover:drop-shadow-[0_25px_50px_rgba(33,150,243,0.4)]"
                  />
                </Link>
              ) : (
                <div className="relative">
                  <img
                    src={getImageUrl(subBanner.image_url)}
                    alt={subBanner.title || "Featured Sub Banner"}
                    className="w-auto max-w-full max-h-[360px] sm:max-h-[440px] lg:max-h-[500px] object-contain drop-shadow-[0_20px_40px_rgba(0,0,0,0.5)]"
                  />
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Slider controls if multiple banners exist */}
      {banners.length > 1 && (
        <div className="absolute bottom-6 right-6 sm:right-12 flex items-center gap-2 z-20">
          <button
            onClick={() =>
              setCurrentSlide(
                (prev) => (prev - 1 + banners.length) % banners.length,
              )
            }
            className="w-9 h-9 rounded-full bg-black/40 backdrop-blur-md border border-white/20 text-white flex items-center justify-center hover:bg-black/60 transition-colors shadow-lg cursor-pointer"
            aria-label="Previous slide"
          >
            <ChevronLeft className="w-5 h-5" />
          </button>
          <div className="flex items-center gap-1.5 px-1">
            {banners.map((_, idx) => (
              <button
                key={idx}
                onClick={() => setCurrentSlide(idx)}
                className={`h-2 rounded-full transition-all cursor-pointer ${
                  idx === currentSlide
                    ? "w-6 bg-white"
                    : "w-2 bg-white/40 hover:bg-white/70"
                }`}
                aria-label={`Slide ${idx + 1}`}
              />
            ))}
          </div>
          <button
            onClick={() =>
              setCurrentSlide((prev) => (prev + 1) % banners.length)
            }
            className="w-9 h-9 rounded-full bg-black/40 backdrop-blur-md border border-white/20 text-white flex items-center justify-center hover:bg-black/60 transition-colors shadow-lg cursor-pointer"
            aria-label="Next slide"
          >
            <ChevronRight className="w-5 h-5" />
          </button>
        </div>
      )}
    </section>
  );
}
