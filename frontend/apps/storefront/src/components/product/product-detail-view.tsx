'use client'

import React, { useState } from 'react'
import { useRouter } from 'next/navigation'
import type { Product } from '@/lib/api-server'
import { formatCurrency } from '@/lib/format'
import { useCartStore } from '@/stores/cart-store'
import {
  ShoppingBag,
  Zap,
  ShieldCheck,
  Truck,
  RotateCcw,
  Plus,
  Minus,
} from 'lucide-react'
import { toast } from 'sonner'

interface ProductDetailViewProps {
  product: Product
}

export function ProductDetailView({ product }: ProductDetailViewProps) {
  const router = useRouter()
  const { addItem } = useCartStore()

  // Collect all available images
  const allImages =
    product.images && product.images.length > 0
      ? product.images.map((img) => img.image_url)
      : product.image_url
      ? [product.image_url]
      : []

  const [activeImageIndex, setActiveImageIndex] = useState(0)
  const [selectedVariantUuid, setSelectedVariantUuid] = useState<string | undefined>(
    product.variants?.[0]?.uuid
  )
  const [quantity, setQuantity] = useState(1)

  // Determine current active variant if variants exist
  const activeVariant = product.variants?.find(
    (v) => v.uuid === selectedVariantUuid
  )

  const currentPrice = activeVariant ? activeVariant.price : product.price
  const currentCompareAtPrice = activeVariant
    ? activeVariant.compare_at_price
    : product.compare_at_price
  const currentStock = activeVariant ? activeVariant.stock_quantity : product.stock
  const isInStock = currentStock > 0

  const hasDiscount =
    currentCompareAtPrice && currentCompareAtPrice > currentPrice
  const discountPercent = hasDiscount
    ? Math.round(
        ((currentCompareAtPrice! - currentPrice) / currentCompareAtPrice!) * 100
      )
    : 0

  const activeImage = allImages[activeImageIndex] || ''

  const handleAddToCart = (redirectAfter = false) => {
    if (!isInStock) {
      toast.error('This product or variant is currently out of stock')
      return
    }

    addItem({
      productUuid: product.uuid,
      variantUuid: activeVariant?.uuid,
      name: product.name,
      price: currentPrice,
      compareAtPrice: currentCompareAtPrice,
      image: activeImage,
      quantity,
      selectedSize: activeVariant?.size,
      selectedColor: activeVariant?.color,
      sku: activeVariant?.sku || product.sku,
      stockQuantity: currentStock,
    })

    toast.success(`Added ${quantity} item(s) to your cart!`)

    if (redirectAfter) {
      router.push('/checkout')
    }
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-12 gap-10">
      {/* Left: Gallery (5 cols) */}
      <div className="lg:col-span-6 space-y-4">
        {/* Main Image Display */}
        <div className="relative aspect-square w-full rounded-3xl overflow-hidden bg-white border border-slate-200/80 shadow-xs flex items-center justify-center">
          {activeImage ? (
            <img
              src={activeImage}
              alt={product.name}
              className="w-full h-full object-cover object-center"
            />
          ) : (
            <div className="flex flex-col items-center justify-center text-slate-400 p-8">
              <ShoppingBag className="w-16 h-16 stroke-[1.2] text-slate-300 mb-2" />
              <span className="text-sm font-medium">No Image Available</span>
            </div>
          )}

          {hasDiscount && (
            <span className="absolute top-4 left-4 px-3 py-1 rounded-full bg-rose-500 text-white text-xs font-bold shadow-md">
              Save {discountPercent}%
            </span>
          )}
        </div>

        {/* Thumbnail Selector */}
        {allImages.length > 1 && (
          <div className="flex items-center gap-3 overflow-x-auto pb-2">
            {allImages.map((imgUrl, idx) => (
              <button
                key={idx}
                onClick={() => setActiveImageIndex(idx)}
                className={`relative w-20 h-20 rounded-2xl overflow-hidden shrink-0 border-2 transition-all ${
                  activeImageIndex === idx
                    ? 'border-[#2196F3] shadow-md ring-2 ring-[#2196F3]/20'
                    : 'border-slate-200 opacity-70 hover:opacity-100'
                }`}
              >
                <img
                  src={imgUrl}
                  alt={`Thumbnail ${idx + 1}`}
                  className="w-full h-full object-cover"
                />
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Right: Info & Purchase Controls (7 cols) */}
      <div className="lg:col-span-6 space-y-6">
        {/* Category & Stock Tag */}
        <div className="flex items-center gap-3">
          {product.category_name && (
            <span className="px-3 py-1 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-bold">
              {product.category_name}
            </span>
          )}
          <span
            className={`px-3 py-1 rounded-full text-xs font-bold inline-flex items-center gap-1 ${
              isInStock
                ? 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                : 'bg-rose-50 text-rose-700 border border-rose-200'
            }`}
          >
            <span
              className={`w-1.5 h-1.5 rounded-full ${
                isInStock ? 'bg-emerald-500' : 'bg-rose-500'
              }`}
            />
            {isInStock ? `In Stock (${currentStock} left)` : 'Out of Stock'}
          </span>
        </div>

        {/* Product Title */}
        <h1 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight leading-tight">
          {product.name}
        </h1>

        {/* SKU */}
        {(activeVariant?.sku || product.sku) && (
          <p className="text-xs text-slate-500 font-mono">
            SKU: <span className="font-semibold text-slate-700">{activeVariant?.sku || product.sku}</span>
          </p>
        )}

        {/* Pricing Block */}
        <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200/80 flex items-baseline gap-4">
          <span className="text-3xl font-black text-[#0D47A1]">
            {formatCurrency(currentPrice)}
          </span>
          {hasDiscount && (
            <span className="text-sm text-slate-400 line-through">
              {formatCurrency(currentCompareAtPrice)}
            </span>
          )}
        </div>

        {/* Variant Selector (if product has variants) */}
        {product.variants && product.variants.length > 0 && (
          <div className="space-y-3 pt-2">
            <label className="text-xs font-bold text-[#0D1B2A] uppercase tracking-wider block">
              Choose Option
            </label>
            <div className="flex flex-wrap gap-2">
              {product.variants.map((variant) => {
                const isSelected = variant.uuid === selectedVariantUuid
                const isVariantAvailable = variant.stock_quantity > 0
                return (
                  <button
                    key={variant.uuid}
                    onClick={() => setSelectedVariantUuid(variant.uuid)}
                    className={`px-4 py-2.5 rounded-xl text-xs font-semibold border transition-all ${
                      isSelected
                        ? 'border-[#2196F3] bg-[#E3F2FD] text-[#0D47A1] shadow-xs'
                        : isVariantAvailable
                        ? 'border-slate-200 bg-white text-slate-700 hover:border-slate-300'
                        : 'border-slate-200 bg-slate-100 text-slate-400 line-through'
                    }`}
                  >
                    <span>
                      {variant.size || variant.color || variant.sku}
                    </span>
                    <span className="ml-1.5 opacity-80">
                      ({formatCurrency(variant.price)})
                    </span>
                  </button>
                )
              })}
            </div>
          </div>
        )}

        {/* Quantity Controller & CTAs */}
        <div className="space-y-4 pt-4 border-t border-slate-100">
          <div className="flex items-center gap-4">
            <label className="text-xs font-bold text-[#0D1B2A] uppercase tracking-wider">
              Quantity
            </label>
            <div className="flex items-center border border-slate-200 rounded-xl bg-white overflow-hidden shadow-xs">
              <button
                onClick={() => setQuantity((q) => Math.max(1, q - 1))}
                className="p-2.5 hover:bg-slate-100 transition-colors text-slate-600"
                aria-label="Decrease quantity"
              >
                <Minus className="w-4 h-4" />
              </button>
              <span className="px-4 text-sm font-bold text-[#0D1B2A] min-w-[36px] text-center">
                {quantity}
              </span>
              <button
                onClick={() =>
                  setQuantity((q) => Math.min(currentStock || 99, q + 1))
                }
                className="p-2.5 hover:bg-slate-100 transition-colors text-slate-600"
                aria-label="Increase quantity"
              >
                <Plus className="w-4 h-4" />
              </button>
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
            <button
              onClick={() => handleAddToCart(false)}
              disabled={!isInStock}
              className={`w-full py-3.5 px-6 rounded-2xl text-sm font-bold flex items-center justify-center gap-2 transition-all shadow-md ${
                isInStock
                  ? 'bg-[#2196F3] text-white hover:bg-[#0D47A1] hover:scale-[1.01] active:scale-[0.99]'
                  : 'bg-slate-200 text-slate-400 cursor-not-allowed'
              }`}
            >
              <ShoppingBag className="w-4 h-4" />
              <span>Add to Cart</span>
            </button>

            <button
              onClick={() => handleAddToCart(true)}
              disabled={!isInStock}
              className={`w-full py-3.5 px-6 rounded-2xl text-sm font-bold flex items-center justify-center gap-2 transition-all shadow-md ${
                isInStock
                  ? 'bg-[#0D47A1] text-white hover:bg-[#1565C0] hover:scale-[1.01] active:scale-[0.99]'
                  : 'bg-slate-100 text-slate-400 cursor-not-allowed'
              }`}
            >
              <Zap className="w-4 h-4 text-[#90CAF9]" />
              <span>Buy Now</span>
            </button>
          </div>
        </div>

        {/* Value Props Pills */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-4 border-t border-slate-100 text-xs text-slate-600">
          <div className="flex items-center gap-2 p-3 rounded-xl bg-slate-50 border border-slate-100">
            <Truck className="w-4 h-4 text-[#2196F3] shrink-0" />
            <span>Fast GHN Delivery</span>
          </div>
          <div className="flex items-center gap-2 p-3 rounded-xl bg-slate-50 border border-slate-100">
            <ShieldCheck className="w-4 h-4 text-[#2196F3] shrink-0" />
            <span>100% Genuine</span>
          </div>
          <div className="flex items-center gap-2 p-3 rounded-xl bg-slate-50 border border-slate-100">
            <RotateCcw className="w-4 h-4 text-[#2196F3] shrink-0" />
            <span>30-Day Return</span>
          </div>
        </div>

        {/* Description & Details */}
        <div className="pt-6 border-t border-slate-100 space-y-3">
          <h3 className="text-base font-bold text-[#0D1B2A]">Description</h3>
          <div className="prose prose-sm max-w-none text-slate-600 leading-relaxed text-sm whitespace-pre-line">
            {product.description ||
              'No additional details provided for this product. All products undergo strict authenticity checks before shipment.'}
          </div>
        </div>
      </div>
    </div>
  )
}
