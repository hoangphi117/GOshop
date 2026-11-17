import React from 'react'
import Link from 'next/link'
import { notFound } from 'next/navigation'
import {
  getProductByUuidOrSlug,
  getProducts,
  getProductReviewSummary,
  getProductReviews,
} from '@/lib/api-server'
import { ProductDetailView } from '@/components/product/product-detail-view'
import { ProductGrid } from '@/components/product/product-grid'
import { ProductViewTracker } from '@/components/product/product-view-tracker'
import { SimilarProducts } from '@/components/product/similar-products'
import { RecentlyViewedShelf } from '@/components/product/recently-viewed-shelf'
import { ChevronRight, ArrowLeft } from 'lucide-react'
import type { Metadata } from 'next'

interface ProductPageProps {
  params: Promise<{ slug: string }>
}

export async function generateMetadata({
  params,
}: ProductPageProps): Promise<Metadata> {
  const { slug } = await params
  const product = await getProductByUuidOrSlug(slug)

  if (!product) {
    return {
      title: 'Product Not Found — GOshop',
    }
  }

  return {
    title: `${product.name} — GOshop`,
    description:
      product.description?.slice(0, 160) ||
      `Buy ${product.name} at best prices on GOshop. Fast shipping, guaranteed authenticity.`,
  }
}

export const revalidate = 60

export default async function ProductDetailPage({ params }: ProductPageProps) {
  const { slug } = await params
  const product = await getProductByUuidOrSlug(slug)

  if (!product) {
    notFound()
  }

  // Fetch related products, review summary, and initial reviews in parallel
  const [relatedData, reviewSummary, reviewsData] = await Promise.all([
    getProducts({
      category: product.category_uuid || product.category_name,
      limit: 4,
    }),
    getProductReviewSummary(product.uuid),
    getProductReviews(product.uuid, { limit: 10, order_by: 'created_at', sort: 'desc' }),
  ])

  const relatedProducts = (relatedData.products || []).filter(
    (p) => p.uuid !== product.uuid
  )

  return (
    <div className="bg-[#F8FAFC] min-h-screen py-6">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 space-y-6">
        <ProductViewTracker productUuid={product.uuid} />
        {/* Breadcrumb Navigation */}
        <nav className="flex items-center gap-2 text-xs text-[#64748B]">
          <Link href="/" className="hover:text-[#007D9C] transition-colors">
            Home
          </Link>
          <ChevronRight className="w-3.5 h-3.5 text-[#94A3B8]" />
          <Link href="/products" className="hover:text-[#007D9C] transition-colors">
            Catalog
          </Link>
          {product.category_name && (
            <>
              <ChevronRight className="w-3.5 h-3.5 text-[#94A3B8]" />
              <Link
                href={`/products?category=${encodeURIComponent(product.category_uuid || '')}`}
                className="hover:text-[#007D9C] transition-colors"
              >
                {product.category_name}
              </Link>
            </>
          )}
          <ChevronRight className="w-3.5 h-3.5 text-[#94A3B8]" />
          <span className="font-semibold text-[#0F172A] truncate max-w-xs sm:max-w-md">
            {product.name}
          </span>
        </nav>

        {/* Back Link */}
        <div>
          <Link
            href="/products"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-[#64748B] hover:text-[#007D9C] transition-colors"
          >
            <ArrowLeft className="w-3.5 h-3.5" />
            <span>Back to Products</span>
          </Link>
        </div>

        {/* Interactive Product Details */}
        <ProductDetailView
          product={product}
          initialSummary={reviewSummary}
          initialReviews={reviewsData?.reviews || []}
        />

        {/* Related Products Section */}
        {relatedProducts.length > 0 && (
          <section className="pt-10 border-t border-[#E2E8F0] space-y-5">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-lg sm:text-xl font-bold text-[#0F172A] tracking-tight">
                  Frequently Paired With
                </h2>
                <p className="text-xs text-[#64748B]">
                  Complete your gear setup with matching hardware accessories.
                </p>
              </div>
              <Link
                href="/products"
                className="text-xs font-semibold text-[#007D9C] hover:text-[#00657E] transition-colors"
              >
                View Catalog →
              </Link>
            </div>

            <ProductGrid products={relatedProducts} />
          </section>
        )}

        {/* Vector nearest neighbours (server filters self + out of stock) */}
        <SimilarProducts productUuid={product.uuid} />

        {/* Local view history, resolved against the live catalog */}
        <RecentlyViewedShelf excludeUuid={product.uuid} />
      </div>
    </div>
  )
}
