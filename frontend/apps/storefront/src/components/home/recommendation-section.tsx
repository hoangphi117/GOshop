'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { authFetch } from '@/lib/api'
import type { Product } from '@/lib/api-server'
import { ProductCard } from '@/components/product/product-card'

/**
 * The shelf shape returned by GET /api/v1/recommendations/personalized.
 *
 * source is the server's own label: "personalized" when the visitor has usable
 * history, otherwise the fallback's name ("orders" / "fallback_rating"). The
 * heading must follow it — a promoted best-seller shelf is never presented as
 * a personal one.
 */
interface Shelf {
  source: string
  items: Product[]
}

const options = { staleTime: 60_000, refetchOnWindowFocus: false }

async function fetchShelf(limit: number): Promise<Shelf | null> {
  const res = await authFetch(`/api/v1/recommendations/personalized?limit=${limit}`)
  if (!res.ok) return null
  const json = await res.json()
  const data = json?.data
  if (!data || !Array.isArray(data.items)) return null
  return { source: data.source ?? '', items: data.items }
}

/**
 * The currently showing items: the API answered, or else the server-rendered
 * shelf. A populated shelf always beats the fallback, otherwise the personalized
 * one that arrives later would be replaced by generic best sellers when it
 * happens to be shorter than the fallback.
 */
function currentShelf(shelf: Shelf | null | undefined, initial: Shelf | null): Shelf | null {
  if (shelf && shelf.items.length > 0) return shelf
  if (initial && initial.items.length > 0) return initial
  return null
}

function SkeletonCard() {
  return (
    <div className="w-[220px] shrink-0 snap-start rounded-md bg-white border border-slate-200/90 p-4 animate-pulse">
      <div className="aspect-square w-full rounded-md bg-slate-100 mb-3" />
      <div className="h-3 w-1/3 bg-slate-100 rounded mb-2" />
      <div className="h-4 w-full bg-slate-100 rounded mb-1.5" />
      <div className="h-4 w-2/3 bg-slate-100 rounded mb-4" />
      <div className="h-5 w-1/2 bg-slate-100 rounded" />
    </div>
  )
}

interface RecommendationSectionProps {
  /** Server-rendered shelf, so the first paint is never empty. */
  initialShelf?: Shelf | null
  limit?: number
}

export function RecommendationSection({
  initialShelf = null,
  limit = 12,
}: RecommendationSectionProps) {
  // The anonymous cookie is HttpOnly and minted by middleware, so the SSR pass
  // already personalizes; this refetch only catches a profile that changed
  // after the server rendered (e.g. the previous page recorded a view).
  const [mounted, setMounted] = useState(false)
  useEffect(() => setMounted(true), [])

  const { data, isError } = useQuery({
    queryKey: ['recommendations', 'personalized', limit],
    queryFn: () => fetchShelf(limit),
    initialData: initialShelf ?? undefined,
    enabled: mounted,
    ...options,
  })

  // Keep the last good shelf on screen. A failed or empty refetch must not
  // blank a section that already rendered — that is what made the shelf vanish
  // on reload.
  const shelf = currentShelf(data, initialShelf)
  const items = shelf?.items?.slice(0, limit) ?? []
  const personalized = shelf?.source === 'personalized'

  if (items.length === 0) {
    if (isError) return null
    return (
      <section className="max-w-7xl mx-auto px-4 sm:px-6 mb-16">
        <div className="flex gap-4 overflow-hidden">
          {Array.from({ length: 4 }).map((_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      </section>
    )
  }

  return (
    <section className="max-w-7xl mx-auto px-4 sm:px-6 mb-16">
      <div className="flex items-end justify-between gap-4 mb-6">
        <div>
          <span className="text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
            {personalized ? 'Picked for you' : 'Shop the crowd favourites'}
          </span>
          <h2 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5">
            {personalized ? 'Recommended for you' : 'Best Sellers'}
          </h2>
          <p className="text-xs sm:text-sm text-slate-500 font-body mt-1">
            {personalized
              ? 'Based on your recent activity'
              : 'Top-rated and most bought this week'}
          </p>
        </div>
        <Link
          href="/products"
          className="shrink-0 text-xs font-semibold text-[#007D9C] hover:text-[#00657E] transition-colors"
        >
          View all →
        </Link>
      </div>

      <div className="flex gap-4 overflow-x-auto snap-x snap-mandatory pb-3 -mx-4 px-4 sm:mx-0 sm:px-0 custom-scrollbar">
        {items.map((product) => (
          <div key={product.uuid} className="w-[220px] shrink-0 snap-start sm:w-[240px]">
            <ProductCard product={product} />
          </div>
        ))}
      </div>
    </section>
  )
}
