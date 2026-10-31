"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { formatCurrency } from "@/lib/format";
import { getImageUrl } from "@/lib/utils";
import { authFetch } from "@/lib/api";
import {
  ShoppingCart,
  Trash2,
  Plus,
  Minus,
  ArrowRight,
  ArrowLeft,
  ShieldCheck,
  Tag,
  Lock,
  RefreshCw,
  AlertTriangle,
  AlertCircle,
  CheckCircle2,
  Sparkles,
} from "lucide-react";
import { toast } from "sonner";
import { useMounted } from "@/hooks/use-mounted";

export default function CartPage() {
  const {
    items,
    isSyncing,
    fetchServerCart,
    updateQuantity,
    removeItem,
    clearCart,
    toggleSelectItem,
    selectAllItems,
    removeSelectedItems,
    removeUnavailableItems,
    appliedCoupon,
    setAppliedCoupon,
  } = useCartStore();
  const { user, accessToken } = useAuthStore();

  const mounted = useMounted();
  const [couponCode, setCouponCode] = useState("");
  const [isApplyingCoupon, setIsApplyingCoupon] = useState(false);
  const [couponError, setCouponError] = useState<string | null>(null);
  const [showClearConfirm, setShowClearConfirm] = useState(false);

  if (!mounted) {
    return (
      <div className="min-h-[60vh] flex items-center justify-center">
        <div className="w-8 h-8 rounded-full border-2 border-[#007D9C] border-t-transparent animate-spin" />
      </div>
    );
  }

  if (!user || !accessToken) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-16 h-16 rounded-md bg-[#E6F7FB] text-[#007D9C] flex items-center justify-center mx-auto shadow-xs">
            <Lock className="w-8 h-8" />
          </div>
          <h1 className="text-2xl font-bold text-slate-900 tracking-tight">
            Sign In Required
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            Please sign in to your GOshop account to view and manage your
            shopping cart.
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              href="/login?redirect=/cart"
              className="w-full sm:w-auto px-5 py-2.5 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-sm font-semibold transition-all shadow-xs text-center cursor-pointer"
            >
              Sign In Now
            </Link>
            <Link
              href="/products"
              className="w-full sm:w-auto px-5 py-2.5 rounded-md border border-slate-200 bg-white text-slate-700 text-sm font-semibold hover:bg-slate-50 transition-all text-center shadow-xs cursor-pointer"
            >
              Continue Shopping
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const availableItems = items.filter(
    (item) => item.isAvailable !== false && item.stockQuantity > 0,
  );
  const unavailableItems = items.filter(
    (item) => item.isAvailable === false || item.stockQuantity <= 0,
  );

  const selectedItems = availableItems.filter(
    (item) => item.selected !== false,
  );
  const selectedCount = selectedItems.length;
  const allSelected =
    availableItems.length > 0 && selectedCount === availableItems.length;

  const subtotal = selectedItems.reduce(
    (total, item) => total + item.price * item.quantity,
    0,
  );
  const discountAmount = appliedCoupon ? appliedCoupon.discountAmount : 0;
  const grandTotal = Math.max(0, subtotal - discountAmount);

  const handleApplyCoupon = async (e: React.FormEvent) => {
    e.preventDefault();
    setCouponError(null);
    if (!couponCode.trim()) {
      setCouponError("Please enter a promo code.");
      return;
    }

    const code = couponCode.trim().toUpperCase();

    if (!accessToken) {
      setCouponError("Please sign in to apply coupon codes.");
      return;
    }

    if (subtotal <= 0) {
      setCouponError(
        "Please select valid in-stock items before applying a coupon.",
      );
      return;
    }

    setIsApplyingCoupon(true);
    try {
      const res = await authFetch("/api/v1/coupons/preview", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          code,
          subtotal_amount: subtotal,
        }),
      });

      const data = await res.json().catch(() => null);

      if (res.ok && data?.data) {
        setAppliedCoupon({
          code: data.data.coupon_code,
          name: data.data.coupon_name,
          discountAmount: data.data.discount_amount,
        });
        setCouponError(null);
        setCouponCode("");
      } else {
        const errMsg =
          data?.detail ||
          data?.error ||
          data?.message ||
          "Invalid or expired coupon code.";
        setCouponError(errMsg);
      }
    } catch {
      setCouponError("Network error while verifying coupon.");
    } finally {
      setIsApplyingCoupon(false);
    }
  };

  const handleRemoveCoupon = () => {
    setAppliedCoupon(null);
    setCouponCode("");
    setCouponError(null);
  };

  const handleClearCart = async () => {
    await clearCart();
    setShowClearConfirm(false);
    toast.info("Your cart has been cleared.");
  };

  if (items.length === 0) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-16 h-16 rounded-2xl bg-[#E6F7FB] text-[#007D9C] flex items-center justify-center mx-auto shadow-xs">
            <ShoppingCart className="w-8 h-8" />
          </div>
          <h1 className="text-2xl font-bold text-slate-900 tracking-tight">
            Your Cart is Empty
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            You don&apos;t have any products in your cart yet. Explore our
            latest arrivals and best selling items now!
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              href="/products"
              className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-5 py-2.5 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-sm font-semibold transition-all shadow-xs cursor-pointer"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>Continue Shopping</span>
            </Link>
            <button
              type="button"
              onClick={() => fetchServerCart()}
              disabled={isSyncing}
              className="w-full sm:w-auto inline-flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-md border border-slate-200 bg-white text-slate-700 text-sm font-semibold hover:bg-slate-50 hover:text-slate-900 transition-all shadow-xs cursor-pointer disabled:opacity-60"
            >
              <RefreshCw
                className={`w-4 h-4 ${isSyncing ? "animate-spin text-[#007D9C]" : "text-slate-400"}`}
              />
              <span>Sync Cart</span>
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-10">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-6 border-b border-slate-200/80 mb-6">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl sm:text-3xl font-bold text-slate-900 tracking-tight">
                Shopping Cart
              </h1>
              <span className="bg-[#E6F7FB] text-[#007D9C] text-xs font-bold px-2.5 py-1 rounded-full">
                {items.length} {items.length === 1 ? "item" : "items"}
              </span>
            </div>
            <p className="text-xs text-slate-500 mt-1">
              Review your items, apply vouchers, and proceed to checkout
              securely.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => fetchServerCart()}
              disabled={isSyncing}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl border border-slate-200 bg-white text-xs font-semibold text-slate-700 hover:bg-slate-50 transition-colors shadow-xs cursor-pointer disabled:opacity-60"
              title="Sync cart items with latest server stock"
            >
              <RefreshCw
                className={`w-3.5 h-3.5 ${isSyncing ? "animate-spin text-[#007D9C]" : "text-slate-400"}`}
              />
              <span>{isSyncing ? "Syncing..." : "Sync Stock"}</span>
            </button>

            {showClearConfirm ? (
              <div className="flex items-center gap-1.5 bg-rose-50 border border-rose-200 p-1 rounded-xl">
                <span className="text-[11px] font-semibold text-rose-700 px-2">
                  Clear all?
                </span>
                <button
                  onClick={handleClearCart}
                  className="px-2.5 py-1 rounded-lg bg-rose-600 text-white text-xs font-bold hover:bg-rose-700 transition-colors cursor-pointer"
                >
                  Yes
                </button>
                <button
                  onClick={() => setShowClearConfirm(false)}
                  className="px-2.5 py-1 rounded-lg bg-white text-slate-600 text-xs font-semibold hover:bg-slate-100 transition-colors cursor-pointer"
                >
                  Cancel
                </button>
              </div>
            ) : (
              <button
                onClick={() => setShowClearConfirm(true)}
                className="text-xs font-semibold text-rose-500 hover:text-rose-700 transition-colors cursor-pointer px-2 py-1"
              >
                Clear Cart
              </button>
            )}
          </div>
        </div>

        {/* Unavailable items alert banner */}
        {unavailableItems.length > 0 && (
          <div className="mb-6 p-4 rounded-2xl bg-amber-50/90 border border-amber-200 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 shadow-xs">
            <div className="flex items-center gap-3">
              <AlertTriangle className="w-5 h-5 text-amber-600 shrink-0" />
              <div>
                <p className="text-xs font-bold text-amber-900">
                  {unavailableItems.length}{" "}
                  {unavailableItems.length === 1 ? "item is" : "items are"}{" "}
                  currently unavailable or out of stock
                </p>
                <p className="text-[11px] text-amber-700">
                  Unavailable items are excluded from checkout. You can remove
                  them or keep them for later.
                </p>
              </div>
            </div>
            <button
              onClick={() => removeUnavailableItems()}
              className="px-3.5 py-1.5 rounded-xl bg-amber-600 text-white text-xs font-bold hover:bg-amber-700 transition-colors shadow-xs shrink-0 cursor-pointer"
            >
              Remove Unavailable ({unavailableItems.length})
            </button>
          </div>
        )}

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          {/* Items List (8 cols) */}
          <div className="lg:col-span-8 space-y-4">
            <div className="rounded-2xl bg-white border border-slate-200/80 shadow-xs overflow-hidden divide-y divide-slate-100">
              {/* Select All Action Bar */}
              <div className="px-5 py-3.5 bg-slate-50/90 flex items-center justify-between">
                <label className="flex items-center gap-3 cursor-pointer select-none text-xs font-bold text-slate-900">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    onChange={(e) => selectAllItems(e.target.checked)}
                    disabled={availableItems.length === 0}
                    className="w-4 h-4 rounded text-[#007D9C] focus:ring-[#007D9C] border-slate-300 accent-[#007D9C] cursor-pointer disabled:opacity-40"
                  />
                  <span>
                    Select All ({selectedCount}/{availableItems.length}{" "}
                    available items)
                  </span>
                </label>

                {selectedCount > 0 && (
                  <button
                    onClick={() => {
                      if (
                        window.confirm(
                          `Remove ${selectedCount} selected items from your cart?`,
                        )
                      ) {
                        removeSelectedItems();
                        toast.info("Selected items removed.");
                      }
                    }}
                    className="text-xs font-semibold text-rose-500 hover:text-rose-700 transition-colors flex items-center gap-1 cursor-pointer"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                    <span>Delete Selected ({selectedCount})</span>
                  </button>
                )}
              </div>

              {items.map((item) => {
                const isOutOfStock =
                  item.isAvailable === false || item.stockQuantity <= 0;
                const isLowStock =
                  !isOutOfStock &&
                  item.stockQuantity > 0 &&
                  item.stockQuantity <= 5;
                const isSelected = item.selected !== false && !isOutOfStock;

                return (
                  <div
                    key={item.id}
                    className={`p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 transition-colors ${
                      isOutOfStock
                        ? "bg-slate-50/70 opacity-60"
                        : !isSelected
                          ? "bg-slate-50/40 opacity-75"
                          : "hover:bg-slate-50/50"
                    }`}
                  >
                    {/* Checkbox, Thumbnail & Info */}
                    <div className="flex items-center gap-3.5 min-w-0 flex-1">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        disabled={isOutOfStock}
                        onChange={() => toggleSelectItem(item.id)}
                        aria-label={`Select ${item.name}`}
                        className="w-4 h-4 rounded text-[#007D9C] focus:ring-[#007D9C] border-slate-300 accent-[#007D9C] cursor-pointer shrink-0 disabled:opacity-30 disabled:cursor-not-allowed"
                      />

                      <div className="relative w-20 h-20 rounded-xl bg-slate-100 overflow-hidden shrink-0 border border-slate-200/60 flex items-center justify-center">
                        {item.image ? (
                          <img
                            src={getImageUrl(item.image)}
                            alt={item.name}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <ShoppingCart className="w-6 h-6 text-slate-400" />
                        )}
                        {isOutOfStock && (
                          <div className="absolute inset-0 bg-slate-900/60 flex items-center justify-center p-1 text-center">
                            <span className="text-[10px] font-black uppercase tracking-wider text-white">
                              Sold Out
                            </span>
                          </div>
                        )}
                      </div>

                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2 flex-wrap">
                          <Link
                            href={`/products/${item.productUuid}`}
                            className="text-sm font-bold text-[#0D1B2A] hover:text-[#2196F3] transition-colors truncate block"
                          >
                            {item.name}
                          </Link>
                          {isOutOfStock ? (
                            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold bg-rose-100 text-rose-700">
                              Out of stock
                            </span>
                          ) : isLowStock ? (
                            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold bg-amber-100 text-amber-800">
                              Only {item.stockQuantity} left
                            </span>
                          ) : null}
                        </div>

                        {(item.selectedSize || item.selectedColor) && (
                          <p className="text-xs text-slate-500 mt-0.5">
                            {item.selectedSize && `Size: ${item.selectedSize}`}
                            {item.selectedSize && item.selectedColor && " • "}
                            {item.selectedColor &&
                              `Color: ${item.selectedColor}`}
                          </p>
                        )}
                        <p className="text-xs text-[#0D47A1] font-bold mt-1">
                          {formatCurrency(item.price)}
                        </p>
                      </div>
                    </div>

                    {/* Quantity & Actions */}
                    <div className="flex items-center justify-between w-full sm:w-auto gap-4 self-end sm:self-center">
                      <div className="flex items-center border border-slate-200 rounded-xl bg-slate-50 overflow-hidden">
                        <button
                          onClick={() =>
                            updateQuantity(item.id, item.quantity - 1)
                          }
                          disabled={isOutOfStock || item.quantity <= 1}
                          className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                          aria-label="Decrease quantity"
                        >
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                        <span className="px-3 text-xs font-bold text-slate-700 min-w-[28px] text-center">
                          {item.quantity}
                        </span>
                        <button
                          onClick={() =>
                            updateQuantity(item.id, item.quantity + 1)
                          }
                          disabled={
                            isOutOfStock ||
                            (item.stockQuantity > 0 &&
                              item.quantity >= item.stockQuantity)
                          }
                          className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                          aria-label="Increase quantity"
                        >
                          <Plus className="w-3.5 h-3.5" />
                        </button>
                      </div>

                      <span className="text-sm font-black text-[#0D1B2A] min-w-[90px] text-right">
                        {formatCurrency(item.price * item.quantity)}
                      </span>

                      <button
                        onClick={() => removeItem(item.id)}
                        className="p-1.5 text-slate-400 hover:text-rose-500 rounded-lg transition-colors cursor-pointer"
                        title="Remove item"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>

            <div className="pt-2">
              <Link
                href="/products"
                className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-600 hover:text-[#2196F3] transition-colors"
              >
                <ArrowLeft className="w-4 h-4" />
                <span>Continue Shopping</span>
              </Link>
            </div>
          </div>

          {/* Order Summary Card (4 cols) */}
          <div className="lg:col-span-4 space-y-4">
            <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-5">
              <h2 className="text-base font-bold text-[#0D1B2A]">
                Order Summary
              </h2>

              {/* Coupon Form */}
              <div className="space-y-2">
                <form onSubmit={handleApplyCoupon} className="space-y-1.5">
                  <div className="flex gap-2">
                    <div className="relative flex-1">
                      <input
                        type="text"
                        value={couponCode}
                        onChange={(e) => {
                          setCouponCode(e.target.value);
                          if (couponError) setCouponError(null);
                        }}
                        placeholder="Promo Code (e.g. SOLOLEVELING)"
                        disabled={isApplyingCoupon}
                        className={`w-full pl-8 pr-3 py-2 rounded-xl bg-slate-50 border text-xs uppercase text-slate-900 focus:outline-none disabled:opacity-60 transition-colors ${
                          couponError
                            ? "border-rose-400 focus:border-rose-500"
                            : "border-slate-200 focus:border-[#007D9C]"
                        }`}
                      />
                      <Tag className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                    </div>
                    <button
                      type="submit"
                      disabled={isApplyingCoupon || !couponCode.trim()}
                      className="px-3.5 py-2 rounded-xl bg-[#007D9C] text-white text-xs font-semibold hover:bg-[#00657E] transition-colors cursor-pointer disabled:opacity-60 shadow-xs"
                    >
                      {isApplyingCoupon ? "Verifying..." : "Apply"}
                    </button>
                  </div>
                  {couponError && (
                    <div className="flex items-center gap-1.5 text-[11px] text-rose-600 font-medium px-0.5">
                      <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                      <span>{couponError}</span>
                    </div>
                  )}
                </form>

                {appliedCoupon && (
                  <div className="flex items-center justify-between text-xs px-3 py-2 rounded-lg bg-emerald-50 text-emerald-700 border border-emerald-200">
                    <div className="flex flex-col">
                      <span className="font-bold">
                        Code: {appliedCoupon.code}
                      </span>
                      {appliedCoupon.name && (
                        <span className="text-[11px] text-emerald-600 font-normal">
                          {appliedCoupon.name}
                        </span>
                      )}
                    </div>
                    <button
                      type="button"
                      onClick={handleRemoveCoupon}
                      className="text-xs font-bold text-rose-600 hover:text-rose-800 hover:underline cursor-pointer"
                    >
                      Remove
                    </button>
                  </div>
                )}
              </div>

              {/* Breakdown */}
              <div className="space-y-2.5 text-xs text-slate-600 pt-2 border-t border-slate-100">
                <div className="flex justify-between">
                  <span>
                    Subtotal ({selectedCount}{" "}
                    {selectedCount === 1 ? "item" : "items"})
                  </span>
                  <span className="font-semibold text-slate-900">
                    {formatCurrency(subtotal)}
                  </span>
                </div>

                {discountAmount > 0 && (
                  <div className="flex justify-between text-emerald-600 font-semibold">
                    <span>Discount ({appliedCoupon?.code})</span>
                    <span>-{formatCurrency(discountAmount)}</span>
                  </div>
                )}

                <div className="flex justify-between items-baseline">
                  <div>
                    <span>Shipping</span>
                    <p className="text-[10px] text-slate-400">
                      GHN Express live rate
                    </p>
                  </div>
                  <span className="font-semibold text-slate-500 text-xs">
                    {selectedCount === 0 ? "—" : "Calculated at checkout"}
                  </span>
                </div>

                <div className="flex justify-between text-base font-bold text-slate-900 pt-3 border-t border-slate-200">
                  <span>Total</span>
                  <span className="text-[#007D9C] font-extrabold">
                    {formatCurrency(grandTotal)}
                  </span>
                </div>
              </div>

              {/* Checkout Button */}
              {selectedCount > 0 ? (
                <Link
                  href="/checkout"
                  className="w-full py-3 px-6 rounded-xl bg-[#007D9C] text-white text-sm font-bold flex items-center justify-center gap-2 hover:bg-[#00657E] transition-all shadow-xs active:scale-[0.99] text-center cursor-pointer"
                >
                  <span>Proceed to Checkout ({selectedCount})</span>
                  <ArrowRight className="w-4 h-4" />
                </Link>
              ) : (
                <button
                  type="button"
                  disabled
                  className="w-full py-3 px-6 rounded-xl bg-slate-200 text-slate-400 text-sm font-bold flex items-center justify-center gap-2 cursor-not-allowed"
                >
                  <span>Select items to checkout</span>
                </button>
              )}

              {/* Trust Badge */}
              <div className="flex items-center gap-2 text-xs text-slate-500 pt-2">
                <ShieldCheck className="w-4 h-4 text-emerald-500 shrink-0" />
                <span>SSL Encrypted Checkout with MoMo & COD</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
