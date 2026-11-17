'use client'

import { useEffect } from 'react'
import { installEventTracking, track } from '@/lib/event-tracker'
import { useRecentlyViewedStore } from '@/stores/recently-viewed-store'

/**
 * The PDP is a cached Server Component, so the view is reported here on the
 * client. `key` changes only when the visitor moves to another product, which
 * keeps a re-render (e.g. router prefetch) from counting the same view twice.
 */
export function ProductViewTracker({ productUuid }: { productUuid: string }) {
  useEffect(() => {
    installEventTracking()
  }, [])

  useEffect(() => {
    if (!productUuid) return
    track('view_item', { product_uuid: productUuid })
    // Local history for the "Recently Viewed" shelf; ids only, resolved
    // against the catalog on render.
    useRecentlyViewedStore.getState().record(productUuid)
  }, [productUuid])

  return null
}
