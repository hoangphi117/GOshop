'use client'

import React from 'react'
import Link from 'next/link'
import type { Product } from '@/lib/api-server'
import { formatCurrency } from '@/lib/format'
import { useCartStore } from '@/stores/cart-store'
import { ShoppingBag, Eye } from 'lucide-react'
import { toast } from 'sonner'

interface ProductCardProps {
  product: Product
}

export function ProductCard({ product }: ProductCardProps) {
  const { addItem } = useCartStore()

  // Get primary image or fallback
  const primaryImg =
    product.images?.find((img) => img.is_primary)?.image_url ||
    product.images?.[0]?.image_url ||
    product.image_url ||
    ''

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
              <ShoppingBag className="w-10 h-10 stroke-[1.5] text-slate-300 mb-1" />
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

        {/* Category */}
        {product.category_name && (
          <p className="text-[11px] font-semibold text-[#2196F3] uppercase tracking-wider mb-1">
            {product.category_name}
          </p>
        )}

        {/* Title */}
        <Link href={productUrl} className="block">
          <h3 className="text-sm font-bold text-[#0D1B2A] line-clamp-2 hover:text-[#2196F3] transition-colors leading-snug">
            {product.name}
          </h3>
        </Link>
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
              ? 'bg-[#2196F3] text-white hover:bg-[#0D47A1] active:scale-95'
              : 'bg-slate-100 text-slate-400 cursor-not-allowed'
          }`}
          title={isInStock ? 'Add to cart' : 'Out of stock'}
        >
          <ShoppingBag className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">
            {isInStock ? 'Add' : 'Sold Out'}
          </span>
        </button>
      </div>
    </div>
  )
}
