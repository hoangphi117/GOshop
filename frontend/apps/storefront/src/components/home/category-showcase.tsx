"use client";

import React from "react";
import Link from "next/link";
import { ArrowRight, Layers } from "lucide-react";
import type { Category } from "@/lib/api-server";

interface CategoryShowcaseProps {
  categories?: Category[];
}

// Fallback high-fidelity image mapping for categories if image_url is not set
const CATEGORY_IMAGE_MAP: Record<string, string> = {
  "smartphones-tablets":
    "https://images.unsplash.com/photo-1561154464-82e9adf32764?w=800",
  "laptops-computers":
    "https://images.unsplash.com/photo-1588872657578-7efd1f1555ed?w=800",
  "audio-sound":
    "https://images.unsplash.com/photo-1590658268037-6bf12165a8df?w=800",
  "smartwatches-wearables":
    "https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=800",
  "gaming-consoles":
    "https://images.unsplash.com/photo-1606813907291-d86efa9b94db?w=800",
  "cameras-photography":
    "https://images.unsplash.com/photo-1502920917128-1aa500764cbd?w=800",
  "smart-home-appliances":
    "https://images.unsplash.com/photo-1585771724684-38269d6639fd?w=800",
  "accessories-cables":
    "https://images.unsplash.com/photo-1609081219090-a6d8173087ec?w=800",
};

export function CategoryShowcase({ categories = [] }: CategoryShowcaseProps) {
  // Take top 4 categories from backend
  const displayCategories = categories.slice(0, 4);

  if (displayCategories.length === 0) {
    return null;
  }

  return (
    <section
      id="categories"
      aria-label="Collections"
      className="max-w-7xl mx-auto px-4 sm:px-6 mb-12 sm:mb-16"
    >
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-end gap-3 mb-8">
        <div>
          <span className="text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
            Curated Hardware & Merch
          </span>
          <h2 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5">
            Explore by Department
          </h2>
        </div>
        <Link
          href="/products"
          className="text-[#007D9C] hover:text-[#00637C] font-bold text-sm flex items-center gap-1.5 font-heading"
        >
          <span>View All Departments</span>
          <ArrowRight className="w-4 h-4" />
        </Link>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        {displayCategories.map((cat, idx) => {
          const catImage =
            cat.image_url ||
            CATEGORY_IMAGE_MAP[cat.slug] ||
            "https://images.unsplash.com/photo-1550009158-9ebf69173e03?w=800";
          const tagLabel =
            idx === 0
              ? "Trending"
              : idx === 1
                ? "Popular"
                : idx === 2
                  ? "Top Rated"
                  : "Official";
          const tagBg =
            idx === 0
              ? "bg-[#FFE2D5] text-[#F26522]"
              : idx === 1
                ? "bg-[#E0F7FC] text-[#007D9C]"
                : idx === 2
                  ? "bg-[#BAEAFF] text-[#004D62]"
                  : "bg-[#E6F7FB] text-[#007D9C]";

          return (
            <Link
              key={cat.uuid || cat.slug}
              href={`/products?category=${cat.slug}`}
              className="group bg-white rounded-md p-5 border border-slate-200/90 hover:border-[#007D9C] hover:shadow-xl cursor-pointer overflow-hidden shadow-xs flex flex-col justify-between"
            >
              <div>
                <div className="w-full h-44 rounded-md bg-slate-50 overflow-hidden relative mb-4 border border-slate-100">
                  <span
                    className={`absolute top-3 left-3 text-[11px] font-bold px-2.5 py-0.5 rounded-full z-10 shadow-xs font-heading ${tagBg}`}
                  >
                    {tagLabel}
                  </span>
                  <img
                    src={catImage}
                    alt={cat.category_name}
                    className="w-full h-full object-cover"
                  />
                </div>

                <h3 className="text-lg font-bold text-slate-900 font-heading truncate">
                  {cat.category_name}
                </h3>
                <p className="text-xs text-slate-500 mt-1 font-body line-clamp-2">
                  {cat.description ||
                    "High-performance tech gear & official merch"}
                </p>
              </div>

              <div className="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between text-[#007D9C] font-bold text-xs font-heading">
                <span>Shop Collection</span>
                <ArrowRight className="w-4 h-4" />
              </div>
            </Link>
          );
        })}
      </div>
    </section>
  );
}
