/**
 * Device intent: should this browser be shown a shelf built from its history?
 *
 * Why a flag and not a cookie clear. The anon cookie (`goshop_anon_id`) is the
 * only thing tying this device to its search history, and a visitor who signs
 * out must not lose that history — signing back in has to bring it back. So
 * logout keeps the cookie and instead marks the device "not mine right now".
 *
 * localStorage, not sessionStorage: the point outlives the tab. A shared
 * machine must not show the previous person's picks to whoever opens it next.
 * Signing in clears it, because at that point the history really is the
 * account's again.
 */

const KEY = 'goshop_device_intent'

function store(): Storage | null {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage
  } catch {
    // Safari in private mode and partitioned iframes throw on access. No flag
    // means the shelf stays personal — the honest read, since nothing was
    // proven otherwise.
    return null
  }
}

/** True when this device's browsing history may be used to rank a shelf. */
export function deviceIntentAllowed(): boolean {
  // A flag that cannot be read is not a block: blocking on a storage error
  // would make signed-in visitors lose their personal shelf in privacy mode.
  return store()?.getItem(KEY) !== 'blocked'
}

export function blockDeviceIntent() {
  try {
    store()?.setItem(KEY, 'blocked')
  } catch {
    // Storage full or disabled: the shelf stays personal, which is the same
    // answer as a failed read.
  }
  bump()
}

export function clearDeviceIntent() {
  try {
    store()?.removeItem(KEY)
  } catch {
    // Nothing to undo.
  }
  bump()
}

/**
 * Same-page signal for a blocked device.
 *
 * The flag lives in localStorage, which the server shelf cannot read: a page
 * that is already rendered, or cached by the router, keeps the personal row it
 * painted before the logout. Bumping this counter tells those components to
 * re-read the flag without a reload.
 */
const CHANGED = 'goshop:device-intent-changed'
const STORAGE_KEY = 'goshop_device_intent'
let revision = 0

function bump() {
  revision++
  if (typeof window !== 'undefined') window.dispatchEvent(new Event(CHANGED))
}

// The revision must survive a reload, or `useSyncExternalStore` can hand React
// a getSnapshot that changes without any store notification (a refresh resets
// it to 0 while the previous value was 3), which React reports as a
// tearing/infinite-loop render. Seeding from the wall clock keeps it
// monotonically increasing across page loads.
if (typeof window !== 'undefined') revision = Date.now()

export function deviceIntentRevision(): number {
  return revision
}

/**
 * Subscribes to every allow/block change. Returns the unsubscribe function.
 *
 * Two sources, because the flag has two writers: this tab (bump, synchronous
 * with the click that logged out) and any other tab on the same origin
 * (the browser's own `storage` event). Without the second, a tab that was
 * already open kept the personal shelf after a logout elsewhere — it never saw
 * the write, so it never refetched.
 */
export function subscribeDeviceIntent(cb: () => void): () => void {
  if (typeof window === 'undefined') return () => {}
  const onStorage = (event: StorageEvent) => {
    // Ignore unrelated keys; `key === null` means the whole store was cleared,
    // which is also a change of intent.
    if (event.key !== null && event.key !== STORAGE_KEY) return
    bump()
  }
  window.addEventListener(CHANGED, cb)
  window.addEventListener('storage', onStorage)
  return () => {
    window.removeEventListener(CHANGED, cb)
    window.removeEventListener('storage', onStorage)
  }
}
