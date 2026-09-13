'use client'

import React, { useState } from 'react'
import { useRouter } from 'next/navigation'
import type { Product, Review, ReviewSummary } from '@/lib/api-server'
import { formatCurrency } from '@/lib/format'
import { getImageUrl } from '@/lib/utils'
import { useCartStore } from '@/stores/cart-store'
import { useAuthStore } from '@/stores/auth-store'
import { ProductReviewsSection } from '@/components/review/product-reviews-section'
import {
  ShoppingCart,
  Package,
  Zap,
  ShieldCheck,
  Truck,
  RotateCcw,
  Plus,
  Minus,
  Star,
  Share2,
  Heart,
  CheckCircle2,
} from 'lucide-react'
import { toast } from 'sonner'

interface ProductDetailViewProps {
  product: Product
  initialSummary?: ReviewSummary | null
  initialReviews?: Review[]
}

export function ProductDetailView({
  product,
  initialSummary,
  initialReviews,
}: ProductDetailViewProps) {
  const router = useRouter()
  const { addItem, setBuyNowItem } = useCartStore()
  const { accessToken } = useAuthStore()

  // Collect all available images
  const allImages = (
    product.images && product.images.length > 0
      ? product.images.map((img) => getImageUrl(img.image_url))
      : product.image_url
      ? [getImageUrl(product.image_url)]
      : []
  ).filter(Boolean)

  const [activeImageIndex, setActiveImageIndex] = useState(0)
  const [selectedVariantUuid, setSelectedVariantUuid] = useState<string | undefined>(
    product.variants?.[0]?.uuid
  )
  const [quantity, setQuantity] = useState(1)
  const [activeTab, setActiveTab] = useState<'desc' | 'specs' | 'shipping' | 'reviews'>('desc')
  const [isWishlisted, setIsWishlisted] = useState(false)

  // Active variant logic
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
        ((currentCompareAtPrice - currentPrice) / currentCompareAtPrice) * 100
      )
    : 0
  const savedAmount = hasDiscount
    ? currentCompareAtPrice! - currentPrice
    : 0

  const activeImage = allImages[activeImageIndex] || ''

  const handleAddToCart = () => {
    if (!accessToken) {
      toast.error('Please sign in to add items to your cart.')
      const currentUrl =
        typeof window !== 'undefined'
          ? window.location.pathname
          : `/products/${product.slug || product.uuid}`
      router.push(`/login?redirect=${encodeURIComponent(currentUrl)}`)
      return
    }

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

    toast.success(`Added ${quantity} "${product.name}" to cart!`)
  }

  const handleBuyNow = () => {
    if (!accessToken) {
      toast.error('Please sign in to proceed to checkout.')
      const currentUrl =
        typeof window !== 'undefined'
          ? window.location.pathname
          : `/products/${product.slug || product.uuid}`
      router.push(`/login?redirect=${encodeURIComponent(currentUrl)}`)
      return
    }

    if (!isInStock) {
      toast.error('This product or variant is currently out of stock')
      return
    }

    const buyNowId = activeVariant?.uuid
      ? `${product.uuid}-${activeVariant.uuid}`
      : `${product.uuid}-${activeVariant?.size || 'default'}-${activeVariant?.color || 'default'}`

    setBuyNowItem({
      id: buyNowId,
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
      selected: true,
    })

    router.push('/checkout?buyNow=1')
  }

  const handleShare = () => {
    if (typeof window !== 'undefined') {
      navigator.clipboard.writeText(window.location.href)
      toast.success('Product link copied to clipboard!')
    }
  }

  const toggleWishlist = () => {
    setIsWishlisted(!isWishlisted)
    if (!isWishlisted) {
      toast.success('Added to your Wishlist!')
    } else {
      toast.info('Removed from your Wishlist.')
    }
  }

  return (
    <div className="space-y-12">
      {/* Upper Main Section: Gallery + Purchase Controls */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-10">
        {/* Left: Gallery (6 cols) */}
        <div className="lg:col-span-6 space-y-4">
          {/* Main Showcase Image */}
          <div className="relative aspect-square w-full rounded-3xl overflow-hidden bg-white border border-slate-200/80 shadow-xs flex items-center justify-center group">
            {activeImage ? (
              <img
                src={activeImage}
                alt={product.name}
                className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-300"
              />
            ) : (
              <div className="flex flex-col items-center justify-center text-slate-400 p-8">
                <Package className="w-16 h-16 stroke-[1.2] text-slate-300 mb-2" />
                <span className="text-sm font-medium">No Image Available</span>
              </div>
            )}

            {hasDiscount && (
              <span className="absolute top-4 left-4 px-3 py-1 rounded-full bg-rose-500 text-white text-xs font-bold shadow-md">
                Save {discountPercent}%
              </span>
            )}

            {/* Quick Actions (Wishlist & Share) */}
            <div className="absolute top-4 right-4 flex items-center gap-2">
              <button
                onClick={toggleWishlist}
                className={`w-9 h-9 rounded-full backdrop-blur-md flex items-center justify-center transition-all shadow-md ${
                  isWishlisted
                    ? 'bg-rose-500 text-white'
                    : 'bg-white/90 text-slate-600 hover:bg-white hover:text-rose-500'
                }`}
                title="Save to wishlist"
              >
                <Heart className={`w-4 h-4 ${isWishlisted ? 'fill-white' : ''}`} />
              </button>
              <button
                onClick={handleShare}
                className="w-9 h-9 rounded-full bg-white/90 backdrop-blur-md text-slate-600 shadow-md flex items-center justify-center hover:bg-white hover:text-[#2196F3] transition-all"
                title="Share product link"
              >
                <Share2 className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Thumbnail Strip */}
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

        {/* Right: Product Details & Buying Actions (6 cols) */}
        <div className="lg:col-span-6 space-y-6">
          {/* Department & Stock Badge */}
          <div className="flex flex-wrap items-center gap-3">
            {product.category_name && (
              <span className="px-3.5 py-1 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-bold uppercase tracking-wider">
                {product.category_name}
              </span>
            )}
            <span
              className={`px-3 py-1 rounded-full text-xs font-bold inline-flex items-center gap-1.5 ${
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
              {isInStock ? `In Stock (${currentStock} units available)` : 'Sold Out'}
            </span>
          </div>

          {/* Title */}
          <h1 className="text-2xl sm:text-3xl lg:text-4xl font-black text-[#0D1B2A] tracking-tight leading-tight">
            {product.name}
          </h1>

          {/* Rating & SKU Bar */}
          <div className="flex flex-wrap items-center gap-4 text-xs text-slate-500 pb-2 border-b border-slate-100">
            <button
              type="button"
              onClick={() => setActiveTab('reviews')}
              className="flex items-center gap-1.5 hover:opacity-80 transition-opacity cursor-pointer text-left"
            >
              <div className="flex text-amber-400">
                {[...Array(5)].map((_, i) => (
                  <Star
                    key={i}
                    className={`w-4 h-4 ${
                      i < Math.round(initialSummary?.avg_rating || 5)
                        ? 'fill-amber-400 text-amber-400'
                        : 'text-slate-200'
                    }`}
                  />
                ))}
              </div>
              <span className="font-bold text-[#0D1B2A]">
                {initialSummary && initialSummary.avg_rating > 0
                  ? initialSummary.avg_rating.toFixed(1)
                  : '5.0'}
              </span>
              <span className="text-slate-400">
                ({initialSummary?.total_reviews || 0} verified reviews)
              </span>
            </button>

            <span>•</span>

            {(activeVariant?.sku || product.sku) && (
              <p className="font-mono">
                SKU: <strong className="text-slate-800">{activeVariant?.sku || product.sku}</strong>
              </p>
            )}
          </div>

          {/* Price Highlight Card */}
          <div className="p-5 rounded-2xl bg-gradient-to-r from-slate-50 to-[#E3F2FD]/30 border border-slate-200/80 flex flex-wrap items-baseline gap-4">
            <span className="text-3xl sm:text-4xl font-black text-[#0D47A1]">
              {formatCurrency(currentPrice)}
            </span>

            {hasDiscount && (
              <div className="flex items-center gap-2">
                <span className="text-base text-slate-400 line-through">
                  {formatCurrency(currentCompareAtPrice)}
                </span>
                <span className="px-2.5 py-0.5 rounded-md bg-emerald-100 text-emerald-800 text-xs font-bold">
                  Save {formatCurrency(savedAmount)}
                </span>
              </div>
            )}
          </div>

          {/* Variant Selector */}
          {product.variants && product.variants.length > 0 && (
            <div className="space-y-3 pt-1">
              <label className="text-xs font-bold text-[#0D1B2A] uppercase tracking-wider block">
                Choose Variant / Configuration:
              </label>
              <div className="flex flex-wrap gap-2.5">
                {product.variants.map((variant) => {
                  const isSelected = variant.uuid === selectedVariantUuid
                  const isVariantAvailable = variant.stock_quantity > 0
                  return (
                    <button
                      key={variant.uuid}
                      onClick={() => setSelectedVariantUuid(variant.uuid)}
                      className={`px-4 py-2.5 rounded-xl text-xs font-semibold border transition-all flex items-center gap-2 ${
                        isSelected
                          ? 'border-[#2196F3] bg-[#E3F2FD] text-[#0D47A1] shadow-xs ring-2 ring-[#2196F3]/20'
                          : isVariantAvailable
                          ? 'border-slate-200 bg-white text-slate-700 hover:border-slate-300'
                          : 'border-slate-200 bg-slate-100 text-slate-400 line-through'
                      }`}
                    >
                      <span>{variant.size || variant.color || variant.sku}</span>
                      <span className="opacity-75">
                        ({formatCurrency(variant.price)})
                      </span>
                    </button>
                  )
                })}
              </div>
            </div>
          )}

          {/* Quantity & Actions */}
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
                type="button"
                onClick={handleAddToCart}
                disabled={!isInStock}
                className={`w-full py-4 px-6 rounded-2xl text-sm font-bold flex items-center justify-center gap-2 transition-all shadow-md ${
                  isInStock
                    ? 'bg-[#2196F3] text-white hover:bg-[#0D47A1] hover:scale-[1.01] active:scale-[0.99] cursor-pointer'
                    : 'bg-slate-200 text-slate-400 cursor-not-allowed'
                }`}
              >
                <ShoppingCart className="w-4 h-4" />
                <span>Add to Cart</span>
              </button>

              <button
                type="button"
                onClick={handleBuyNow}
                disabled={!isInStock}
                className={`w-full py-4 px-6 rounded-2xl text-sm font-bold flex items-center justify-center gap-2 transition-all shadow-md ${
                  isInStock
                    ? 'bg-[#0D47A1] text-white hover:bg-[#1565C0] hover:scale-[1.01] active:scale-[0.99] cursor-pointer'
                    : 'bg-slate-100 text-slate-400 cursor-not-allowed'
                }`}
              >
                <Zap className="w-4 h-4 text-[#90CAF9]" />
                <span>Buy Now</span>
              </button>
            </div>
          </div>

          {/* Quick Value Props Cards */}
          <div className="grid grid-cols-3 gap-3 pt-3 text-xs text-slate-600">
            <div className="p-3 rounded-2xl bg-white border border-slate-200/70 shadow-2xs text-center space-y-1">
              <Truck className="w-5 h-5 text-[#2196F3] mx-auto" />
              <p className="font-bold text-[#0D1B2A]">Fast Delivery</p>
              <p className="text-[10px] text-slate-400">1 - 3 business days</p>
            </div>
            <div className="p-3 rounded-2xl bg-white border border-slate-200/70 shadow-2xs text-center space-y-1">
              <ShieldCheck className="w-5 h-5 text-[#2196F3] mx-auto" />
              <p className="font-bold text-[#0D1B2A]">100% Genuine</p>
              <p className="text-[10px] text-slate-400">Official warranty</p>
            </div>
            <div className="p-3 rounded-2xl bg-white border border-slate-200/70 shadow-2xs text-center space-y-1">
              <RotateCcw className="w-5 h-5 text-[#2196F3] mx-auto" />
              <p className="font-bold text-[#0D1B2A]">Easy Returns</p>
              <p className="text-[10px] text-slate-400">30 days return</p>
            </div>
          </div>
        </div>
      </div>

      {/* Lower Section: Tabs (Description, Specifications, Shipping, Reviews) */}
      <div className="p-6 sm:p-8 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
        {/* Tabs Bar */}
        <div className="flex border-b border-slate-200 overflow-x-auto gap-2 sm:gap-6">
          {[
            { id: 'desc', label: 'Product Overview' },
            { id: 'specs', label: 'Technical Specifications' },
            { id: 'shipping', label: 'Shipping & Warranty' },
            {
              id: 'reviews',
              label: `Customer Reviews (${initialSummary?.total_reviews ?? 0})`,
            },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as 'desc' | 'specs' | 'shipping' | 'reviews')}
              className={`pb-4 px-2 text-sm font-bold transition-all relative shrink-0 ${
                activeTab === tab.id
                  ? 'text-[#0D47A1]'
                  : 'text-slate-500 hover:text-slate-800'
              }`}
            >
              <span>{tab.label}</span>
              {activeTab === tab.id && (
                <div className="absolute bottom-0 inset-x-0 h-0.5 bg-[#2196F3] rounded-full" />
              )}
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <div className="pt-2">
          {activeTab === 'desc' && (
            <div className="space-y-6 max-w-3xl">
              <div>
                <h3 className="text-lg font-bold text-[#0D1B2A] mb-3">
                  About {product.name}
                </h3>
                <div className="text-sm text-slate-600 leading-relaxed whitespace-pre-line">
                  {product.description ||
                    'Experience top-tier quality and premium design with this authentic product from GOshop. Designed to elevate your everyday workflow and lifestyle.'}
                </div>
              </div>

              <div className="space-y-3 pt-4 border-t border-slate-100">
                <h4 className="text-sm font-bold text-[#0D1B2A]">
                  Key Highlights & Features:
                </h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs text-slate-600">
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
                    <span>Premium build quality and durable finish</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
                    <span>Tested and certified for long-lasting reliability</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
                    <span>Covered by GOshop 100% genuine guarantee</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
                    <span>Fast delivery with secure insured packaging</span>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'specs' && (
            <div className="max-w-2xl">
              <div className="rounded-2xl border border-slate-200 overflow-hidden divide-y divide-slate-100 text-xs">
                <div className="p-3.5 bg-slate-50 flex justify-between">
                  <span className="font-semibold text-slate-600">Product Name</span>
                  <span className="font-bold text-[#0D1B2A]">{product.name}</span>
                </div>
                <div className="p-3.5 bg-white flex justify-between">
                  <span className="font-semibold text-slate-600">SKU Number</span>
                  <span className="font-mono text-slate-800">{product.sku || 'N/A'}</span>
                </div>
                <div className="p-3.5 bg-slate-50 flex justify-between">
                  <span className="font-semibold text-slate-600">Department</span>
                  <span className="text-[#0D47A1] font-bold">{product.category_name || 'General'}</span>
                </div>
                <div className="p-3.5 bg-white flex justify-between">
                  <span className="font-semibold text-slate-600">Availability</span>
                  <span className="text-emerald-600 font-bold">
                    {isInStock ? `In Stock (${currentStock} items)` : 'Out of Stock'}
                  </span>
                </div>
                <div className="p-3.5 bg-slate-50 flex justify-between">
                  <span className="font-semibold text-slate-600">Warranty Term</span>
                  <span className="text-slate-800 font-medium">12 Months Manufacturer Warranty</span>
                </div>
                <div className="p-3.5 bg-white flex justify-between">
                  <span className="font-semibold text-slate-600">Authenticity</span>
                  <span className="text-emerald-600 font-bold">100% Genuine Guaranteed</span>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'shipping' && (
            <div className="space-y-6 max-w-3xl text-sm text-slate-600">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="p-5 rounded-2xl bg-slate-50 border border-slate-200/80 space-y-2">
                  <div className="flex items-center gap-2 font-bold text-[#0D1B2A]">
                    <Truck className="w-5 h-5 text-[#2196F3]" />
                    <span>Nationwide GHN Express</span>
                  </div>
                  <p className="text-xs text-slate-500 leading-relaxed">
                    Orders in Ho Chi Minh City & Hanoi are delivered within 24 hours.
                    Other provinces: 2 - 3 business days.
                  </p>
                </div>

                <div className="p-5 rounded-2xl bg-slate-50 border border-slate-200/80 space-y-2">
                  <div className="flex items-center gap-2 font-bold text-[#0D1B2A]">
                    <RotateCcw className="w-5 h-5 text-[#2196F3]" />
                    <span>30-Day Free Return Policy</span>
                  </div>
                  <p className="text-xs text-slate-500 leading-relaxed">
                    If there is any defect or damage during shipment, contact our support team
                    for immediate exchange or 100% money-back refund.
                  </p>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'reviews' && (
            <div className="max-w-4xl">
              <ProductReviewsSection
                productUuid={product.uuid}
                productName={product.name}
                productImageUrl={activeImage}
                initialSummary={initialSummary}
                initialReviews={initialReviews}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
