/**
 * Shelf labelling, shared by the homepage carousel and the "See all" page.
 *
 * The server tells us where a shelf came from (`source`) and that must decide
 * the wording. The point of this module is that the mapping lives in one place:
 * when the homepage and the full-list page disagreed, a session shelf read as
 * "Recommended for you" on one and "Best Sellers" on the other for the same
 * visitor.
 *
 * `personalized_session` is deliberately grouped with `personalized`. It is
 * still a personal shelf — it is built from what this visitor is looking at
 * right now — and labelling it as a crowd favourite would be wrong in the other
 * direction: the row really is theirs. The subtitle is what tells the truth
 * about the signal behind it.
 */

export interface ShelfLabels {
  /** Small uppercase kicker above the heading. */
  eyebrow: string
  heading: string
  /** Short description under the heading. */
  subtitle: string
  /** Breadcrumb tail and document title. */
  short: string
  /** True when the row was built from this visitor's own behaviour. */
  personalized: boolean
}

const PERSONALIZED: ShelfLabels = {
  eyebrow: 'Picked for you',
  heading: 'Recommended for you',
  subtitle: 'Based on your recent activity',
  short: 'Recommended',
  personalized: true,
}

const SESSION: ShelfLabels = {
  eyebrow: 'Picked for you',
  heading: 'Recommended for you',
  subtitle: 'Based on what you are browsing now',
  short: 'Recommended',
  personalized: true,
}

const BEST_SELLERS: ShelfLabels = {
  eyebrow: 'Shop the crowd favourites',
  heading: 'Best Sellers',
  subtitle: 'Top-rated and most bought this week',
  short: 'Best sellers',
  personalized: false,
}

/**
 * Trending is the first non-personal tier: it is still built from other
 * people's history, so it must not borrow "Recommended for you". Same eyebrow
 * as best sellers on purpose — both are crowd rows and the heading carries the
 * difference.
 */
const TRENDING: ShelfLabels = {
  eyebrow: 'Shop the crowd favourites',
  heading: 'Trending today',
  subtitle: 'Most bought in the last 24 hours',
  short: 'Trending today',
  personalized: false,
}

const BY_SOURCE: Record<string, ShelfLabels> = {
  personalized: PERSONALIZED,
  personalized_session: SESSION,
  trending: TRENDING,
  orders: BEST_SELLERS,
  fallback_rating: BEST_SELLERS,
}

/**
 * Labels for a shelf source. An unknown source falls back to the crowd copy:
 * claiming a row was picked for you is an overclaim, and the shelf's own
 * `source` field is the only evidence either way.
 */
export function shelfLabels(source: string | null | undefined): ShelfLabels {
  if (!source) return BEST_SELLERS
  return BY_SOURCE[source] ?? BEST_SELLERS
}
