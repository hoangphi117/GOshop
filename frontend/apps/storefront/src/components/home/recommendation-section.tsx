'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import useEmblaCarousel from 'embla-carousel-react'
import Autoplay from 'embla-carousel-autoplay'
import { ChevronLeft, ChevronRight } from 'lucide-react'
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

const options = {
  staleTime: 60_000,
  refetchOnWindowFocus: false,
}

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
    <div className="shrink-0 w-[55%] max-w-[210px] sm:w-[30%] md:w-[22%] lg:w-[17%] rounded-md bg-white border border-slate-200/90 p-3 animate-pulse">
      <div className="aspect-square w-full rounded-md bg-slate-100 mb-2.5" />
      <div className="h-2.5 w-1/3 bg-slate-100 rounded mb-1.5" />
      <div className="h-3 w-full bg-slate-100 rounded mb-1" />
      <div className="h-3 w-2/3 bg-slate-100 rounded mb-2" />
      <div className="h-4 w-1/2 bg-slate-100 rounded" />
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

  // Autoplay plugin instance must be stable across renders.
  const [emblaRef, emblaApi] = useEmblaCarousel(
    { align: 'start', containScroll: 'trimSnaps', dragFree: false },
    [
      Autoplay({
        delay: 5000,
        stopOnInteraction: false,
        stopOnMouseEnter: true,
      }),
    ],
  )
  const [canPrev, setCanPrev] = useState(false)
  const [canNext, setCanNext] = useState(false)

  const syncButtons = useCallback((api = emblaApi) => {
    if (!api) return
    setCanPrev(api.canScrollPrev())
    setCanNext(api.canScrollNext())
  }, [emblaApi])

  useEffect(() => {
    if (!emblaApi) return
    const sync = () => syncButtons(emblaApi)
    emblaApi.on('select', sync)
    emblaApi.on('reInit', sync)
    // Read the initial position after the first layout pass.
    requestAnimationFrame(sync)
  }, [emblaApi, syncButtons])

  if (items.length === 0) {
    if (isError) return null
    return (
      <section className="max-w-7xl mx-auto px-4 sm:px-6 mb-16">
        <div className="flex gap-4 overflow-hidden">
          {Array.from({ length: 6 }).map((_, i) => (
            <SkeletonCard key={i} />
          ))}
        </div>
      </section>
    )
  }

  return (
    <section
      aria-labelledby="recommended-heading"
      className="max-w-7xl mx-auto px-4 sm:px-6 mb-16"
    >
      <div className="flex items-end justify-between gap-4 mb-6">
        <div>
          <span className="text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
            {personalized ? 'Picked for you' : 'Shop the crowd favourites'}
          </span>
          <h2
            id="recommended-heading"
            className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5"
          >
            {personalized ? 'Recommended for you' : 'Best Sellers'}
          </h2>
          <p className="text-xs sm:text-sm text-slate-500 font-body mt-1">
            {personalized
              ? 'Based on your recent activity'
              : 'Top-rated and most bought this week'}
          </p>
        </div>

        <Link
          href="/recommendations"
          className="shrink-0 hidden sm:inline-flex items-center gap-1 text-xs font-semibold text-[#007D9C] hover:text-[#00657E] transition-colors"
        >
          See all
          <ChevronRight className="w-3.5 h-3.5" aria-hidden="true" />
        </Link>
      </div>

      <div className="relative">
        <button
          type="button"
          onClick={() => emblaApi?.scrollPrev()}
          disabled={!canPrev}
          aria-label="Scroll recommendations left"
          className="hidden sm:flex absolute left-0 top-1/2 -translate-y-1/2 -translate-x-3 lg:-translate-x-5 z-20 w-10 h-10 rounded-full bg-white border border-slate-200 shadow-md text-slate-600 hover:border-[#007D9C] hover:text-[#007D9C] disabled:opacity-0 disabled:pointer-events-none items-center justify-center transition-all cursor-pointer"
        >
          <ChevronLeft className="w-5 h-5" aria-hidden="true" />
        </button>

        {/* embla viewport: overflow hidden, no scrollbar, CSS-driven transform */}
        <div ref={emblaRef} className="overflow-hidden -mx-4 px-4 sm:mx-0 sm:px-0">
          <div className="flex gap-4 touch-pan-y">
            {items.map((product) => (
              <div
                key={product.uuid}
                className="w-[55%] max-w-[210px] shrink-0 sm:w-[30%] md:w-[22%] lg:w-[17%]"
              >
                <ProductCard product={product} />
              </div>
            ))}
          </div>
        </div>

        <button
          type="button"
          onClick={() => emblaApi?.scrollNext()}
          disabled={!canNext}
          aria-label="Scroll recommendations right"
          className="hidden sm:flex absolute right-0 top-1/2 -translate-y-1/2 translate-x-3 lg:translate-x-5 z-20 w-10 h-10 rounded-full bg-white border border-slate-200 shadow-md text-slate-600 hover:border-[#007D9C] hover:text-[#007D9C] disabled:opacity-0 disabled:pointer-events-none items-center justify-center transition-all cursor-pointer"
        >
          <ChevronRight className="w-5 h-5" aria-hidden="true" />
        </button>
      </div>
    </section>
  )
}
