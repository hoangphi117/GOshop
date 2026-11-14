import { test } from 'node:test'
import assert from 'node:assert/strict'

// The shelf gate, copied verbatim from recommendation-section.tsx. Kept here
// instead of exported for a test: the rule is two functions, and a unit-only
// export would be the tail wagging the module.
function isPersonalSource(source) {
  return source === 'personalized' || source === 'personalized_session'
}

function usable(shelf, allowed) {
  if (!shelf) return null
  if (!allowed && isPersonalSource(shelf.source)) return null
  return shelf
}

function currentShelf(shelf, initial) {
  if (shelf && shelf.items.length > 0) return shelf
  if (initial && initial.items.length > 0) return initial
  return null
}

// Mirrors the call site: the SSR shelf is only offered to an allowed device.
function shown(data, initialShelf, allowed) {
  return usable(currentShelf(data, allowed ? initialShelf : null), allowed)
}

const personal = { source: 'personalized', items: [1, 2, 3] }

test('a blocked device never paints the server-rendered personal shelf', () => {
  // The SSR pass ran with the anon cookie, so `initialShelf` is personal even
  // though the client fetch has not answered yet. This is the logout case:
  // before the fix this returned the personal shelf, and `usable` then dropped
  // it to a skeleton that never resolved, because TanStack treats initialData
  // as the answer and stops fetching.
  assert.equal(shown(undefined, personal, false), null)
  assert.equal(shown(personal, personal, false), null)
})

test('a blocked device prefers the crowd answer the moment it lands', () => {
  // Crowd is shorter than the personal row on purpose: a populated stale shelf
  // beating a real answer is exactly how the personal row survived a logout.
  const short = { source: 'orders', items: [4, 5] }
  assert.equal(shown(short, personal, false), short)
})

test('an allowed device keeps the personal shelf even if the refetch is empty', () => {
  assert.equal(shown(undefined, personal, true), personal)
})

test('an unknown source is treated as crowd, not as personal', () => {
  const trending = { source: 'trending', items: [7] }
  assert.equal(shown(trending, null, false), trending)
})
