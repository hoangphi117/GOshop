import React from "react";
import Link from "next/link";
import { cookies } from "next/headers";
import {
  getBanners,
  getCategories,
  getProducts,
  getRecommendations,
  type Product,
} from "@/lib/api-server";
import { HeroBanner } from "@/components/home/hero-banner";
import { FlashDeals } from "@/components/home/flash-deals";
import { CategoryShowcase } from "@/components/home/category-showcase";
import { RecommendationSection } from "@/components/home/recommendation-section";
import { HomePopup } from "@/components/home/home-popup";
import { ProductGrid } from "@/components/product/product-grid";
import { ArrowRight } from "lucide-react";

// Dynamic rendering ensures freshly updated products & categories display instantly
export const revalidate = 0;

export default async function HomePage() {
  // Minted by src/middleware.ts before this render, so the shelf below is the
  // visitor's own and not the best-seller fallback.
  const anonId = (await cookies()).get("goshop_anon_id")?.value;

  // Fetch real data concurrently from Go backend
  const [banners, subBanners, categories, productsResponse, initialShelf] =
    await Promise.all([
      getBanners("home_main"),
      getBanners("home_sub"),
      getCategories(),
      getProducts({ limit: 24 }),
      getRecommendations(12, anonId),
    ]);
  // Popup is a separate slot; a missing promo must not delay the shelf above.
  const popupBanners = await getBanners("home_popup");

  const initialProducts: Product[] = productsResponse?.products || [];

  // Top 4 products for flash deals
  const dealProducts = initialProducts.slice(0, 4);
  // Catalog products
  const catalogProducts =
    initialProducts.length > 4 ? initialProducts.slice(4) : initialProducts;

  return (
    <div className="flex flex-col min-h-screen bg-[#F8FAFC]">
      {/* 0. Promo modal — once per session, never blocks the page underneath */}
      <HomePopup banners={popupBanners} />

      {/* 1. Hero Showcase Section */}
      <HeroBanner banners={banners} subBanners={subBanners} />

      {/* 2. Flash Sale Countdown & Real Deals Ticker */}
      <FlashDeals products={dealProducts} />

      {/* 3. Curated Department Grid (4 Columns) */}
      <CategoryShowcase categories={categories} />

      {/* 4. Personalized shelf — server-rendered fallback keeps first paint full */}
      <RecommendationSection initialShelf={initialShelf} limit={12} />

      {/* 5. Featured Drops & Best Sellers Catalog Grid */}
      <section id="featured" className="max-w-7xl mx-auto px-4 sm:px-6 mb-16">
        <div className="flex flex-col md:flex-row justify-between items-start md:items-end gap-4 mb-8">
          <div>
            <span className="text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
              Official Catalog
            </span>
            <h2 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5">
              Featured Drops
            </h2>
          </div>

          {/* Dynamic Category Filter Links */}
          <div className="flex items-center gap-2 overflow-x-auto custom-scrollbar w-full md:w-auto pb-2 md:pb-0">
            <Link
              href="/products"
              aria-current="page"
              className="bg-[#007D9C] text-white px-4 py-1.5 rounded-md text-xs font-bold shrink-0 shadow-xs font-heading"
            >
              All Collections
            </Link>
            {categories.slice(0, 4).map((cat) => (
              <Link
                key={cat.uuid || cat.slug}
                href={`/products?category=${cat.slug}`}
                className="bg-white hover:bg-slate-50 text-slate-700 border border-slate-200/90 px-4 py-1.5 rounded-md text-xs font-semibold shrink-0 transition-colors font-heading"
              >
                {cat.category_name}
              </Link>
            ))}
          </div>
        </div>

        {/* Real Product Grid */}
        {catalogProducts.length > 0 ? (
          <ProductGrid products={catalogProducts} />
        ) : (
          <div className="p-12 text-center bg-white rounded-3xl border border-slate-200 text-slate-500 text-sm">
            Catalog is updating. Check back shortly for new drops!
          </div>
        )}

        {/* Load More Official Gear CTA */}
        <div className="mt-10 text-center">
          <Link
            href="/products"
            className="inline-flex items-center gap-2 bg-white hover:bg-slate-50 text-[#007D9C] border-2 border-[#007D9C]/30 hover:border-[#007D9C] px-8 py-3.5 rounded-md text-sm font-bold transition-all shadow-xs hover:shadow-sm active:scale-95 font-heading cursor-pointer"
          >
            <span>Explore Full Official Catalog</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>
      </section>
    </div>
  );
}
