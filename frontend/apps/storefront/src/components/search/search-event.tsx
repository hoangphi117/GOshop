'use client'

import { useEffect, useRef } from 'react'
import { installEventTracking, track } from '@/lib/event-tracker'

/**
 * Server Components cannot reach the browser event queue, so the products page
 * mounts this with the already-resolved result count. A search is only recorded
 * when it actually returned something: a typo that found nothing is noise.
 */
export function SearchEvent({
  query,
  resultCount,
}: {
  query: string
  resultCount: number
}) {
  const reported = useRef<string | null>(null)

  useEffect(() => {
    installEventTracking()
  }, [])

  useEffect(() => {
    if (!query || resultCount <= 0) return
    // Re-running on every render would double-count a single search.
    if (reported.current === query) return
    reported.current = query
    track('search', { query, result_count: resultCount })
  }, [query, resultCount])

  return null
}
