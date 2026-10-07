"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import type { Product, Review, ReviewSummary } from "@/lib/api-server";
import { formatCurrency } from "@/lib/format";
import { getImageUrl } from "@/lib/utils";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { ProductReviewsSection } from "@/components/review/product-reviews-section";
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
  CheckCircle2,
} from "lucide-react";
import { toast } from "sonner";

interface ProductDetailViewProps {
  product: Product;
  initialSummary?: ReviewSummary | null;
  initialReviews?: Review[];
}

export function ProductDetailView({
  product,
  initialSummary,
  initialReviews,
}: ProductDetailViewProps) {
  const router = useRouter();
  const { addItem, setBuyNowItem } = useCartStore();
  const { accessToken } = useAuthStore();

  // Collect all available images
  const allImages = (
    product.images && product.images.length > 0
      ? product.images.map((img) => getImageUrl(img.image_url))
      : product.image_url
        ? [getImageUrl(product.image_url)]
        : []
  ).filter(Boolean);

  const [activeImageIndex, setActiveImageIndex] = useState(0);
  const [selectedVariantUuid, setSelectedVariantUuid] = useState<
    string | undefined
  >(product.variants?.[0]?.uuid);
  const [quantity, setQuantity] = useState(1);
  const [activeTab, setActiveTab] = useState<"desc" | "specs">("desc");

  const scrollToReviews = () => {
    const el = document.getElementById("customer-reviews");
    if (el) {
      const top = el.getBoundingClientRect().top + window.scrollY - 80;
      window.scrollTo({ top, behavior: "smooth" });
    }
  };

  // Active variant logic
  const activeVariant = product.variants?.find(
    (v) => v.uuid === selectedVariantUuid,
  );

  const currentPrice = activeVariant ? activeVariant.price : product.price;
  const currentCompareAtPrice = activeVariant
    ? activeVariant.compare_at_price
    : product.compare_at_price;
  const currentStock = activeVariant
    ? activeVariant.stock_quantity
    : product.stock;
  const isInStock = currentStock > 0;

  const hasDiscount =
    currentCompareAtPrice && currentCompareAtPrice > currentPrice;
  const discountPercent = hasDiscount
    ? Math.round(
        ((currentCompareAtPrice - currentPrice) / currentCompareAtPrice) * 100,
      )
    : 0;
  const savedAmount = hasDiscount ? currentCompareAtPrice! - currentPrice : 0;

  const activeImage = allImages[activeImageIndex] || "";

  const handleAddToCart = () => {
    if (!accessToken) {
      toast.error("Please sign in to add items to your cart.");
      const currentUrl =
        typeof window !== "undefined"
          ? window.location.pathname
          : `/products/${product.slug || product.uuid}`;
      router.push(`/login?redirect=${encodeURIComponent(currentUrl)}`);
      return;
    }

    if (!isInStock) {
      toast.error("This product or variant is currently out of stock");
      return;
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
    });

    toast.success(`Added ${quantity} "${product.name}" to cart!`);
  };

  const handleBuyNow = () => {
    if (!accessToken) {
      toast.error("Please sign in to proceed to checkout.");
      const currentUrl =
        typeof window !== "undefined"
          ? window.location.pathname
          : `/products/${product.slug || product.uuid}`;
      router.push(`/login?redirect=${encodeURIComponent(currentUrl)}`);
      return;
    }

    if (!isInStock) {
      toast.error("This product or variant is currently out of stock");
      return;
    }

    const buyNowId = activeVariant?.uuid
      ? `${product.uuid}-${activeVariant.uuid}`
      : `${product.uuid}-${activeVariant?.size || "default"}-${activeVariant?.color || "default"}`;

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
    });

    router.push("/checkout?buyNow=1");
  };

  return (
    <div className="space-y-8">
      {/* Upper Main Section: Gallery + Purchase Controls */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 lg:gap-8 items-start">
        {/* Left: Gallery Showcase (7 cols) */}
        <div className="lg:col-span-7 space-y-3">
          {/* Main Showcase Image */}
          <div className="relative aspect-[4/3] sm:aspect-[16/10] lg:aspect-[4/3] w-full rounded-md overflow-hidden bg-white border border-[#E2E8F0] shadow-xs flex items-center justify-center p-4">
            {activeImage ? (
              <img
                src={activeImage}
                alt={product.name}
                className="w-full h-full object-contain"
              />
            ) : (
              <div className="flex flex-col items-center justify-center text-[#94A3B8] p-8">
                <Package className="w-12 h-12 stroke-[1.5] text-[#CBD5E1] mb-2" />
                <span className="text-xs font-semibold">
                  No Image Available
                </span>
              </div>
            )}

            {hasDiscount && (
              <span className="absolute top-3 left-3 px-2 py-0.5 rounded-md bg-[#F26522] text-white text-xs font-bold shadow-xs">
                -{discountPercent}%
              </span>
            )}
          </div>

          {/* Thumbnail Strip */}
          {allImages.length > 1 && (
            <div className="grid grid-cols-4 sm:grid-cols-6 gap-2.5">
              {allImages.map((imgUrl, idx) => (
                <button
                  key={idx}
                  onClick={() => setActiveImageIndex(idx)}
                  className={`relative aspect-[4/3] rounded-md overflow-hidden bg-white p-1 border transition-colors ${
                    activeImageIndex === idx
                      ? "border-2 border-[#007D9C] ring-1 ring-[#00ADD8]/40"
                      : "border-[#E2E8F0] hover:border-[#CBD5E1]"
                  }`}
                  aria-label={`View image thumbnail ${idx + 1}`}
                >
                  <img
                    src={imgUrl}
                    alt={`Thumbnail ${idx + 1}`}
                    className="w-full h-full object-contain rounded-sm"
                  />
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Right: Product Details & Buying Actions (5 cols) */}
        <div className="lg:col-span-5 bg-white border border-[#E2E8F0] rounded-md p-5 shadow-xs space-y-4">
          {/* Header Tag & SKU */}
          <div>
            <div className="flex items-center justify-between gap-2 mb-1.5">
              {product.category_name ? (
                <span className="inline-flex items-center gap-1 text-[11px] font-bold text-[#64748B] bg-slate-150 px-2 py-0.5 rounded-md">
                  {product.category_name}
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 text-[11px] font-bold text-[#64748B] bg-slate-150 px-2 py-0.5 rounded-md">
                  Official Hardware
                </span>
              )}
              {(activeVariant?.sku || product.sku) && (
                <span className="text-xs font-mono text-[#64748B]">
                  SKU: {activeVariant?.sku || product.sku}
                </span>
              )}
            </div>

            {/* Title */}
            <h1 className="font-heading text-xl sm:text-2xl font-bold text-[#0F172A] leading-tight">
              {product.name}
            </h1>

            {/* Rating & Stock Subtitle */}
            <div className="flex items-center flex-wrap gap-2 text-xs mt-2 text-[#64748B]">
              <button
                type="button"
                onClick={scrollToReviews}
                className="flex items-center gap-1.5 text-left hover:text-[#007D9C] transition-colors cursor-pointer"
              >
                <div className="flex text-amber-500">
                  {[...Array(5)].map((_, i) => (
                    <Star
                      key={i}
                      className={`w-3.5 h-3.5 ${
                        i < Math.round(initialSummary?.avg_rating || 5)
                          ? "fill-amber-400 text-amber-400"
                          : "text-slate-200"
                      }`}
                    />
                  ))}
                </div>
                <span className="font-bold text-[#0F172A]">
                  {initialSummary && initialSummary.avg_rating > 0
                    ? initialSummary.avg_rating.toFixed(1)
                    : "4.9"}
                </span>
                <span className="text-[#64748B]">
                  ({initialSummary?.total_reviews || 0} reviews)
                </span>
              </button>
            </div>
          </div>

          <div className="h-px bg-[#E2E8F0]" />

          {/* Price Block */}
          <div className="flex items-baseline gap-3">
            <span className="font-heading text-2xl sm:text-3xl font-bold text-[#0F172A]">
              {formatCurrency(currentPrice)}
            </span>

            {hasDiscount && (
              <>
                <span className="text-sm text-[#64748B] line-through">
                  {formatCurrency(currentCompareAtPrice)}
                </span>
                <span className="font-heading text-xs font-bold text-white bg-[#F26522] px-2 py-0.5 rounded-md">
                  -{discountPercent}% OFF
                </span>
                <span className="ml-auto text-xs text-[#007D9C] font-semibold bg-[#F0F9FB] px-2 py-0.5 rounded-md border border-[#E0F7FC]">
                  Save {formatCurrency(savedAmount)}
                </span>
              </>
            )}
          </div>

          {/* Variant Selector */}
          {product.variants && product.variants.length > 0 && (
            <div className="space-y-2 pt-1">
              <div className="flex items-center justify-between">
                <label className="font-heading text-xs font-bold text-[#0F172A] uppercase tracking-wider">
                  Options & Variants
                </label>
                <span className="text-[11px] text-[#64748B]">
                  {product.variants.length} available
                </span>
              </div>
              <div className="grid grid-cols-1 gap-1.5">
                {product.variants.map((variant) => {
                  const isSelected = variant.uuid === selectedVariantUuid;
                  const isVariantAvailable = variant.stock_quantity > 0;
                  return (
                    <button
                      key={variant.uuid}
                      type="button"
                      disabled={!isVariantAvailable}
                      onClick={() => setSelectedVariantUuid(variant.uuid)}
                      className={`flex items-center justify-between px-3 py-2 rounded-md text-xs font-medium border text-left transition-colors ${
                        isSelected
                          ? "border-2 border-[#007D9C] bg-[#F0F9FB] text-[#007D9C] font-semibold"
                          : isVariantAvailable
                            ? "border-[#E2E8F0] hover:border-[#CBD5E1] bg-white text-[#0F172A]"
                            : "border-[#E2E8F0] bg-[#F8FAFC] text-[#94A3B8] line-through cursor-not-allowed"
                      }`}
                    >
                      <span className="truncate">
                        {variant.size ||
                          variant.color ||
                          variant.sku ||
                          "Default"}
                      </span>
                      <span className="text-[11px] font-semibold shrink-0 ml-2">
                        {formatCurrency(variant.price)}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Quantity Stepper & Stock status (Upgraded Mobile Touch Target) */}
          <div className="flex items-center gap-4 pt-1">
            <div className="flex items-center border border-[#E2E8F0] rounded-md h-10 sm:h-9 bg-white shadow-xs">
              <button
                type="button"
                onClick={() => setQuantity((q) => Math.max(1, q - 1))}
                className="w-10 sm:w-8 h-full flex items-center justify-center text-[#64748B] hover:text-[#0F172A] hover:bg-[#F8FAFC] rounded-l-md transition-colors"
                aria-label="Decrease quantity"
              >
                <Minus className="w-3.5 h-3.5" />
              </button>
              <span className="w-10 sm:w-9 h-full flex items-center justify-center text-xs font-bold text-[#0F172A]">
                {quantity}
              </span>
              <button
                type="button"
                onClick={() =>
                  setQuantity((q) => Math.min(currentStock || 99, q + 1))
                }
                className="w-10 sm:w-8 h-full flex items-center justify-center text-[#64748B] hover:text-[#0F172A] hover:bg-[#F8FAFC] rounded-r-md transition-colors"
                aria-label="Increase quantity"
              >
                <Plus className="w-3.5 h-3.5" />
              </button>
            </div>
            <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-[#64748B]">
              {isInStock ? `In Stock (${currentStock} units)` : "Out of Stock"}
            </span>
          </div>

          {/* Action CTA Buttons */}
          <div className="grid grid-cols-2 gap-3 pt-1">
            <button
              type="button"
              onClick={handleAddToCart}
              disabled={!isInStock}
              className={`flex items-center justify-center gap-2 font-heading font-semibold text-xs sm:text-sm h-11 px-4 rounded-md transition-colors shadow-xs ${
                isInStock
                  ? "bg-[#007D9C] hover:bg-[#00657E] text-white cursor-pointer"
                  : "bg-[#E2E8F0] text-[#94A3B8] cursor-not-allowed"
              }`}
            >
              <ShoppingCart className="w-4 h-4" />
              <span>Add to Cart</span>
            </button>

            <button
              type="button"
              onClick={handleBuyNow}
              disabled={!isInStock}
              className={`flex items-center justify-center gap-2 font-heading font-semibold text-xs sm:text-sm h-11 px-4 rounded-md transition-colors shadow-xs ${
                isInStock
                  ? "bg-[#F26522] hover:bg-[#EA580C] text-white cursor-pointer"
                  : "bg-[#E2E8F0] text-[#94A3B8] cursor-not-allowed"
              }`}
            >
              <Zap className="w-4 h-4" />
              <span>Buy Now</span>
            </button>
          </div>

          {/* Trust & Delivery Bar */}
          <div className="grid grid-cols-3 gap-2 pt-2 border-t border-[#E2E8F0]">
            <div className="flex items-center gap-1.5 p-2 bg-[#F8FAFC] border border-[#E2E8F0] rounded-md text-[11px] text-[#0F172A]">
              <Truck className="w-4 h-4 text-[#007D9C] shrink-0" />
              <span className="font-medium leading-tight">
                Free 2-Day Shipping
              </span>
            </div>
            <div className="flex items-center gap-1.5 p-2 bg-[#F8FAFC] border border-[#E2E8F0] rounded-md text-[11px] text-[#0F172A]">
              <RotateCcw className="w-4 h-4 text-[#007D9C] shrink-0" />
              <span className="font-medium leading-tight">30-Day Returns</span>
            </div>
            <div className="flex items-center gap-1.5 p-2 bg-[#F8FAFC] border border-[#E2E8F0] rounded-md text-[11px] text-[#0F172A]">
              <ShieldCheck className="w-4 h-4 text-[#007D9C] shrink-0" />
              <span className="font-medium leading-tight">2-Yr Warranty</span>
            </div>
          </div>
        </div>
      </div>

      {/* Lower Section: Tabs (Description, Specifications) */}
      <div className="bg-white border border-[#E2E8F0] rounded-md overflow-hidden shadow-xs">
        {/* Tabs Navigation Header */}
        <div className="flex border-b border-[#E2E8F0] bg-[#F8FAFC] px-4 overflow-x-auto">
          {[
            { id: "desc", label: "Product Overview" },
            { id: "specs", label: "Technical Specifications" },
          ].map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id as "desc" | "specs")}
              className={`py-3 px-4 text-xs font-semibold whitespace-nowrap border-b-2 transition-colors ${
                activeTab === tab.id
                  ? "text-[#007D9C] border-[#007D9C] bg-white"
                  : "text-[#64748B] hover:text-[#0F172A] border-transparent"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab Content Body */}
        <div className="p-5 sm:p-6">
          {activeTab === "desc" && (
            <div className="space-y-5 max-w-3xl">
              <div>
                <h3 className="text-sm font-bold text-[#0F172A] mb-2">
                  About {product.name}
                </h3>
                <div className="text-xs text-[#64748B] leading-relaxed whitespace-pre-line">
                  {product.description ||
                    "Experience top-tier quality and engineering with this authentic product from GOshop. Built for performance, durability, and a clean workspace setup."}
                </div>
              </div>

              <div className="space-y-2.5 pt-4 border-t border-[#E2E8F0]">
                <h4 className="text-xs font-bold text-[#0F172A] uppercase tracking-wider">
                  Highlights & Build Quality:
                </h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs text-[#64748B]">
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-3.5 h-3.5 text-[#007D9C] shrink-0" />
                    <span>Premium materials with high structural rigidity</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-3.5 h-3.5 text-[#007D9C] shrink-0" />
                    <span>Engineered for long-lasting performance</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-3.5 h-3.5 text-[#007D9C] shrink-0" />
                    <span>
                      Backed by official GOshop 100% genuine guarantee
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-3.5 h-3.5 text-[#007D9C] shrink-0" />
                    <span>Secure, tamper-evident protective packaging</span>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeTab === "specs" && (
            <div className="max-w-2xl">
              <div className="rounded-md border border-[#E2E8F0] overflow-hidden divide-y divide-[#E2E8F0] text-xs">
                <div className="p-2.5 bg-[#F8FAFC] flex justify-between">
                  <span className="font-semibold text-[#64748B]">
                    Product Name
                  </span>
                  <span className="font-bold text-[#0F172A]">
                    {product.name}
                  </span>
                </div>
                <div className="p-2.5 bg-white flex justify-between">
                  <span className="font-semibold text-[#64748B]">
                    SKU Number
                  </span>
                  <span className="font-mono text-[#0F172A]">
                    {product.sku || "GOPHER-KB-PRO96"}
                  </span>
                </div>
                <div className="p-2.5 bg-[#F8FAFC] flex justify-between">
                  <span className="font-semibold text-[#64748B]">Category</span>
                  <span className="text-[#007D9C] font-semibold">
                    {product.category_name || "Keyboards & Mice"}
                  </span>
                </div>
                <div className="p-2.5 bg-white flex justify-between">
                  <span className="font-semibold text-[#64748B]">
                    Availability
                  </span>
                  <span className="text-emerald-700 font-semibold">
                    {isInStock
                      ? `In Stock (${currentStock} units)`
                      : "Out of Stock"}
                  </span>
                </div>
                <div className="p-2.5 bg-[#F8FAFC] flex justify-between">
                  <span className="font-semibold text-[#64748B]">
                    Warranty Coverage
                  </span>
                  <span className="text-[#0F172A] font-medium">
                    24 Months Official Warranty
                  </span>
                </div>
                <div className="p-2.5 bg-white flex justify-between">
                  <span className="font-semibold text-[#64748B]">
                    Authenticity
                  </span>
                  <span className="text-emerald-700 font-semibold">
                    100% Genuine Certified
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Standalone Customer Reviews Section */}
      <ProductReviewsSection
        productUuid={product.uuid}
        productName={product.name}
        productImageUrl={activeImage}
        initialSummary={initialSummary}
        initialReviews={initialReviews}
      />
    </div>
  );
}
