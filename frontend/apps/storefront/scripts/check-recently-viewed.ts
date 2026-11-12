// Recently-viewed store check — ids only, newest first, de-duplicated.
// Run from the storefront package:
//   npx tsx scripts/check-recently-viewed.ts
// No framework: the store is plain zustand + localStorage.

export {}

const mem = new Map<string, string>()
;(globalThis as unknown as { window: unknown }).window = {
  localStorage: {
    getItem: (k: string) => mem.get(k) ?? null,
    setItem: (k: string, v: string) => void mem.set(k, v),
  },
}

const { useRecentlyViewedStore, hydrateRecentlyViewed } = await import(
  '../src/stores/recently-viewed-store'
)

const record = useRecentlyViewedStore.getState().record

record('a')
record('b')
record('a') // revisiting must move it to the front, not duplicate it

let entries = useRecentlyViewedStore.getState().entries
console.assert(entries.length === 2, 'expected 2 entries, got', entries.length)
console.assert(entries[0].uuid === 'a', 'expected "a" first, got', entries[0].uuid)
console.assert(typeof entries[0].viewedAt === 'number', 'expected a timestamp')

for (let i = 0; i < 30; i++) record(`p${i}`)
entries = useRecentlyViewedStore.getState().entries
console.assert(entries.length === 20, 'expected the cap at 20, got', entries.length)

hydrateRecentlyViewed()
console.assert(
  useRecentlyViewedStore.getState().entries.length === 20,
  'hydration must reload the persisted list'
)

console.log('recently-viewed store: OK')
