"use client";

import React, { useState, useEffect, useCallback } from "react";
import Link from "next/link";
import Image from "next/image";
import type { Banner } from "@/lib/api-server";
import { ChevronLeft, ChevronRight } from "lucide-react";

interface HeroBannerProps {
  banners?: Banner[];
  subBanners?: Banner[];
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

const DEFAULT_MAIN_SLIDES: Partial<Banner>[] = [
  {
    uuid: "default-1",
    title: "Level Up Your Setup with Official GoLang & Gaming Drops",
    image_url:
      "https://images.unsplash.com/photo-1550009158-9ebf69173e03?w=1200&q=80",
    link_url: "/products",
  },
  {
    uuid: "default-2",
    title: "New Arrivals - Mechanical Keyboards & Audio",
    image_url:
      "https://images.unsplash.com/photo-1595225476474-87563907a212?w=1200&q=80",
    link_url: "/products?sort=newest",
  },
];

const DEFAULT_SUB_BANNERS: Partial<Banner>[] = [
  {
    uuid: "sub-1",
    title: "Flash Sale Tech Gear",
    image_url:
      "https://images.unsplash.com/photo-1527814050087-3793815479db?w=800&q=80",
    link_url: "/products?category=accessories",
  },
  {
    uuid: "sub-2",
    title: "Official Developer Merch",
    image_url:
      "https://images.unsplash.com/photo-1618366712010-f4ae9c647dcb?w=800&q=80",
    link_url: "/products?category=merch",
  },
];

export function HeroBanner({
  banners = [],
  subBanners = [],
  subBanner = null,
}: HeroBannerProps) {
  const slides = banners.length > 0 ? banners : DEFAULT_MAIN_SLIDES;

  // Fill the two right-hand slots with the real sub banners first (already
  // ordered by banner_order from the API), then the legacy single prop, and
  // only then the placeholder art. Every slot must come from the same source
  // list or the second real banner is silently replaced by a default.
  const subPool = subBanners.length > 0 ? subBanners : subBanner ? [subBanner] : [];
  const resolvedRightBanners: Partial<Banner>[] = [
    subPool[0] || DEFAULT_SUB_BANNERS[0],
    subPool[1] || DEFAULT_SUB_BANNERS[1],
  ];

  const [currentSlide, setCurrentSlide] = useState(0);
  const [isHovered, setIsHovered] = useState(false);

  const nextSlide = useCallback(() => {
    setCurrentSlide((prev) => (prev + 1) % slides.length);
  }, [slides.length]);

  const prevSlide = useCallback(() => {
    setCurrentSlide((prev) => (prev - 1 + slides.length) % slides.length);
  }, [slides.length]);

  useEffect(() => {
    if (slides.length <= 1 || isHovered) return;
    const timer = setInterval(nextSlide, 5000);
    return () => clearInterval(timer);
  }, [slides.length, isHovered, nextSlide]);

  return (
    <section
      id="hero-banner"
      aria-label="Hero Showcase"
      className="w-full bg-white py-3 sm:py-4 lg:py-6"
    >
      {/* Vùng chứa bọc ngoài chiếm full width, bên trong giới hạn max-w-7xl */}
      <div className="w-full max-w-7xl mx-auto px-4 sm:px-6">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-3">
          {/* CỘT TRÁI (8/12): SLIDER BANNER LỚN */}
          <div
            className="lg:col-span-8 relative group overflow-hidden rounded-sm bg-slate-100 aspect-[16/9] sm:aspect-[21/9] lg:aspect-auto lg:h-[320px] xl:h-[350px]"
            onMouseEnter={() => setIsHovered(true)}
            onMouseLeave={() => setIsHovered(false)}
          >
            {/* Dải trượt các slide */}
            <div
              className="flex h-full w-full transition-transform duration-500 ease-out"
              style={{ transform: `translateX(-${currentSlide * 100}%)` }}
            >
              {slides.map((slide, idx) => (
                <Link
                  key={slide.uuid || idx}
                  href={normalizeBannerUrl(slide.link_url)}
                  className="relative w-full h-full shrink-0 block"
                  aria-label={slide.title || `Slide ${idx + 1}`}
                >
                  <Image
                    src={
                      slide.image_url ||
                      "https://images.unsplash.com/photo-1550009158-9ebf69173e03?w=1200"
                    }
                    alt={slide.title || "Hero Banner"}
                    fill
                    sizes="(max-width: 1024px) 100vw, 67vw"
                    priority={idx === 0}
                    className="object-cover"
                  />
                  <div className="absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-black/40 to-transparent pointer-events-none" />
                </Link>
              ))}
            </div>

            {/* 2 Nút mũi tên điều hướng */}
            {slides.length > 1 && (
              <>
                <button
                  type="button"
                  onClick={prevSlide}
                  aria-label="Banner trước"
                  className="absolute left-0 top-1/2 -translate-y-1/2 z-20 bg-black/30 hover:bg-black/55 text-white w-8 h-14 sm:w-10 sm:h-16 flex items-center justify-center rounded-r-sm opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity cursor-pointer"
                >
                  <ChevronLeft className="w-6 h-6" />
                </button>

                <button
                  type="button"
                  onClick={nextSlide}
                  aria-label="Banner tiếp theo"
                  className="absolute right-0 top-1/2 -translate-y-1/2 z-20 bg-black/30 hover:bg-black/55 text-white w-8 h-14 sm:w-10 sm:h-16 flex items-center justify-center rounded-l-sm opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity cursor-pointer"
                >
                  <ChevronRight className="w-6 h-6" />
                </button>

                {/* Dãy chấm Indicator */}
                <div className="absolute bottom-3 left-1/2 -translate-x-1/2 z-20 flex items-center gap-2">
                  {slides.map((slide, idx) => (
                    <button
                      key={slide.uuid || idx}
                      type="button"
                      onClick={() => setCurrentSlide(idx)}
                      aria-label={`Chuyển đến slide ${idx + 1}`}
                      aria-current={idx === currentSlide}
                      className={`h-2.5 rounded-full transition-all cursor-pointer border border-white/60 ${
                        idx === currentSlide
                          ? "w-6 bg-[#00ADD8] border-[#00ADD8]"
                          : "w-2.5 bg-white/60 hover:bg-white"
                      }`}
                    />
                  ))}
                </div>
              </>
            )}
          </div>

          {/* CỘT PHẢI (4/12): 2 BANNER PHỤ XẾP CHỒNG */}
          <div className="lg:col-span-4 grid grid-cols-2 lg:grid-cols-1 lg:grid-rows-2 gap-3 lg:h-[320px] xl:h-[350px]">
            {resolvedRightBanners.map((item, idx) => (
              <Link
                key={item.uuid || idx}
                href={normalizeBannerUrl(item.link_url)}
                className="relative w-full h-32 sm:h-40 lg:h-full overflow-hidden rounded-sm bg-slate-100 block group/sub"
                aria-label={item.title || `Sub banner ${idx + 1}`}
              >
                <Image
                  src={
                    item.image_url ||
                    "https://images.unsplash.com/photo-1527814050087-3793815479db?w=800"
                  }
                  alt={item.title || `Promotional Banner ${idx + 1}`}
                  fill
                  sizes="(max-width: 1024px) 50vw, 33vw"
                  className="object-cover"
                />
              </Link>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
