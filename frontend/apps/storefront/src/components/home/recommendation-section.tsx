'use client'

import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import useEmblaCarousel from 'embla-carousel-react'
import Autoplay from 'embla-carousel-autoplay'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { authFetch } from '@/lib/api'
import type { Product } from '@/lib/api-server'
import {
  deviceIntentAllowed,
  deviceIntentRevision,
  subscribeDeviceIntent,
} from '@/lib/device-intent'
import { shelfLabels } from '@/lib/shelf-labels'
import { useMounted } from '@/hooks/use-mounted'
import { ProductCard } from '@/components/product/product-card'

/**
 * The shelf shape returned by GET /api/v1/recommendations/personalized.
 *
 * source is the server's own label: "personalized" when the 90-day profile was
 * deep enough to rank, "personalized_session" when it ranked the visitor's
 * current browsing pass instead, "trending" when today's sales answered, and
 * otherwise the fallback's name ("orders" / "fallback_rating"). The heading
 * follows it — see shelfLabels — because a crowd row is never presented as a
 * personal one.
 */
interface Shelf {
  source: string
  items: Product[]
}

const options = {
  staleTime: 60_000,
  refetchOnWindowFocus: false,
}

/**
 * Client fetch, used when the browser decides the shelf is its own business.
 *
 * The flag cannot be read during SSR — it lives in localStorage — so this
 * section fetches the shelf itself in that case.
 *
 * `tier=crowd` is how a blocked device asks for a crowd row while keeping its
 * anon cookie, which is deliberate: the cookie is this browser's search history
 * and must survive a logout, but a logged-out device must not be ranked by it.
 * Without the cookie the service answers the same way, which is why the flag is
 * about intent rather than about data.
 */
async function fetchShelf(limit: number, crowd: boolean): Promise<Shelf | null> {
  const query = `limit=${limit}${crowd ? '&tier=crowd' : ''}`
  const res = await authFetch(`/api/v1/recommendations/personalized?${query}`)
  if (!res.ok) return null
  const json = await res.json()
  const data = json?.data
  if (!data || !Array.isArray(data.items)) return null
  return { source: data.source ?? '', items: data.items }
}

/** True when the source names a tier that ranked this visitor's own history. */
function isPersonalSource(source: string | undefined): boolean {
  return source === 'personalized' || source === 'personalized_session'
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

/**
 * True when a shelf may be built from this device's history.
 *
 * A blocked device also ignores a personal shelf that is still in the query
 * cache: the cached copy is exactly what a logout has to stop showing, and
 * TanStack would otherwise hand it back before the crowd refetch lands.
 */
function usable(shelf: Shelf | null | undefined, allowed: boolean): Shelf | null {
  if (!shelf) return null
  if (!allowed && isPersonalSource(shelf.source)) return null
  return shelf
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
  // The anonymous cookie is HttpOnly and minted by proxy, so the SSR pass
  // already personalizes; this refetch only catches a profile that changed
  // after the server rendered (e.g. the previous page recorded a view).
  const mounted = useMounted()

  // False after a logout on this device: the cookie is still there, so an
  // honest answer has to come from the client, which can read the flag. Reading
  // it only after mount keeps the server shelf on the first paint instead of
  // rendering a different DOM than the one that was hydrated.
  //
  // Deliberately not in an effect: `mounted` flips on the same first client
  // render, so deriving the answer there avoids a second pass that would
  // briefly paint the personal row and then drop it. The revision subscribes
  // to logout/sign-in so a page that is already painted — and TanStack's cache
  // still holding the personal shelf — drops the row without a reload.
  const revision = useSyncExternalStore(
    subscribeDeviceIntent,
    deviceIntentRevision,
    deviceIntentRevision,
  )
  // One read per revision: store the flag itself, not a call, so the query key
  // and the gate agree and a blocked device refetches instead of reusing the
  // personal shelf it is still holding.
  const allowedFlag = useMemo(
    () => (mounted ? deviceIntentAllowed() : true),
    // revision is the re-read trigger; the store is the source.
    [mounted, revision], // eslint-disable-line react-hooks/exhaustive-deps
  )

  const { data } = useQuery({
    // The tier is part of the key: a blocked device and a guest reader ask the
    // server different questions now (`tier=crowd` vs. the device's own
    // history), so their answers must not share a cache entry.
    queryKey: ['recommendations', 'personalized', limit, allowedFlag ? 'personal' : 'crowd'],
    queryFn: () => fetchShelf(limit, !allowedFlag),
    // A blocked device must not seed from the server shelf: `initialData` makes
    // the query `success` and, with `staleTime`, *fresh*, so the crowd fetch
    // never ran and the request never appeared in the network panel. Only an
    // allowed device gets the SSR row as its starting point.
    initialData: allowedFlag ? initialShelf ?? undefined : undefined,
    enabled: mounted,
    ...options,
  })

  // Keep the last good shelf on screen. A failed or empty refetch must not
  // blank a section that already rendered — that is what made the shelf vanish
  // on reload.
  //
  // The fallback shelf is only offered to an allowed device. It was rendered
  // during an SSR pass that had this browser's anon cookie, so for a blocked
  // device it is the personal row under another name — and `currentShelf` would
  // hand it back as soon as the personal refetch is the one in flight. With no
  // fallback the skeleton shows until the crowd answer lands, which is the
  // honest state for a device that just signed out.
  const fallback = allowedFlag ? initialShelf : null
  const shelf = usable(currentShelf(data, fallback), allowedFlag)
  const items = shelf?.items?.slice(0, limit) ?? []
  const labels = shelfLabels(shelf?.source)

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
    // A blocked device paints skeletons for exactly as long as its client
    // refetch takes, then the crowd row replaces them — the same as any first
    // load. Worst case (the refetch fails) the section renders nothing, which
    // is still honest, unlike the personal row it just replaced.
    //
    // The heading is *not* withheld. Without it the section reads as a generic
    // grey loader, which is exactly how a blocked shelf looked like a broken
    // one. It shows the crowd heading instead: that is what is about to arrive
    // from a blocked device, and it is already true on the first paint of a
    // visitor with no history.
    const pending = shelfLabels('orders')
    return (
      <section aria-labelledby="recommended-heading" className="max-w-7xl mx-auto px-4 sm:px-6 mb-16">
        <div className="flex items-end justify-between gap-4 mb-6">
          <div>
            <span className="text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
              {pending.eyebrow}
            </span>
            <h2
              id="recommended-heading"
              className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5"
            >
              {pending.heading}
            </h2>
            <p className="text-sm text-slate-500 mt-1 font-body">{pending.subtitle}</p>
          </div>
        </div>
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
            {labels.eyebrow}
          </span>
          <h2
            id="recommended-heading"
            className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5"
          >
            {labels.heading}
          </h2>
          <p className="text-xs sm:text-sm text-slate-500 font-body mt-1">
            {labels.subtitle}
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
