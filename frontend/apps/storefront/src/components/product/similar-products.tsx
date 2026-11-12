'use client'

import { useQuery } from '@tanstack/react-query'
import { authFetch } from '@/lib/api'
import type { Product } from '@/lib/api-server'
import { ProductGrid } from './product-grid'

interface SimilarShelf {
  source: string
  items: Product[]
}

async function fetchSimilar(uuid: string, limit: number): Promise<SimilarShelf | null> {
  const res = await authFetch(
    `/api/v1/products/similar?uuid=${encodeURIComponent(uuid)}&limit=${limit}`
  )
  if (!res.ok) return null
  const json = await res.json()
  const data = json?.data
  if (!data || !Array.isArray(data.items)) return null
  return { source: data.source ?? '', items: data.items }
}

/**
 * Vector nearest-neighbours for the open product. The server already excludes
 * the product itself and anything out of stock, so no client-side filtering is
 * repeated here.
 */
export function SimilarProducts({ productUuid }: { productUuid: string }) {
  const { data } = useQuery({
    queryKey: ['products', 'similar', productUuid],
    queryFn: () => fetchSimilar(productUuid, 8),
    staleTime: 5 * 60_000,
    refetchOnWindowFocus: false,
  })

  const items = (data?.items ?? []).filter((p) => p.uuid !== productUuid)
  if (items.length === 0) return null

  return (
    <section className="pt-10 border-t border-[#E2E8F0] space-y-5">
      <div>
        <h2 className="text-lg sm:text-xl font-bold text-[#0F172A] tracking-tight">
          You may also like
        </h2>
        <p className="text-xs text-[#64748B]">
          Similar products other shoppers compared with this one.
        </p>
      </div>
      <ProductGrid products={items} />
    </section>
  )
}
