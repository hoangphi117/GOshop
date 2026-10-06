import React from "react";
import Link from "next/link";
import {
  getBanners,
  getCategories,
  getProducts,
  type Product,
} from "@/lib/api-server";
import { HeroBanner } from "@/components/home/hero-banner";
import { FlashDeals } from "@/components/home/flash-deals";
import { CategoryShowcase } from "@/components/home/category-showcase";
import { ProductGrid } from "@/components/product/product-grid";
import { ArrowRight } from "lucide-react";

// Dynamic rendering ensures freshly updated products & categories display instantly
export const revalidate = 0;

export default async function HomePage() {
  // Fetch real data concurrently from Go backend
  const [banners, subBanners, categories, productsResponse] = await Promise.all(
    [
      getBanners("home_main"),
      getBanners("home_sub"),
      getCategories(),
      getProducts({ limit: 12 }),
    ],
  );

  const subBanner = subBanners[0] || null;
  const initialProducts: Product[] = productsResponse?.products || [];

  // Top 4 products for flash deals
  const dealProducts = initialProducts.slice(0, 4);
  // Catalog products
  const catalogProducts =
    initialProducts.length > 4 ? initialProducts.slice(4) : initialProducts;

  return (
    <div className="flex flex-col min-h-screen bg-[#F8FAFC]">
      {/* 1. Hero Showcase Section */}
      <HeroBanner banners={banners} subBanner={subBanner} />

      {/* 2. Flash Sale Countdown & Real Deals Ticker */}
      <FlashDeals products={dealProducts} />

      {/* 3. Curated Department Grid (4 Columns) */}
      <CategoryShowcase categories={categories} />

      {/* 4. Featured Drops & Best Sellers Catalog Grid */}
      <section id="featured" className="max-w-7xl mx-auto px-4 sm:px-6 mb-16">
        <div className="flex flex-col md:flex-row justify-between items-start md:items-end gap-4 mb-8">
          <div>
            <span className="text-xs font-bold text-[#007D9C] uppercase tracking-wider font-heading">
              Official Catalog
            </span>
            <h2 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading mt-0.5">
              Featured Drops & Best Sellers
            </h2>
          </div>

          {/* Dynamic Category Filter Links */}
          <div className="flex items-center gap-2 overflow-x-auto custom-scrollbar w-full md:w-auto pb-2 md:pb-0">
            <Link
              href="/products"
              className="bg-[#007D9C] text-white px-4 py-1.5 rounded-full text-xs font-bold shrink-0 shadow-xs font-heading"
            >
              All Merch
            </Link>
            {categories.slice(0, 4).map((cat) => (
              <Link
                key={cat.uuid || cat.slug}
                href={`/products?category=${cat.slug}`}
                className="bg-white hover:bg-slate-50 text-slate-700 border border-slate-200/90 px-4 py-1.5 rounded-full text-xs font-semibold shrink-0 transition-colors font-heading"
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
