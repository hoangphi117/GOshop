'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { authFetch } from '@/lib/api'
import type { Product } from '@/lib/api-server'
import {
  hydrateRecentlyViewed,
  useRecentlyViewedStore,
} from '@/stores/recently-viewed-store'
import { ProductCard } from './product-card'

const MAX_SHELF = 8

async function fetchByUuids(uuids: string[]): Promise<Product[]> {
  if (uuids.length === 0) return []
  const res = await authFetch(
    `/api/v1/products?uuids=${uuids.map(encodeURIComponent).join(',')}`
  )
  if (!res.ok) return []
  const json = await res.json()
  const data = json?.data
  if (!Array.isArray(data)) return []

  // The batch query does not preserve the requested order, so re-sort by
  // viewing recency — the shelf reads as history, not as a random grid.
  const rank = new Map(uuids.map((uuid, i) => [uuid, i]))
  return (data as Product[])
    .filter((p) => rank.has(p.uuid))
    .sort((a, b) => (rank.get(a.uuid) ?? 0) - (rank.get(b.uuid) ?? 0))
}

interface RecentlyViewedShelfProps {
  /** Product currently open, hidden from its own shelf. */
  excludeUuid?: string
}

/** Client-only: the history lives in local storage and has no server twin. */
export function RecentlyViewedShelf({ excludeUuid }: RecentlyViewedShelfProps) {
  const [hydrated, setHydrated] = useState(false)
  const entries = useRecentlyViewedStore((s) => s.entries)

  useEffect(() => {
    hydrateRecentlyViewed()
    setHydrated(true)
  }, [])

  const uuids = entries
    .map((e) => e.uuid)
    .filter((uuid) => uuid !== excludeUuid)
    .slice(0, MAX_SHELF)

  const { data: products = [] } = useQuery({
    queryKey: ['recently-viewed', uuids],
    queryFn: () => fetchByUuids(uuids),
    enabled: hydrated && uuids.length > 0,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
  })

  if (!hydrated || products.length === 0) return null

  return (
    <section className="pt-10 border-t border-[#E2E8F0] space-y-5">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="text-lg sm:text-xl font-bold text-[#0F172A] tracking-tight">
            Recently Viewed
          </h2>
          <p className="text-xs text-[#64748B]">
            Pick up where you left off.
          </p>
        </div>
        <Link
          href="/products"
          className="shrink-0 text-xs font-semibold text-[#007D9C] hover:text-[#00657E] transition-colors"
        >
          Browse Catalog →
        </Link>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
        {products.map((product) => (
          <ProductCard key={product.uuid} product={product} />
        ))}
      </div>
    </section>
  )
}
