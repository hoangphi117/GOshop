# ADR 0011 — Recommendations are a tier ladder over visitor identity

- **Status:** Accepted
- **Date:** 2026-11
- **Scope:** recommendations, analytics (event producer), storefront
- **Code:** `backend/gomod/services/recommendations`, `frontend/apps/storefront/src/lib/{shelf-labels,device-intent,event-tracker}.ts`, `frontend/apps/storefront/src/components/home/recommendation-section.tsx`

## Context

The homepage shelf had one question and one answer: does this visitor have a 90-day
affinity profile (yes → "Recommended for you", no → "Best Sellers"). Two things about that
were wrong, and one of them made the feature look broken rather than absent.

**The identity was half-recorded.** Events carried `anon_id` alone, or `user_id` alone once
signed in, with nothing joining the two trails. A shopper who browsed for a week and then
signed in saw the shelf reset to best sellers, and signing out handed the next anon cookie
a stranger's shelf. There was no way to recover the link either, because the schema had no
identity column and the recommendations service only reads the analytics mirror.

**The gap between "empty" and "rich" was the whole ladder.** A visitor with two product
views has a profile too thin to mean anything, and was served the 30-day popularity rollup
— a list rebuilt at boot, identical for everyone, and labelled with the same heading a
personalized visitor sees.

## Decision

**One ladder, five tiers, each one labelled by its own `source` string.** The caller walks
it in order and stops at the first tier that reports it answered:

| Tier | `source` | Requires | Heading |
|---|---|---|---|
| profile | `personalized` | affinity depth ≥ `sessionThreshold` (3) | Recommended for you |
| session | `personalized_session` | ≥ 1 product in the latest browsing pass | Recommended for you |
| trending | `trending` | ≥ `TrendingMinimum` (2) rows sold in 24h | Trending today |
| best sellers | `orders` | non-empty 30-day rollup | Best Sellers |
| ratings | `fallback_rating` | always | Best Sellers |

**Identity is bridged from the event log, not a link table.** `visitorPredicate`
(`affinity_repository.go`) matches either an id directly, or any id of the *other* kind
that ever appeared on a row carrying this one. A row with both `user_id` and `anon_id` is
proof those were the same person at that moment, and it is the only place that proof
exists. Emitted as plain column predicates so the remote indexes stay usable.

**The session tier is a depth threshold, not an emptiness check.** Both tiers read the same
90-day events, so a one-product profile is just the current pass, staler. At three products
the profile averages more than one intent and becomes the better answer.

**Trending reads the event mirror, not the popularity rollup.** The rollup is a 30-day
aggregate rebuilt at boot and cannot answer "today". Trending sums purchased units
(`props.quantity`, coalescing to 1) from `analytics_mirror.user_events` over a one-day
window, and reports `false` below two rows so a quiet day falls through to the all-time
list instead of a two-item "trend".

**Logout keeps the anon cookie and blocks the shelf client-side.** The cookie is the only
thing tying the device to its search history; flushing it loses history the user asked to
keep. Instead `localStorage['goshop_device_intent']` is set to `blocked` on logout and
cleared on any sign-in. The flag is unreadable during SSR, so both shelf surfaces read it
after mount and, when blocked, refetch without the cookie — the service then answers with
its crowd fallback and names it, which is the honest shelf for a signed-out device.

**Every non-personal tier renders under a crowd heading.** The storefront maps
`source → labels` in one module (`lib/shelf-labels.ts`); an unknown source falls back to
the crowd copy rather than the personal one, because claiming a row was picked for you is
the overclaim.

## Consequences

**Positive**

- Each tier is honest about itself: the heading always matches the signal that produced it.
- The identity bridge is invisible to every caller — `visitorPredicate` is one string, and
  a signed-in shopper keeps the trail they built before signing in.
- A new tier is a source constant, a label row, and a branch in the handler; the two
  frontends read the same map, so they cannot disagree again.
- Signing out on a shared machine stops showing that person's picks without destroying
  their history.

**Trade-offs**

- The identity bridge is two extra index-only reads per shelf. Marked `ponytail` in the
  predicate: materialise it when `user_events` crosses ~50M rows or the p95 plan shows it.
- Trending is eventually consistent through the event mirror, so a purchase shows up in
  the row one ingest flush later (up to ~5s).
- The device-intent block depends on the browser: with storage disabled the shelf stays
  personal. Treating a failed read as a block would cost signed-in visitors their shelf in
  privacy mode, which is the worse error.
- A signed-out visitor still has an anon cookie the server can use, so the block lives in
  the client. Any new shelf surface must read `deviceIntentAllowed()` or it will leak.

## Rejected alternatives

- **Flush the anon cookie on logout** — the user rejected it: signing back in would lose
  their search history, which is the only thing the cookie exists to carry.
- **A `user_identity` link table written on login** — a second writer, a backfill, and a
  new failure mode (a missed login link loses the trail) to replace a subquery that reads
  the same evidence the events already contain. Revisit only at the scale noted above.
- **Trigger the session tier on "profile is empty"** — an empty profile is rare (both tiers
  read the same events) and a one-product profile already produces a centroid, so the
  stated trigger almost never fired.
- **Server-side trending window on the popularity rollup** — the rollup is rebuilt at boot
  over 30 days; adding a second window means a second rollup, not a second query.
- **Count views as trending demand** — it made the row a duplicate of whatever the last few
  shoppers opened, which is what the personal tiers are for.
- **Clear `localStorage` / cookies wholesale on logout** — deletes the cart, theme and
  search-history inputs that are not this decision's to discard.
