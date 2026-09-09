import React from 'react'
import Link from 'next/link'
import type { Category } from '@/lib/api-server'
import {
  Gamepad2,
  Tv,
  Smartphone,
  Watch,
  Headphones,
  Laptop,
  ShoppingBag,
  ArrowRight,
} from 'lucide-react'

interface CategoryShowcaseProps {
  categories: Category[]
}

export function CategoryShowcase({ categories }: CategoryShowcaseProps) {
  // Helper to pick a fitting icon based on category name or slug
  const getCategoryIcon = (name: string) => {
    const lower = name.toLowerCase()
    if (lower.includes('game')) return Gamepad2
    if (lower.includes('electronic') || lower.includes('tv')) return Tv
    if (lower.includes('phone') || lower.includes('mobile')) return Smartphone
    if (lower.includes('watch')) return Watch
    if (lower.includes('audio') || lower.includes('headphone')) return Headphones
    if (lower.includes('computer') || lower.includes('laptop')) return Laptop
    return ShoppingBag
  }

  return (
    <section id="categories" className="py-16 bg-[#FAFBFC]">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-8 gap-4">
          <div>
            <div className="inline-flex items-center gap-1 text-xs font-bold text-[#2196F3] uppercase tracking-wider mb-2">
              Browse Categories
            </div>
            <h2 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
              Shop by Department
            </h2>
          </div>
          <Link
            href="/products"
            className="inline-flex items-center gap-1.5 text-sm font-semibold text-[#0D47A1] hover:text-[#2196F3] transition-colors group"
          >
            <span>View All Categories</span>
            <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-1" />
          </Link>
        </div>

        {categories.length === 0 ? (
          <div className="p-8 text-center bg-white rounded-2xl border border-slate-200/60 text-slate-500 text-sm">
            No categories available at the moment.
          </div>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
            {categories.map((cat) => {
              const Icon = getCategoryIcon(cat.category_name)
              return (
                <Link
                  key={cat.uuid}
                  href={`/products?category=${encodeURIComponent(cat.slug || cat.uuid)}`}
                  className="group p-5 rounded-2xl bg-white border border-slate-200/60 hover:border-[#2196F3]/50 shadow-xs hover:shadow-lg transition-all duration-200 flex flex-col items-center text-center"
                >
                  <div className="w-14 h-14 rounded-2xl bg-[#E3F2FD] group-hover:bg-[#2196F3] text-[#0D47A1] group-hover:text-white flex items-center justify-center transition-colors mb-3 shadow-xs">
                    <Icon className="w-7 h-7" />
                  </div>
                  <h3 className="text-sm font-bold text-[#0D1B2A] group-hover:text-[#2196F3] transition-colors truncate w-full">
                    {cat.category_name}
                  </h3>
                  {cat.description && (
                    <p className="text-[11px] text-slate-400 mt-1 line-clamp-1">
                      {cat.description}
                    </p>
                  )}
                </Link>
              )
            })}
          </div>
        )}
      </div>
    </section>
  )
}
