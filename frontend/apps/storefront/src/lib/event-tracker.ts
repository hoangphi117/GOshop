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
  payload: Record<string, unknown>
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
  if (typeof navigator !== 'undefined' && navigator.sendBeacon) {
    // sendBeacon survives pagehide. The API key is injected server-side by the
    // edge, so no custom header is needed on this path.
    const blob = new Blob([body], { type: 'application/json' })
    if (navigator.sendBeacon(url, blob)) return
  }
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
  }).catch(() => {
    // Fire and forget: telemetry must never break the shopping flow.
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
 */
export function track(event_type: EventType, payload: Record<string, unknown> = {}) {
  if (typeof window === 'undefined') return
  queue.push({
    event_id: uuid(),
    event_type,
    occurred_at: new Date().toISOString(),
    payload,
  })
  if (queue.length >= MAX_BATCH) {
    flushEvents()
    return
  }
  timer ??= setTimeout(flushEvents, FLUSH_MS)
}

let installed = false

/** Wires the pagehide/visibility flush exactly once per page load. */
export function installEventTracking() {
  if (installed || typeof window === 'undefined') return
  installed = true
  window.addEventListener('pagehide', flushEvents)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flushEvents()
  })
}
