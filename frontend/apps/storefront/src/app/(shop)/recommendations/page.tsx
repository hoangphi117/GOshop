import React from "react";
import Link from "next/link";
import { cookies } from "next/headers";
import { ArrowLeft, Sparkles, Star } from "lucide-react";
import { getRecommendations } from "@/lib/api-server";
import { ProductGrid } from "@/components/product/product-grid";
import { ANON_COOKIE_NAME } from "@/lib/search-history";

export const dynamic = "force-dynamic";

export const metadata = {
  title: "Recommended for you — GOshop",
  description:
    "Everything GOshop picked for you, plus the crowd favourites when you are new here.",
};

// The service caps a shelf at 50 items, so that is the whole list.
const SHELF_LIMIT = 50;

export default async function RecommendationsPage() {
  const anonId = (await cookies()).get(ANON_COOKIE_NAME)?.value;
  const shelf = await getRecommendations(SHELF_LIMIT, anonId);
  const items = shelf?.items ?? [];
  const personalized = shelf?.source === "personalized";

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
          {personalized ? "Recommended" : "Best sellers"}
        </span>
      </nav>

      <header className="mb-8">
        <span className="inline-flex items-center gap-1.5 text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
          {personalized ? (
            <Sparkles className="w-3.5 h-3.5" aria-hidden="true" />
          ) : (
            <Star className="w-3.5 h-3.5" aria-hidden="true" />
          )}
          {personalized ? "Picked for you" : "Shop the crowd favourites"}
        </span>
        <h1 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-1">
          {personalized ? "Recommended for you" : "Best Sellers"}
        </h1>
        <p className="text-sm text-slate-500 font-body mt-1.5">
          {personalized
            ? `The full shelf: ${items.length} items ranked from your recent activity.`
            : `The full shelf: ${items.length} top-rated and most bought items this week.`}
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
