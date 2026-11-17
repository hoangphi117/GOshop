"use client";

import React, { useEffect, useState, useSyncExternalStore } from "react";
import Link from "next/link";
import { ArrowLeft, Sparkles, Star } from "lucide-react";
import { authFetch } from "@/lib/api";
import type { RecommendationShelf } from "@/lib/api-server";
import {
  deviceIntentAllowed,
  deviceIntentRevision,
  subscribeDeviceIntent,
} from "@/lib/device-intent";
import { shelfLabels } from "@/lib/shelf-labels";
import { ProductGrid } from "@/components/product/product-grid";

interface RecommendationsViewProps {
  /** Server-rendered shelf, ranked from this request's visitor cookie. */
  initialShelf: RecommendationShelf | null;
}

// The service caps a shelf at 50 items, so that is the whole list.
const SHELF_LIMIT = 50;

/** True when the source names a tier that ranked this visitor's own history. */
function isPersonalSource(source: string | undefined): boolean {
  return source === "personalized" || source === "personalized_session";
}

/**
 * Same-origin refetch. `tier=crowd` is sent when this device has been blocked:
 * the visitor cookie survives a logout by design, so without the flag the
 * server would keep ranking the shelf by the history this device asked to stop
 * seeing.
 */
async function fetchShelf(crowd: boolean): Promise<RecommendationShelf | null> {
  const query = `limit=${SHELF_LIMIT}${crowd ? "&tier=crowd" : ""}`;
  const res = await authFetch(`/api/v1/recommendations/personalized?${query}`);
  if (!res.ok) return null;
  const json = await res.json();
  const data = json?.data;
  if (!data || !Array.isArray(data.items)) return null;
  return { source: data.source ?? "", items: data.items };
}

/**
 * The "See all" body, split from the page because the device-intent flag only
 * exists in the browser.
 *
 * After a logout the visitor cookie stays — so a later sign-in gets the history
 * back — which means the server still ranks a personal shelf, and the flag is
 * the only thing that says "not mine right now". It is unreadable during SSR,
 * so it is read after mount; when it says blocked the shelf is refetched and
 * swapped in. The already-painted server shelf stays put while that happens:
 * briefly showing the wrong heading beats a spinner over rendered content.
 *
 * "Already painted" is the important part. A logout that flips the flag while
 * this page is mounted — from the navbar, or the account layout redirecting
 * out — must not leave the personal row on screen for a second.
 */
export function RecommendationsView({ initialShelf }: RecommendationsViewProps) {
  const [shelf, setShelf] = useState(initialShelf);

  // Re-run on every logout/sign-in: the server shelf was rendered with the
  // cookie, so a flag flipped after this page mounted has to replace it too.
  const revision = useSyncExternalStore(
    subscribeDeviceIntent,
    deviceIntentRevision,
    deviceIntentRevision,
  );

  useEffect(() => {
    if (deviceIntentAllowed()) {
      // Allowed again (signed in): the server shelf matches the visitor. The
      // crowd shelf fetched while blocked is dropped by the gate below, so
      // there is nothing to write back.
      return;
    }
    let cancelled = false;
    void fetchShelf(true).then((next) => {
      // A failed refetch keeps the server shelf: the wrong label is recoverable,
      // an empty page is not.
      if (!cancelled && next) setShelf(next);
    });
    return () => {
      cancelled = true;
    };
  }, [revision, initialShelf]);

  // The flag is unreadable during SSR, so this renders the server shelf on the
  // first paint. Gating the paint on the client read would flash a different
  // heading on every load; the effect above is the block, and it runs before
  // the browser paints the committed frame.
  //
  // Both shelves go through the gate: the server shelf is personal by default,
  // and a crowd shelf fetched while blocked is only shown to a visitor who is
  // still blocked. Signing back in drops it for the server's answer.
  const personal = isPersonalSource(shelf?.source);
  const shown = deviceIntentAllowed() === personal ? shelf : null;
  const items = shown?.items ?? [];
  const labels = shelfLabels(shown?.source);

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
      <nav
        aria-label="Breadcrumb"
        className="flex items-center gap-1.5 text-xs text-slate-500 mb-5 font-body"
      >
        <Link href="/" className="hover:text-[#007D9C] transition-colors">
          Home
        </Link>
        <span aria-hidden="true">/</span>
        <span className="text-slate-800 font-semibold">
          {labels.short}
        </span>
      </nav>

      <header className="mb-8">
        <span className="inline-flex items-center gap-1.5 text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
          {labels.personalized ? (
            <Sparkles className="w-3.5 h-3.5" aria-hidden="true" />
          ) : (
            <Star className="w-3.5 h-3.5" aria-hidden="true" />
          )}
          {labels.eyebrow}
        </span>
        <h1 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-1">
          {labels.heading}
        </h1>
        <p className="text-sm text-slate-500 font-body mt-1.5">
          The full shelf: {items.length} items. {labels.subtitle}.
        </p>
      </header>

      {items.length > 0 ? (
        <ProductGrid products={items} />
      ) : (
        <div className="p-12 text-center bg-white rounded-xl border border-slate-200">
          <p className="text-sm text-slate-600 font-body">
            No recommendations available right now. Browse the catalog instead.
          </p>
          <Link
            href="/products"
            className="inline-flex items-center gap-2 mt-4 px-5 py-2.5 rounded-md bg-[#007D9C] text-white text-xs font-bold font-heading hover:bg-[#00637C] transition-colors"
          >
            <ArrowLeft className="w-3.5 h-3.5" aria-hidden="true" />
            Browse all products
          </Link>
        </div>
      )}
    </div>
  );
}
