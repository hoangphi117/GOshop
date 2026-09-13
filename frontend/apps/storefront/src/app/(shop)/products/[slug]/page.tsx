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
    <div className="bg-[#FAFBFC] min-h-screen py-8">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 space-y-8">
        {/* Breadcrumb Navigation */}
        <nav className="flex items-center gap-2 text-xs text-slate-500">
          <Link href="/" className="hover:text-[#2196F3] transition-colors">
            Home
          </Link>
          <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
          <Link href="/products" className="hover:text-[#2196F3] transition-colors">
            Catalog
          </Link>
          {product.category_name && (
            <>
              <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
              <Link
                href={`/products?category=${encodeURIComponent(product.category_uuid || '')}`}
                className="hover:text-[#2196F3] transition-colors"
              >
                {product.category_name}
              </Link>
            </>
          )}
          <ChevronRight className="w-3.5 h-3.5 text-slate-400" />
          <span className="font-semibold text-[#0D1B2A] truncate max-w-xs sm:max-w-md">
            {product.name}
          </span>
        </nav>

        {/* Back Link */}
        <div>
          <Link
            href="/products"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-600 hover:text-[#2196F3] transition-colors"
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
          <section className="pt-16 border-t border-slate-200/80 space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl sm:text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
                You May Also Like
              </h2>
              <Link
                href="/products"
                className="text-xs font-semibold text-[#0D47A1] hover:text-[#2196F3] transition-colors"
              >
                View More
              </Link>
            </div>

            <ProductGrid products={relatedProducts} />
          </section>
        )}
      </div>
    </div>
  )
}
