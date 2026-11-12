'use client'

import { create } from 'zustand'

const STORAGE_KEY = 'goshop_recently_viewed'
const MAX_ITEMS = 20

export interface RecentlyViewedEntry {
  uuid: string
  viewedAt: number
}

interface RecentlyViewedState {
  entries: RecentlyViewedEntry[]
  record: (uuid: string) => void
}

/**
 * UUIDs and timestamps only — never a copy of the product. Cards are resolved
 * against the catalog on render so a price or stock change is never served
 * stale from local storage.
 */
function load(): RecentlyViewedEntry[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []
    return parsed
      .filter(
        (e): e is RecentlyViewedEntry =>
          typeof e === 'object' &&
          e !== null &&
          typeof (e as RecentlyViewedEntry).uuid === 'string' &&
          typeof (e as RecentlyViewedEntry).viewedAt === 'number'
      )
      .slice(0, MAX_ITEMS)
  } catch {
    return []
  }
}

function persist(entries: RecentlyViewedEntry[]) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(entries))
  } catch {
    // A full or disabled storage must not break browsing.
  }
}

export const useRecentlyViewedStore = create<RecentlyViewedState>((set) => ({
  entries: [],
  record: (uuid) =>
    set((state) => {
      if (!uuid) return state
      const entries = [
        { uuid, viewedAt: Date.now() },
        ...state.entries.filter((e) => e.uuid !== uuid),
      ].slice(0, MAX_ITEMS)
      persist(entries)
      return { entries }
    }),
}))

/** Hydrates the store from local storage; call once on mount. */
export function hydrateRecentlyViewed() {
  useRecentlyViewedStore.setState({ entries: load() })
}

export function recentlyViewedUuids(limit = MAX_ITEMS): string[] {
  return useRecentlyViewedStore
    .getState()
    .entries.slice(0, limit)
    .map((e) => e.uuid)
}

export const RECENTLY_VIEWED_MAX = MAX_ITEMS
