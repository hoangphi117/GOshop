"use client";

import React from "react";
import Link from "next/link";
import Image from "next/image";
import { ArrowRight, Package } from "lucide-react";
import type { Category } from "@/lib/api-server";

interface CategoryShowcaseProps {
  categories?: Category[];
}

// Fallback images for the seeded category slugs, used only when the category
// row has no image_url of its own.
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
  if (categories.length === 0) {
    return null;
  }

  return (
    <section
      id="categories"
      aria-labelledby="categories-heading"
      className="max-w-7xl mx-auto px-4 sm:px-6 mb-12 sm:mb-16"
    >
      <div className="flex flex-col sm:flex-row justify-between items-start sm:items-end gap-3 mb-6">
        <div>
          <span className="text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
            Curated Hardware & Merch
          </span>
          <h2
            id="categories-heading"
            className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5"
          >
            Categories
          </h2>
        </div>
        <Link
          href="/products"
          className="text-[#007D9C] hover:text-[#00637C] font-bold text-sm flex items-center gap-1.5 font-heading"
        >
          <span>Shop All</span>
          <ArrowRight className="w-4 h-4" aria-hidden="true" />
        </Link>
      </div>

      <div className="grid grid-cols-3 sm:grid-cols-4 lg:grid-cols-8 gap-2.5 sm:gap-3">
        {categories.map((cat) => {
          const catImage = cat.image_url || CATEGORY_IMAGE_MAP[cat.slug] || null;

          return (
            <Link
              key={cat.uuid || cat.slug}
              href={`/products?category=${cat.slug}`}
              className="group bg-white rounded-md border border-slate-200/90 hover:border-[#007D9C] hover:shadow-md transition-all overflow-hidden shadow-xs flex flex-col"
              title={cat.description || cat.category_name}
            >
              <div className="relative w-full aspect-square bg-gradient-to-br from-[#E6F7FB] to-slate-50 overflow-hidden border-b border-slate-100 flex items-center justify-center">
                {catImage ? (
                  <Image
                    src={catImage}
                    alt=""
                    fill
                    sizes="(max-width: 640px) 33vw, (max-width: 1024px) 25vw, 150px"
                    className="object-cover"
                  />
                ) : (
                  <Package
                    className="w-6 h-6 text-[#007D9C]/40"
                    aria-hidden="true"
                  />
                )}
              </div>

              <h3 className="px-2 py-2 text-[11px] sm:text-xs font-bold text-slate-800 text-center leading-tight line-clamp-2 min-h-[2.5rem] font-heading group-hover:text-[#007D9C]">
                {cat.category_name}
              </h3>
            </Link>
          );
        })}
      </div>
    </section>
  );
}
