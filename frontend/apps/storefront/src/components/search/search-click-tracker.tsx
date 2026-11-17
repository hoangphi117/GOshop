'use client'

import React from 'react'
import { installEventTracking, track } from '@/lib/event-tracker'

/**
 * Records that a result was opened from a search result page.
 *
 * `search_click` is the half of search affinity that carries a product: the
 * `search` event only says a keyword ran, so without this the recommender can
 * see what a shopper typed but never what it meant. Listeners are installed in
 * the capture phase on the container so any link inside a result card counts,
 * which keeps the card component free of tracking props.
 *
 * Only the products page mounts this, so a click from a category page or a shelf
 * is not misattributed to a query.
 */
export function SearchClickTracker({
  query,
  children,
}: {
  query: string
  children: React.ReactNode
}) {
  const ref = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    installEventTracking()
  }, [])

  React.useEffect(() => {
    const node = ref.current
    if (!node || !query) return

    const onClick = (event: MouseEvent) => {
      // Middle-click / ctrl-click still opens the product; count it too.
      const anchor = (event.target as HTMLElement | null)?.closest?.('a[href]')
      if (!anchor) return
      const href = anchor.getAttribute('href') || ''
      // Cards link to /products/<slug-or-uuid>; anything else on the page
      // (pagination, header) is not a result click.
      const match = /^\/products\/([^/?#]+)/.exec(href)
      if (!match) return
      const target = decodeURIComponent(match[1])
      // The link may carry a slug rather than a uuid. Ingest requires a uuid, so
      // only report what can be attributed; the product page's own view_item
      // still records the visit.
      track('search_click', {
        query,
        product_uuid: target,
        props: { href },
      })
    }

    node.addEventListener('click', onClick, true)
    node.addEventListener('auxclick', onClick, true)
    return () => {
      node.removeEventListener('click', onClick, true)
      node.removeEventListener('auxclick', onClick, true)
    }
  }, [query])

  return <div ref={ref}>{children}</div>
}
