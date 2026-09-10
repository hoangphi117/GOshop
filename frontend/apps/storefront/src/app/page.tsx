import React from "react";
import Link from "next/link";
import {
  getBanners,
  getCategories,
  getProducts,
  type Product,
} from "@/lib/api-server";
import { HeroBanner } from "@/components/home/hero-banner";
import { CategoryShowcase } from "@/components/home/category-showcase";
import { ProductGrid } from "@/components/product/product-grid";
import { Sparkles, ArrowRight } from "lucide-react";

// Dynamic rendering ensures newly added or deleted banners display immediately on reload
export const revalidate = 0;

export default async function HomePage() {
  // Fetch real data concurrently from Go backend
  const [banners, subBanners, categories] = await Promise.all([
    getBanners("home_main"),
    getBanners("home_sub"),
    getCategories(),
  ]);

  const subBanner = subBanners[0] || null;

  // Fetch products for each category (up to 12 products per category = max 3 rows of 4 items)
  const categoryProductsList = await Promise.all(
    categories.map(async (cat) => {
      const data = await getProducts({ category_uuid: cat.uuid, limit: 12 });
      return {
        category: cat,
        products: data.products || [],
        total: data.pagination?.total_records || (data.products || []).length,
      };
    }),
  );

  const categorySections = categoryProductsList.filter(
    (item) => item.products.length > 0,
  );

  // Fallback in case no categories have products assigned yet
  let fallbackProducts: Product[] = [];
  if (categorySections.length === 0) {
    const allProducts = await getProducts({ limit: 12 });
    fallbackProducts = allProducts.products || [];
  }

  return (
    <div className="flex flex-col min-h-screen">
      {/* 1. Hero Banner with Gradient & Floating Glass */}
      <HeroBanner banners={banners} subBanner={subBanner} />

      {/* 2. Categories Showcase */}
      <CategoryShowcase categories={categories} />

      {/* 3. Products Grouped by Category (Max 3 rows per category with View More button) */}
      <div id="deals" className="py-12 bg-white border-y border-slate-100">
        {categorySections.map(({ category, products }, index) => {
          const categoryUrl = `/products?category=${encodeURIComponent(category.slug || category.uuid)}`;
          // Up to 3 rows of products (12 items)
          const displayedProducts = products.slice(0, 12);

          return (
            <React.Fragment key={category.uuid}>
              {index > 0 && (
                <div className="max-w-6xl mx-auto px-4 sm:px-6 py-10">
                  <div className="border-t border-slate-200" />
                </div>
              )}
              <section
                id={`cat-${category.slug || category.uuid}`}
                className="py-2"
              >
                <div className="max-w-6xl mx-auto px-4 sm:px-6">
                  {/* Category Header */}
                  <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-8 gap-4 pb-4 border-b border-slate-100">
                    <div>
                      <div className="inline-flex items-center gap-1.5 text-xs font-bold text-[#2196F3] uppercase tracking-wider mb-2">
                        <Sparkles className="w-3.5 h-3.5" />
                        <span>{category.category_name}</span>
                      </div>
                      <h2 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
                        {category.category_name}
                      </h2>
                      {category.description && (
                        <p className="text-xs sm:text-sm text-slate-500 mt-1 max-w-xl line-clamp-1">
                          {category.description}
                        </p>
                      )}
                    </div>

                    <Link
                      href={categoryUrl}
                      className="inline-flex items-center gap-1.5 text-sm font-semibold text-[#0D47A1] hover:text-[#2196F3] transition-colors group shrink-0"
                    >
                      <span>View all {category.category_name}</span>
                      <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-1" />
                    </Link>
                  </div>

                  {/* Products Grid (Max 3 rows) */}
                  <ProductGrid products={displayedProducts} />

                  {/* View More Button for Category */}
                  <div className="mt-8 flex justify-center">
                    <Link
                      href={categoryUrl}
                      className="inline-flex items-center gap-2 px-7 py-3.5 rounded-xl border border-slate-200 bg-white hover:bg-[#E3F2FD] text-xs sm:text-sm font-bold text-[#0D1B2A] hover:border-[#2196F3] hover:text-[#0D47A1] transition-all shadow-xs hover:shadow-sm active:scale-98 cursor-pointer"
                    >
                      <span>View More {category.category_name}</span>
                      <ArrowRight className="w-4 h-4 text-[#2196F3]" />
                    </Link>
                  </div>
                </div>
              </section>
            </React.Fragment>
          );
        })}

        {categorySections.length === 0 && fallbackProducts.length > 0 && (
          <section className="py-4">
            <div className="max-w-6xl mx-auto px-4 sm:px-6">
              <div className="mb-8 pb-4 border-b border-slate-100">
                <h2 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
                  All Products
                </h2>
              </div>
              <ProductGrid products={fallbackProducts.slice(0, 12)} />
              <div className="mt-8 flex justify-center">
                <Link
                  href="/products"
                  className="inline-flex items-center gap-2 px-7 py-3.5 rounded-xl border border-slate-200 bg-white hover:bg-[#E3F2FD] text-xs sm:text-sm font-bold text-[#0D1B2A] hover:border-[#2196F3] hover:text-[#0D47A1] transition-all shadow-xs hover:shadow-sm active:scale-98 cursor-pointer"
                >
                  <span>View All Products</span>
                  <ArrowRight className="w-4 h-4 text-[#2196F3]" />
                </Link>
              </div>
            </div>
          </section>
        )}
      </div>
    </div>
  );
}
