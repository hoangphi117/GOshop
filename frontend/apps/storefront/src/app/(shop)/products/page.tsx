import React from 'react'
import Link from 'next/link'
import { getCategories, getProducts } from '@/lib/api-server'
import { ProductGrid } from '@/components/product/product-grid'
import {
  ChevronRight,
  Filter,
  Search,
  ArrowUpDown,
  ChevronLeft,
  ShoppingBag,
} from 'lucide-react'

interface ProductsPageProps {
  searchParams: Promise<{
    category?: string
    search?: string
    sort?: string
    page?: string
  }>
}

export const dynamic = 'force-dynamic'

export default async function ProductsPage({ searchParams }: ProductsPageProps) {
  const params = await searchParams
  const currentPage = parseInt(params.page || '1', 10)
  const currentCategory = params.category || ''
  const currentSearch = params.search || ''
  const currentSort = params.sort || ''

  // Fetch categories and filtered products concurrently
  const [categories, productsData] = await Promise.all([
    getCategories(),
    getProducts({
      page: currentPage,
      limit: 12,
      category: currentCategory,
      search: currentSearch,
      sort: currentSort,
    }),
  ])

  const products = productsData.products || []
  const pagination = productsData.pagination

  // Helper to construct query string
  const createPageUrl = (pageNumber: number) => {
    const q = new URLSearchParams()
    if (currentCategory) q.set('category', currentCategory)
    if (currentSearch) q.set('search', currentSearch)
    if (currentSort) q.set('sort', currentSort)
    q.set('page', pageNumber.toString())
    return `/products?${q.toString()}`
  }

  const selectedCategoryObj = categories.find(
    (c) => c.slug === currentCategory || c.uuid === currentCategory
  )

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-8">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        {/* Breadcrumbs */}
        <nav className="flex items-center gap-2 text-xs text-slate-500 mb-6">
          <Link href="/" className="hover:text-[#2196F3] transition-colors">
            Home
          </Link>
          <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
          <Link href="/products" className="hover:text-[#2196F3] transition-colors">
            Catalog
          </Link>
          {selectedCategoryObj && (
            <>
              <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
              <span className="font-semibold text-[#0D1B2A]">
                {selectedCategoryObj.category_name}
              </span>
            </>
          )}
          {currentSearch && (
            <>
              <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
              <span className="font-semibold text-[#0D1B2A]">
                Search: &quot;{currentSearch}&quot;
              </span>
            </>
          )}
        </nav>

        {/* Page Title & Count */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-6 border-b border-slate-200/80 mb-8">
          <div>
            <h1 className="text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
              {selectedCategoryObj
                ? selectedCategoryObj.category_name
                : currentSearch
                ? `Results for "${currentSearch}"`
                : 'All Products'}
            </h1>
            <p className="text-xs text-slate-500 mt-1">
              Showing {products.length} of {pagination?.total_records ?? products.length} available items
            </p>
          </div>

          {/* Quick Search Form inside catalog */}
          <form method="GET" action="/products" className="flex items-center gap-2">
            {currentCategory && (
              <input type="hidden" name="category" value={currentCategory} />
            )}
            <div className="relative">
              <input
                type="text"
                name="search"
                defaultValue={currentSearch}
                placeholder="Filter by keyword..."
                className="pl-9 pr-3 py-2 rounded-xl bg-white border border-slate-200 text-xs text-[#0D1B2A] placeholder-slate-400 focus:outline-none focus:border-[#2196F3] w-56 sm:w-64 shadow-xs"
              />
              <Search className="w-3.5 h-3.5 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            </div>
            <button
              type="submit"
              className="px-4 py-2 rounded-xl bg-[#2196F3] text-white text-xs font-semibold hover:bg-[#0D47A1] transition-colors"
            >
              Search
            </button>
          </form>
        </div>

        {/* Main Catalog Layout */}
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
          {/* Sidebar Filter */}
          <aside className="lg:col-span-1 space-y-6">
            <div className="p-5 rounded-2xl bg-white border border-slate-200/70 shadow-xs space-y-4">
              <div className="flex items-center gap-2 text-sm font-bold text-[#0D1B2A] pb-3 border-b border-slate-100">
                <Filter className="w-4 h-4 text-[#2196F3]" />
                <span>Departments</span>
              </div>

              <div className="space-y-1">
                <Link
                  href="/products"
                  className={`block px-3 py-2 rounded-xl text-xs font-semibold transition-colors ${
                    !currentCategory
                      ? 'bg-[#E3F2FD] text-[#0D47A1]'
                      : 'text-slate-600 hover:bg-slate-50'
                  }`}
                >
                  All Categories
                </Link>
                {categories.map((cat) => {
                  const isActive =
                    currentCategory === cat.slug || currentCategory === cat.uuid
                  const queryParams = new URLSearchParams()
                  queryParams.set('category', cat.slug || cat.uuid)
                  if (currentSearch) queryParams.set('search', currentSearch)

                  return (
                    <Link
                      key={cat.uuid}
                      href={`/products?${queryParams.toString()}`}
                      className={`block px-3 py-2 rounded-xl text-xs font-medium transition-colors ${
                        isActive
                          ? 'bg-[#E3F2FD] text-[#0D47A1] font-bold'
                          : 'text-slate-600 hover:bg-slate-50'
                      }`}
                    >
                      {cat.category_name}
                    </Link>
                  )
                })}
              </div>
            </div>

            {/* Help Callout */}
            <div className="p-5 rounded-2xl bg-[#E3F2FD]/60 border border-[#90CAF9]/40 space-y-2">
              <div className="flex items-center gap-2 text-xs font-bold text-[#0D47A1]">
                <ShoppingBag className="w-4 h-4 text-[#2196F3]" />
                <span>Need assistance?</span>
              </div>
              <p className="text-[11px] text-slate-600 leading-relaxed">
                Contact our customer support team anytime for order tracking, wholesale pricing, or warranty inquiries.
              </p>
            </div>
          </aside>

          {/* Product Grid & Pagination */}
          <div className="lg:col-span-3 space-y-8">
            <ProductGrid
              products={products}
              emptyMessage={
                currentSearch
                  ? `No products found matching "${currentSearch}".`
                  : 'No products in this category yet.'
              }
            />

            {/* Pagination Controls */}
            {pagination && pagination.total_pages > 1 && (
              <div className="pt-6 border-t border-slate-200/80 flex items-center justify-between">
                <p className="text-xs text-slate-500">
                  Page <strong className="text-[#0D1B2A]">{pagination.page}</strong> of{' '}
                  <strong className="text-[#0D1B2A]">{pagination.total_pages}</strong>
                </p>

                <div className="flex items-center gap-2">
                  {pagination.has_prev ? (
                    <Link
                      href={createPageUrl(pagination.page - 1)}
                      className="inline-flex items-center gap-1 px-3.5 py-1.5 rounded-xl border border-slate-200 bg-white text-xs font-semibold text-slate-700 hover:bg-slate-50 transition-colors"
                    >
                      <ChevronLeft className="w-4 h-4" />
                      <span>Previous</span>
                    </Link>
                  ) : (
                    <button
                      disabled
                      className="inline-flex items-center gap-1 px-3.5 py-1.5 rounded-xl border border-slate-200 bg-slate-50 text-xs font-semibold text-slate-400 cursor-not-allowed"
                    >
                      <ChevronLeft className="w-4 h-4" />
                      <span>Previous</span>
                    </button>
                  )}

                  {pagination.has_next ? (
                    <Link
                      href={createPageUrl(pagination.page + 1)}
                      className="inline-flex items-center gap-1 px-3.5 py-1.5 rounded-xl border border-slate-200 bg-white text-xs font-semibold text-slate-700 hover:bg-slate-50 transition-colors"
                    >
                      <span>Next</span>
                      <ChevronRight className="w-4 h-4" />
                    </Link>
                  ) : (
                    <button
                      disabled
                      className="inline-flex items-center gap-1 px-3.5 py-1.5 rounded-xl border border-slate-200 bg-slate-50 text-xs font-semibold text-slate-400 cursor-not-allowed"
                    >
                      <span>Next</span>
                      <ChevronRight className="w-4 h-4" />
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
