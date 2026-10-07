"use client";

import React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Image from "next/image";
import type { Product } from "@/lib/api-server";
import { formatCurrency } from "@/lib/format";
import { getImageUrl } from "@/lib/utils";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { ShoppingCart, Package, Star, Eye } from "lucide-react";
import { toast } from "sonner";

interface ProductCardProps {
  product: Product;
}

export function ProductCard({ product }: ProductCardProps) {
  const router = useRouter();
  const { addItem } = useCartStore();
  const { accessToken } = useAuthStore();

  // Primary product image
  const rawImg =
    product.images?.find((img) => img.is_primary)?.image_url ||
    product.images?.[0]?.image_url ||
    product.image_url ||
    "";
  const primaryImg = getImageUrl(rawImg);

  const isInStock = (product.stock ?? 0) > 0;
  const isLowStock = isInStock && (product.stock ?? 0) <= 5;
  const hasDiscount = Boolean(
    product.compare_at_price && product.compare_at_price > product.price,
  );
  const discountPercent = hasDiscount
    ? Math.round(
        ((product.compare_at_price! - product.price) /
          product.compare_at_price!) *
          100,
      )
    : 0;

  const ratingScore = product.rating ?? 0;
  const reviewsCount = product.reviews_count ?? 0;
  const hasReviews = reviewsCount > 0 && ratingScore > 0;

  const productUrl = `/products/${encodeURIComponent(product.slug || product.uuid)}`;

  const handleAddToCart = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    if (!accessToken) {
      toast.error("Please sign in to add items to your cart.");
      router.push(`/login?redirect=${encodeURIComponent(productUrl)}`);
      return;
    }

    if (!isInStock) {
      toast.error("This product is currently out of stock");
      return;
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
    });

    toast.success(`Added "${product.name}" to cart!`);
  };

  return (
    <article className="group relative rounded-md bg-white border border-slate-200/90 p-4 flex flex-col justify-between shadow-xs hover:border-[#007D9C] hover:shadow-md">
      <div>
        {/* Product Image Thumbnail */}
        <div className="relative aspect-square w-full rounded-md overflow-hidden bg-slate-50 mb-3 border border-slate-100 flex items-center justify-center">
          {primaryImg ? (
            <Image
              src={primaryImg}
              alt={product.name}
              fill
              sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 25vw"
              className="object-cover object-center"
            />
          ) : (
            <div className="flex flex-col items-center justify-center text-slate-400 p-4">
              <Package className="w-8 h-8 text-slate-300 mb-1" />
              <span className="text-[11px] font-medium text-slate-400">
                No Image
              </span>
            </div>
          )}

          {/* Discount Badge */}
          {hasDiscount && (
            <span className="absolute top-2.5 left-2.5 px-2 py-0.5 rounded-xs bg-[#F26522] text-white text-[10px] font-black font-heading shadow-xs z-10">
              -{discountPercent}%
            </span>
          )}

          {/* Direct Link Overlay */}
          <Link
            href={productUrl}
            className="absolute inset-0 z-0"
            aria-label={`View details of ${product.name}`}
          />
        </div>

        {/* Category & Star Rating */}
        <div className="flex items-center justify-between gap-2 mb-1 min-h-[20px]">
          {product.category_name ? (
            <span className="text-[11px] font-bold text-[#007D9C] uppercase tracking-wider truncate font-heading">
              {product.category_name}
            </span>
          ) : (
            <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider font-heading">
              Official Merch
            </span>
          )}

          {hasReviews && (
            <div
              className="inline-flex items-center gap-1 shrink-0 px-1.5 py-0.5 rounded-xs bg-amber-50 border border-amber-200/80 text-[11px] font-bold text-amber-800"
              title={`Rating: ${ratingScore.toFixed(1)} / 5.0 (${reviewsCount} reviews)`}
            >
              <Star className="w-3 h-3 fill-amber-400 text-amber-400" />
              <span>{ratingScore.toFixed(1)}</span>
            </div>
          )}
        </div>

        {/* Product Title */}
        <Link href={productUrl} className="block group-hover:text-[#007D9C]">
          <h3 className="text-sm font-bold text-slate-900 line-clamp-2 leading-snug font-heading">
            {product.name}
          </h3>
        </Link>

        {/* Inventory Stock Indicator */}
        <div className="mt-2 flex items-center gap-1.5 text-[11px] font-medium font-body">
          {isInStock ? (
            isLowStock ? (
              <>
                <span className="w-1.5 h-1.5 rounded-full bg-amber-500 shrink-0" />
                <span className="text-amber-700 font-semibold">
                  Only {product.stock} units remaining
                </span>
              </>
            ) : (
              <>
                <span className="text-slate-500">
                  In stock ({product.stock} units)
                </span>
              </>
            )
          ) : (
            <>
              <span className="w-1.5 h-1.5 rounded-full bg-rose-500 shrink-0" />
              <span className="text-rose-600 font-semibold">Out of stock</span>
            </>
          )}
        </div>
      </div>

      {/* Pricing & Add to Cart Action */}
      <div className="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between gap-2">
        <div className="flex flex-col min-w-0">
          <span className="text-base font-black text-slate-900 truncate font-heading">
            {formatCurrency(product.price)}
          </span>
          {hasDiscount && (
            <span className="text-xs text-slate-400 line-through truncate font-body">
              {formatCurrency(product.compare_at_price)}
            </span>
          )}
        </div>

        {/* Add to Cart CTA */}
        <button
          type="button"
          onClick={handleAddToCart}
          disabled={!isInStock}
          className={`shrink-0 px-3.5 py-2 rounded-xs text-xs font-bold flex items-center gap-1.5 font-heading z-10 ${
            isInStock
              ? "bg-[#007D9C] text-white hover:bg-[#00637C] cursor-pointer shadow-xs"
              : "bg-slate-100 text-slate-400 cursor-not-allowed border border-slate-200"
          }`}
          aria-label={
            isInStock ? `Add ${product.name} to cart` : "Out of stock"
          }
          title={isInStock ? "Add to cart" : "Out of stock"}
        >
          <ShoppingCart className="w-3.5 h-3.5 shrink-0" />
          <span className="hidden sm:inline">
            {isInStock ? "Add" : "Sold Out"}
          </span>
        </button>
      </div>
    </article>
  );
}
