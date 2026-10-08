import React from "react";
import Link from "next/link";
import { getCategories, getProducts } from "@/lib/api-server";
import { ProductGrid } from "@/components/product/product-grid";
import { SearchAutocomplete } from "@/components/layout/search-autocomplete";
import {
  ChevronRight,
  Filter,
  Search,
  ChevronLeft,
  SlidersHorizontal,
  X,
  Check,
  RotateCcw,
  Sparkles,
  Star,
  Banknote,
} from "lucide-react";

interface ProductsPageProps {
  searchParams: Promise<{
    category?: string;
    search?: string;
    sort?: string;
    page?: string;
    min_price?: string;
    max_price?: string;
    min_rating?: string;
  }>;
}

export const dynamic = "force-dynamic";

const PRICE_PRESETS = [
  { label: "Under 1,000,000₫", min: undefined, max: 1000000 },
  { label: "1,000,000₫ – 5,000,000₫", min: 1000000, max: 5000000 },
  { label: "5,000,000₫ – 15,000,000₫", min: 5000000, max: 15000000 },
  { label: "Over 15,000,000₫", min: 15000000, max: undefined },
];

const RATING_PRESETS = [
  { label: "4 Stars & above", rating: 4 },
  { label: "3 Stars & above", rating: 3 },
  { label: "2 Stars & above", rating: 2 },
  { label: "1 Star & above", rating: 1 },
];

export default async function ProductsPage({
  searchParams,
}: ProductsPageProps) {
  const params = await searchParams;
  const currentPage = parseInt(params.page || "1", 10);
  const currentSearch = params.search?.trim() || "";
  const currentSort = params.sort || "";
  const currentMinPrice = params.min_price
    ? parseFloat(params.min_price)
    : undefined;
  const currentMaxPrice = params.max_price
    ? parseFloat(params.max_price)
    : undefined;
  const currentMinRating = params.min_rating
    ? parseFloat(params.min_rating)
    : undefined;

  // Fetch categories to resolve active department
  const categories = await getCategories();
  const currentCategory = params.category?.trim() || "";

  // Fetch paginated products matching filters
  const productsData = await getProducts({
    page: currentPage,
    limit: 12,
    category: currentCategory,
    search: currentSearch,
    sort: currentSort,
    min_price: currentMinPrice,
    max_price: currentMaxPrice,
    min_rating: currentMinRating,
  });

  const products = productsData.products || [];
  const pagination = productsData.pagination;

  const selectedCategoryObj = categories.find(
    (c) =>
      c.slug.toLowerCase() === currentCategory.toLowerCase() ||
      c.uuid.toLowerCase() === currentCategory.toLowerCase(),
  );

  // Helper to construct query URLs cleanly
  const buildUrl = (overrides: Record<string, string | undefined>) => {
    const q = new URLSearchParams();
    const merged: Record<string, string | undefined> = {
      category: currentCategory || undefined,
      search: currentSearch || undefined,
      sort: currentSort || undefined,
      min_price:
        currentMinPrice !== undefined ? currentMinPrice.toString() : undefined,
      max_price:
        currentMaxPrice !== undefined ? currentMaxPrice.toString() : undefined,
      min_rating:
        currentMinRating !== undefined
          ? currentMinRating.toString()
          : undefined,
      page: currentPage > 1 ? currentPage.toString() : undefined,
      ...overrides,
    };

    Object.entries(merged).forEach(([k, v]) => {
      if (v !== undefined && v !== "") {
        q.set(k, v);
      }
    });

    const qs = q.toString();
    return `/products${qs ? `?${qs}` : ""}`;
  };

  const hasActiveFilters = Boolean(
    currentCategory ||
    currentSearch ||
    currentSort ||
    currentMinPrice !== undefined ||
    currentMaxPrice !== undefined ||
    currentMinRating !== undefined,
  );

  return (
    <div className="bg-[#F8FAFC] min-h-screen py-6 sm:py-8 font-body">
      <div className="max-w-7xl mx-auto px-4 sm:px-6">
        {/* 1. Breadcrumbs */}
        <nav
          aria-label="Breadcrumb"
          className="flex items-center gap-1.5 text-xs text-slate-500 mb-5"
        >
          <Link href="/" className="hover:text-[#007D9C] font-semibold">
            Home
          </Link>
          <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
          <Link
            href="/products"
            className={`font-semibold ${
              !currentCategory && !currentSearch && !hasActiveFilters
                ? "text-slate-900"
                : "hover:text-[#007D9C]"
            }`}
          >
            Products
          </Link>
          {selectedCategoryObj && (
            <>
              <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
              <span className="font-bold text-slate-900 truncate font-heading">
                {selectedCategoryObj.category_name}
              </span>
            </>
          )}
          {currentSearch && (
            <>
              <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
              <span className="font-bold text-slate-900 truncate font-heading">
                Search: &quot;{currentSearch}&quot;
              </span>
            </>
          )}
        </nav>

        {/* 2. Category Header Banner */}
        <div className="bg-white border border-slate-200/90 rounded-xs p-5 sm:p-6 mb-6 shadow-xs">
          <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
            <div>
              <div className="flex items-center gap-2.5">
                <h1 className="text-2xl sm:text-3xl font-extrabold text-slate-900 tracking-tight font-heading">
                  {selectedCategoryObj
                    ? selectedCategoryObj.category_name
                    : currentSearch
                      ? `Search Results for "${currentSearch}"`
                      : "All Hardware & Tech Merch"}
                </h1>
                <span className="bg-[#E0F7FC] text-[#007D9C] border border-[#BAEAFF] text-xs font-bold px-2 py-0.5 rounded-xs font-heading">
                  {pagination?.total_records ?? products.length} Items
                </span>
              </div>
              <p className="text-xs sm:text-sm text-slate-600 mt-1 max-w-3xl leading-relaxed">
                {selectedCategoryObj?.description ||
                  "Browse genuine tech gear, mechanical setups, collector merch, and high-performance developer accessories."}
              </p>
            </div>

            {/* Clear Filters CTA if active filters */}
            {hasActiveFilters && (
              <Link
                href="/products"
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xs border border-slate-300 bg-slate-50 hover:bg-slate-100 text-xs font-bold text-slate-700 font-heading shrink-0 self-start md:self-auto"
              >
                <RotateCcw className="w-3.5 h-3.5 text-slate-500" />
                <span>Reset All Filters</span>
              </Link>
            )}
          </div>
        </div>

        {/* 3. Main 2-Column Catalog Layout */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
          {/* Left Filter Sidebar (lg:col-span-3 = 280px) */}
          <aside className="lg:col-span-3 space-y-4">
            {/* Department Filter Section */}
            <div className="bg-white border border-slate-200/90 rounded-xs p-4 shadow-xs">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-3">
                <div className="flex items-center gap-2 text-xs font-bold text-slate-900 uppercase tracking-wider font-heading">
                  <Filter className="w-3.5 h-3.5 text-[#007D9C]" />
                  <span>Departments</span>
                </div>
              </div>

              <div className="space-y-1">
                <Link
                  href={buildUrl({ category: undefined, page: "1" })}
                  className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-bold font-heading ${
                    !currentCategory
                      ? "bg-[#007D9C] text-white"
                      : "text-slate-700 hover:bg-[#F0F9FB] hover:text-[#007D9C]"
                  }`}
                >
                  <span>All Categories</span>
                  {!currentCategory && (
                    <Check className="w-3.5 h-3.5 text-white" />
                  )}
                </Link>

                {categories.map((cat) => {
                  const isActive =
                    currentCategory.toLowerCase() === cat.slug.toLowerCase() ||
                    currentCategory.toLowerCase() === cat.uuid.toLowerCase();

                  return (
                    <Link
                      key={cat.uuid || cat.slug}
                      href={buildUrl({ category: cat.slug, page: "1" })}
                      className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-semibold font-body ${
                        isActive
                          ? "bg-[#007D9C] text-white font-bold font-heading"
                          : "text-slate-700 hover:bg-[#F0F9FB] hover:text-[#007D9C]"
                      }`}
                    >
                      <span className="truncate">{cat.category_name}</span>
                      {isActive && (
                        <Check className="w-3.5 h-3.5 text-white shrink-0" />
                      )}
                    </Link>
                  );
                })}
              </div>
            </div>

            {/* Price Range Filter Section */}
            <div className="bg-white border border-slate-200/90 rounded-xs p-4 shadow-xs">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-3">
                <div className="flex items-center gap-2 text-xs font-bold text-slate-900 uppercase tracking-wider font-heading">
                  <Banknote className="w-3.5 h-3.5 text-[#007D9C]" />
                  <span>Price Range</span>
                </div>
                {(currentMinPrice !== undefined ||
                  currentMaxPrice !== undefined) && (
                  <Link
                    href={buildUrl({
                      min_price: undefined,
                      max_price: undefined,
                      page: "1",
                    })}
                    className="text-[11px] font-semibold text-[#007D9C] hover:underline"
                  >
                    Clear
                  </Link>
                )}
              </div>

              {/* Price Presets */}
              <div className="space-y-1 mb-4">
                <Link
                  href={buildUrl({
                    min_price: undefined,
                    max_price: undefined,
                    page: "1",
                  })}
                  className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-semibold ${
                    currentMinPrice === undefined &&
                    currentMaxPrice === undefined
                      ? "bg-[#E0F7FC] text-[#007D9C] font-bold font-heading"
                      : "text-slate-700 hover:bg-slate-50"
                  }`}
                >
                  <span>All Prices</span>
                </Link>

                {PRICE_PRESETS.map((preset) => {
                  const isPresetActive =
                    currentMinPrice === preset.min &&
                    currentMaxPrice === preset.max;

                  return (
                    <Link
                      key={preset.label}
                      href={buildUrl({
                        min_price: preset.min
                          ? preset.min.toString()
                          : undefined,
                        max_price: preset.max
                          ? preset.max.toString()
                          : undefined,
                        page: "1",
                      })}
                      className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-semibold ${
                        isPresetActive
                          ? "bg-[#E0F7FC] text-[#007D9C] font-bold font-heading"
                          : "text-slate-700 hover:bg-slate-50"
                      }`}
                    >
                      <span>{preset.label}</span>
                    </Link>
                  );
                })}
              </div>

              {/* Custom Price Range Form */}
              <form
                method="GET"
                action="/products"
                className="pt-3 border-t border-slate-100 space-y-2.5"
              >
                {currentCategory && (
                  <input
                    type="hidden"
                    name="category"
                    value={currentCategory}
                  />
                )}
                {currentSearch && (
                  <input type="hidden" name="search" value={currentSearch} />
                )}
                {currentSort && (
                  <input type="hidden" name="sort" value={currentSort} />
                )}
                {currentMinRating !== undefined && (
                  <input
                    type="hidden"
                    name="min_rating"
                    value={currentMinRating.toString()}
                  />
                )}

                <div className="text-[11px] font-bold uppercase tracking-wider text-slate-500 font-heading">
                  Custom Range (₫)
                </div>
                <div className="flex items-center gap-2">
                  <input
                    type="number"
                    name="min_price"
                    defaultValue={currentMinPrice ?? ""}
                    placeholder="Min"
                    min="0"
                    step="10000"
                    className="w-full px-2.5 py-1.5 text-xs rounded-xs border border-slate-300 bg-white placeholder-slate-400 outline-none focus:border-[#007D9C]"
                  />
                  <span className="text-slate-400 text-xs">-</span>
                  <input
                    type="number"
                    name="max_price"
                    defaultValue={currentMaxPrice ?? ""}
                    placeholder="Max"
                    min="0"
                    step="10000"
                    className="w-full px-2.5 py-1.5 text-xs rounded-xs border border-slate-300 bg-white placeholder-slate-400 outline-none focus:border-[#007D9C]"
                  />
                </div>
                <button
                  type="submit"
                  className="w-full py-1.5 px-3 rounded-xs bg-slate-900 hover:bg-slate-800 text-white text-xs font-bold font-heading"
                >
                  Apply Price
                </button>
              </form>
            </div>

            {/* Customer Rating Filter Section */}
            <div className="bg-white border border-slate-200/90 rounded-xs p-4 shadow-xs">
              <div className="flex items-center justify-between pb-3 border-b border-slate-100 mb-3">
                <div className="flex items-center gap-2 text-xs font-bold text-slate-900 uppercase tracking-wider font-heading">
                  <Star className="w-3.5 h-3.5 text-[#F59E0B] fill-[#F59E0B]" />
                  <span>Customer Rating</span>
                </div>
                {currentMinRating !== undefined && (
                  <Link
                    href={buildUrl({ min_rating: undefined, page: "1" })}
                    className="text-[11px] font-semibold text-[#007D9C] hover:underline"
                  >
                    Clear
                  </Link>
                )}
              </div>

              <div className="space-y-1">
                <Link
                  href={buildUrl({ min_rating: undefined, page: "1" })}
                  className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-semibold ${
                    currentMinRating === undefined
                      ? "bg-[#E0F7FC] text-[#007D9C] font-bold font-heading"
                      : "text-slate-700 hover:bg-slate-50"
                  }`}
                >
                  <span>All Ratings</span>
                </Link>

                {RATING_PRESETS.map((preset) => {
                  const isRatingActive = currentMinRating === preset.rating;

                  return (
                    <Link
                      key={preset.rating}
                      href={buildUrl({
                        min_rating: preset.rating.toString(),
                        page: "1",
                      })}
                      className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-semibold ${
                        isRatingActive
                          ? "bg-[#E0F7FC] text-[#007D9C] font-bold font-heading"
                          : "text-slate-700 hover:bg-slate-50"
                      }`}
                    >
                      <div className="flex items-center gap-1.5">
                        <div className="flex items-center text-[#F59E0B]">
                          {Array.from({ length: 5 }).map((_, i) => (
                            <Star
                              key={i}
                              className={`w-3.5 h-3.5 ${
                                i < preset.rating
                                  ? "fill-[#F59E0B] text-[#F59E0B]"
                                  : "text-slate-300"
                              }`}
                            />
                          ))}
                        </div>
                        <span className="text-slate-700 font-medium text-[11px]">
                          &amp; Up
                        </span>
                      </div>
                    </Link>
                  );
                })}
              </div>
            </div>

            {/* Quick Sort Options in Sidebar */}
            <div className="bg-white border border-slate-200/90 rounded-xs p-4 shadow-xs">
              <div className="flex items-center gap-2 text-xs font-bold text-slate-900 uppercase tracking-wider font-heading pb-3 border-b border-slate-100 mb-3">
                <SlidersHorizontal className="w-3.5 h-3.5 text-[#007D9C]" />
                <span>Sort Order</span>
              </div>

              <div className="space-y-1">
                {[
                  { label: "Default Order", val: "" },
                  { label: "Top Customer Rated", val: "rating_desc" },
                  { label: "Newest Arrivals", val: "newest" },
                  { label: "Price: Low to High", val: "price_asc" },
                  { label: "Price: High to Low", val: "price_desc" },
                  { label: "Name: A to Z", val: "name_asc" },
                ].map((s) => {
                  const isSortActive = currentSort === s.val;
                  return (
                    <Link
                      key={s.label}
                      href={buildUrl({
                        sort: s.val ? s.val : undefined,
                        page: "1",
                      })}
                      className={`flex items-center justify-between px-3 py-2 rounded-xs text-xs font-semibold ${
                        isSortActive
                          ? "bg-[#E0F7FC] text-[#007D9C] font-bold font-heading"
                          : "text-slate-700 hover:bg-slate-50"
                      }`}
                    >
                      <span>{s.label}</span>
                    </Link>
                  );
                })}
              </div>
            </div>

            {/* Quality & Authenticity Guarantee Note */}
            <div className="bg-[#071F2C] text-white border border-[#00384D] rounded-xs p-4 space-y-2 text-xs shadow-xs">
              <div className="flex items-center gap-1.5 text-xs font-bold text-[#BAEAFF] font-heading">
                <span>100% Genuine Standard</span>
              </div>
              <p className="text-[11px] text-[#B0CDD9] leading-relaxed">
                Direct nationwide fulfillment from verified distributor
                inventory. 7-day hassle-free returns supported.
              </p>
            </div>
          </aside>

          {/* Right Product Grid Column (lg:col-span-9) */}
          <main className="lg:col-span-9 space-y-5">
            {/* Filter Controls & In-Catalog Search Bar (2-Column Layout) */}
            <div className="bg-white border border-slate-200/90 rounded-xs p-3.5 flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3 shadow-xs">
              {/* Column 1: Keyword Search Input (with Meilisearch autocomplete dropdown) */}
              <div className="w-full md:w-72 lg:w-80 shrink-0">
                <SearchAutocomplete
                  variant="compact"
                  placeholder="Filter within results..."
                  initialValue={currentSearch}
                  preserveParams={{
                    category: currentCategory,
                    sort: currentSort,
                    min_price: currentMinPrice,
                    max_price: currentMaxPrice,
                    min_rating: currentMinRating,
                  }}
                />
              </div>

              {/* Column 2: Active Filter Badges (Wraps cleanly without crowding search bar) */}
              <div className="flex-1 flex flex-wrap items-center gap-1.5 md:justify-end text-xs">
                {currentCategory && selectedCategoryObj && (
                  <Link
                    href={buildUrl({ category: undefined, page: "1" })}
                    className="inline-flex items-center gap-1 bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-200/80 text-[11px] font-semibold px-2.5 py-1 rounded-xs font-body transition-colors"
                  >
                    <span>{selectedCategoryObj.category_name}</span>
                    <X className="w-3 h-3 text-slate-400 hover:text-slate-600" />
                  </Link>
                )}
                {currentSearch && (
                  <Link
                    href={buildUrl({ search: undefined, page: "1" })}
                    className="inline-flex items-center gap-1 bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-200/80 text-[11px] font-semibold px-2.5 py-1 rounded-xs font-body transition-colors"
                  >
                    <span>Search: &quot;{currentSearch}&quot;</span>
                    <X className="w-3 h-3 text-slate-400 hover:text-slate-600" />
                  </Link>
                )}
                {(currentMinPrice !== undefined ||
                  currentMaxPrice !== undefined) && (
                  <Link
                    href={buildUrl({
                      min_price: undefined,
                      max_price: undefined,
                      page: "1",
                    })}
                    className="inline-flex items-center gap-1 bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-200/80 text-[11px] font-semibold px-2.5 py-1 rounded-xs font-body transition-colors"
                  >
                    <span>
                      Price:{" "}
                      {currentMinPrice && currentMaxPrice
                        ? `${(currentMinPrice / 1000000).toFixed(0)}M - ${(currentMaxPrice / 1000000).toFixed(0)}M₫`
                        : currentMinPrice
                          ? `> ${(currentMinPrice / 1000000).toFixed(0)}M₫`
                          : `< ${(currentMaxPrice! / 1000000).toFixed(0)}M₫`}
                    </span>
                    <X className="w-3 h-3 text-slate-400 hover:text-slate-600" />
                  </Link>
                )}
                {currentMinRating !== undefined && (
                  <Link
                    href={buildUrl({ min_rating: undefined, page: "1" })}
                    className="inline-flex items-center gap-1 bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-200/80 text-[11px] font-semibold px-2.5 py-1 rounded-xs font-body transition-colors"
                  >
                    <span>Rating: {currentMinRating}★ &amp; up</span>
                    <X className="w-3 h-3 text-slate-400 hover:text-slate-600" />
                  </Link>
                )}
                {currentSort && (
                  <Link
                    href={buildUrl({ sort: undefined, page: "1" })}
                    className="inline-flex items-center gap-1 bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-200/80 text-[11px] font-semibold px-2.5 py-1 rounded-xs font-body transition-colors"
                  >
                    <span>
                      Sort:{" "}
                      {currentSort === "rating_desc"
                        ? "Top Rated"
                        : currentSort === "price_asc"
                          ? "Price Low-High"
                          : currentSort === "price_desc"
                            ? "Price High-Low"
                            : currentSort === "newest"
                              ? "Newest"
                              : currentSort}
                    </span>
                    <X className="w-3 h-3 text-slate-400 hover:text-slate-600" />
                  </Link>
                )}
              </div>
            </div>

            {/* Product Cards Grid */}
            <ProductGrid
              products={products}
              columnsClassName="grid-cols-1 sm:grid-cols-2 lg:grid-cols-3"
              emptyMessage={
                hasActiveFilters
                  ? "No products match the selected filters. Try broadening your criteria."
                  : "No products available in this department."
              }
            />

            {/* Pagination Controls */}
            {pagination && pagination.total_pages > 1 && (
              <div className="bg-white border border-slate-200/90 rounded-xs p-4 flex flex-col sm:flex-row items-center justify-between gap-4 shadow-xs">
                <p className="text-xs text-slate-600 font-body">
                  Showing page{" "}
                  <strong className="text-slate-900 font-heading">
                    {pagination.page}
                  </strong>{" "}
                  of{" "}
                  <strong className="text-slate-900 font-heading">
                    {pagination.total_pages}
                  </strong>{" "}
                  ({pagination.total_records} total items)
                </p>

                <div className="flex items-center gap-1.5 font-heading">
                  {pagination.has_prev ? (
                    <Link
                      href={buildUrl({
                        page: (pagination.page - 1).toString(),
                      })}
                      className="inline-flex items-center gap-1 px-3 py-1.5 rounded-xs border border-slate-300 bg-white text-xs font-bold text-slate-700 hover:bg-slate-50"
                    >
                      <ChevronLeft className="w-3.5 h-3.5" />
                      <span>Prev</span>
                    </Link>
                  ) : (
                    <span className="inline-flex items-center gap-1 px-3 py-1.5 rounded-xs border border-slate-200 bg-slate-50 text-xs font-bold text-slate-400 cursor-not-allowed">
                      <ChevronLeft className="w-3.5 h-3.5" />
                      <span>Prev</span>
                    </span>
                  )}

                  {/* Page Numbers */}
                  {Array.from({
                    length: Math.min(5, pagination.total_pages),
                  }).map((_, i) => {
                    const pNum = i + 1;
                    const isCurr = pNum === pagination.page;
                    return (
                      <Link
                        key={pNum}
                        href={buildUrl({ page: pNum.toString() })}
                        className={`w-8 h-8 flex items-center justify-center rounded-xs text-xs font-bold ${
                          isCurr
                            ? "bg-[#007D9C] text-white"
                            : "border border-slate-200 hover:bg-slate-50 text-slate-700"
                        }`}
                      >
                        {pNum}
                      </Link>
                    );
                  })}

                  {pagination.has_next ? (
                    <Link
                      href={buildUrl({
                        page: (pagination.page + 1).toString(),
                      })}
                      className="inline-flex items-center gap-1 px-3 py-1.5 rounded-xs border border-slate-300 bg-white text-xs font-bold text-slate-700 hover:bg-slate-50"
                    >
                      <span>Next</span>
                      <ChevronRight className="w-3.5 h-3.5" />
                    </Link>
                  ) : (
                    <span className="inline-flex items-center gap-1 px-3 py-1.5 rounded-xs border border-slate-200 bg-slate-50 text-xs font-bold text-slate-400 cursor-not-allowed">
                      <span>Next</span>
                      <ChevronRight className="w-3.5 h-3.5" />
                    </span>
                  )}
                </div>
              </div>
            )}
          </main>
        </div>
      </div>
    </div>
  );
}
