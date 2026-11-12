import { API_BASE_URL, API_KEY } from './api'

export type EventType =
  | 'search'
  | 'search_click'
  | 'view_item'
  | 'add_to_cart'
  | 'purchase'

interface QueuedEvent {
  event_id: string
  event_type: EventType
  occurred_at: string
  [field: string]: unknown
}

const MAX_BATCH = 20
const FLUSH_MS = 5000

const queue: QueuedEvent[] = []
let timer: ReturnType<typeof setTimeout> | null = null

function uuid(): string {
  // crypto.randomUUID needs a secure context; localhost counts, but a plain
  // HTTP LAN dev box does not.
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16)
  })
}

function endpoint(): string {
  // Same-origin through the edge keeps the event cookie HttpOnly-compatible;
  // fall back to the configured API base for direct-to-gateway dev setups.
  return `${API_BASE_URL}/api/v1/events`
}

function send(body: string) {
  const url = endpoint()
  // Always fetch with keepalive, never sendBeacon: sendBeacon cannot set the
  // X-API-KEY header, so behind an edge that does not inject it (dev-host has
  // no nginx) the request was rejected with 400 "missing API Key". keepalive
  // survives pagehide just like sendBeacon.
  //
  // The failure branch is silent in prod but logs in dev: the gateway's 400
  // carries the reason ("missing API Key", "search requires query"), and an
  // event batch that never lands is otherwise indistinguishable from one with
  // nothing to do — which made a broken search-affinity pipeline look idle.
  void fetch(url, {
    method: 'POST',
    body,
    credentials: 'include',
    keepalive: true,
    headers: {
      'Content-Type': 'application/json',
      ...(API_KEY ? { 'X-API-KEY': API_KEY } : {}),
      'X-Client-App': 'storefront',
    },
  })
    .then((res) => {
      if (process.env.NODE_ENV !== 'production' && !res.ok) {
        console.warn('[events] ingest rejected', res.status, res.statusText)
      }
    })
    .catch((err) => {
      // Fire and forget: telemetry must never break the shopping flow.
      if (process.env.NODE_ENV !== 'production') console.warn('[events] ingest failed', err)
    })
}

export function flushEvents() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  if (queue.length === 0) return
  const batch = queue.splice(0, MAX_BATCH)
  send(JSON.stringify({ events: batch }))
  if (queue.length > 0) flushEvents()
}

/**
 * Records one interaction. Identity (anon cookie or bearer token) is attached
 * by the server, so the payload never carries a user id.
 *
 * Fields are spread flat, not nested under `payload`: the ingest DTO reads
 * `query` / `product_uuid` / `result_count` at the top level of each event,
 * and a nested object made every search fail with "search requires query".
 */
export function track(event_type: EventType, payload: Record<string, unknown> = {}) {
  if (typeof window === 'undefined') return
  queue.push({
    ...payload,
    event_id: uuid(),
    event_type,
    occurred_at: new Date().toISOString(),
  })
  if (queue.length >= MAX_BATCH) {
    flushEvents()
    return
  }
  timer ??= setTimeout(flushEvents, FLUSH_MS)
}

let installed = false

const reportedPurchases = new Set<string>()

/**
 * Records the purchase of one order exactly once, at the moment of payment
 * (GA4 semantics). Ingest requires one product_uuid per event, so a multi-item
 * order emits one event per item, all sharing the order uuid. The local guard
 * keeps a React re-render or a retry from double-counting revenue.
 */
export function trackPurchase(
  orderUuid: string,
  lines: { productUuid: string; quantity: number; price: number }[],
  opts: { paymentMethod?: string; value?: number } = {}
) {
  if (!orderUuid || reportedPurchases.has(orderUuid)) return
  reportedPurchases.add(orderUuid)
  installEventTracking()
  for (const line of lines) {
    if (!line.productUuid) continue
    track('purchase', {
      product_uuid: line.productUuid,
      props: {
        order_uuid: orderUuid,
        quantity: line.quantity,
        price: line.price,
        total_value: opts.value,
        payment_method: opts.paymentMethod,
      },
    })
  }
}

/** Wires the pagehide/visibility flush exactly once per page load. */
export function installEventTracking() {
  if (installed || typeof window === 'undefined') return
  installed = true
  window.addEventListener('pagehide', flushEvents)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flushEvents()
  })
}
