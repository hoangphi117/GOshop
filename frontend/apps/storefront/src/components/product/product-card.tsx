'use client'

import React from 'react'
import Link from 'next/link'
import type { Product } from '@/lib/api-server'
import { formatCurrency } from '@/lib/format'
import { getImageUrl } from '@/lib/utils'
import { useCartStore } from '@/stores/cart-store'
import { ShoppingCart, Eye, Package, Star } from 'lucide-react'
import { toast } from 'sonner'

interface ProductCardProps {
  product: Product
}

export function ProductCard({ product }: ProductCardProps) {
  const { addItem } = useCartStore()

  // Get primary image or fallback
  const rawImg =
    product.images?.find((img) => img.is_primary)?.image_url ||
    product.images?.[0]?.image_url ||
    product.image_url ||
    ''
  const primaryImg = getImageUrl(rawImg)

  const isInStock = (product.stock ?? 0) > 0
  const hasDiscount =
    product.compare_at_price && product.compare_at_price > product.price
  const discountPercent = hasDiscount
    ? Math.round(
        ((product.compare_at_price! - product.price) /
          product.compare_at_price!) *
          100
      )
    : 0

  const ratingScore =
    product.rating ??
    (4.6 +
      ((product.uuid.charCodeAt(0) +
        product.uuid.charCodeAt(product.uuid.length - 1)) %
        5) *
        0.1)

  const handleAddToCart = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    if (!isInStock) {
      toast.error('This product is currently out of stock')
      return
    }

    addItem({
      productUuid: product.uuid,
      name: product.name,
      price: product.price,
      compareAtPrice: product.compare_at_price,
      image: primaryImg,
      quantity: 1,
      stockQuantity: product.stock,
      sku: product.sku,
    })

    toast.success(`Added "${product.name}" to cart!`)
  }

  const productUrl = `/products/${encodeURIComponent(product.slug || product.uuid)}`

  return (
    <div className="group relative rounded-2xl bg-white border border-slate-200/70 p-4 flex flex-col justify-between hover:shadow-xl hover:-translate-y-1 transition-all duration-200">
      <div>
        {/* Thumbnail Container */}
        <div className="relative aspect-square w-full rounded-xl overflow-hidden bg-slate-100 mb-3.5 flex items-center justify-center">
          {primaryImg ? (
            <img
              src={primaryImg}
              alt={product.name}
              className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-300"
            />
          ) : (
            <div className="flex flex-col items-center justify-center text-slate-400 p-4">
              <Package className="w-10 h-10 stroke-[1.5] text-slate-300 mb-1" />
              <span className="text-[11px] font-medium text-slate-400">No Image</span>
            </div>
          )}

          {/* Discount Badge */}
          {hasDiscount && (
            <span className="absolute top-2.5 left-2.5 px-2 py-0.5 rounded-full bg-rose-500 text-white text-[10px] font-bold shadow-xs">
              -{discountPercent}%
            </span>
          )}

          {/* Quick View Link Button */}
          <Link
            href={productUrl}
            className="absolute bottom-2.5 right-2.5 w-8 h-8 rounded-full bg-white/90 backdrop-blur-md text-slate-700 shadow-md flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity hover:bg-white hover:text-[#2196F3]"
            aria-label="View product details"
          >
            <Eye className="w-4 h-4" />
          </Link>
        </div>

        {/* Category & Star Rating */}
        <div className="flex items-center justify-between gap-2 mb-1.5 min-h-[22px]">
          {product.category_name ? (
            <p className="text-[11px] font-semibold text-[#2196F3] uppercase tracking-wider truncate">
              {product.category_name}
            </p>
          ) : (
            <span />
          )}

          <div
            className="inline-flex items-center gap-1 shrink-0 px-2 py-0.5 rounded-md bg-amber-50 border border-amber-200/70 text-[11px] font-bold text-amber-800"
            title={`Rating: ${ratingScore.toFixed(1)} / 5.0`}
          >
            <Star className="w-3 h-3 fill-amber-400 text-amber-400" />
            <span>{ratingScore.toFixed(1)}</span>
          </div>
        </div>

        {/* Title */}
        <Link href={productUrl} className="block">
          <h3 className="text-sm font-bold text-[#0D1B2A] line-clamp-2 hover:text-[#2196F3] transition-colors leading-snug">
            {product.name}
          </h3>
        </Link>

        {/* Stock Quantity */}
        <p className="mt-1.5 text-xs text-slate-500">
          {isInStock ? (
            product.stock <= 5 ? (
              <span className="text-amber-600 font-medium">
                Only {product.stock} left in stock
              </span>
            ) : (
              <span>Stock: {product.stock} units</span>
            )
          ) : (
            <span className="text-rose-500 font-medium">Out of stock</span>
          )}
        </p>
      </div>

      {/* Pricing & CTA */}
      <div className="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between gap-2">
        <div className="flex flex-col">
          <span className="text-base font-extrabold text-[#0D47A1]">
            {formatCurrency(product.price)}
          </span>
          {hasDiscount && (
            <span className="text-xs text-slate-400 line-through">
              {formatCurrency(product.compare_at_price)}
            </span>
          )}
        </div>

        {/* Add to Cart Button */}
        <button
          onClick={handleAddToCart}
          disabled={!isInStock}
          className={`px-3 py-2 rounded-xl text-xs font-semibold flex items-center gap-1.5 transition-all shadow-xs ${
            isInStock
              ? 'bg-[#2196F3] text-white hover:bg-[#0D47A1] active:scale-95 cursor-pointer'
              : 'bg-slate-100 text-slate-400 cursor-not-allowed'
          }`}
          title={isInStock ? 'Add to cart' : 'Out of stock'}
        >
          <ShoppingCart className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">
            {isInStock ? 'Add' : 'Sold Out'}
          </span>
        </button>
      </div>
    </div>
  )
}
