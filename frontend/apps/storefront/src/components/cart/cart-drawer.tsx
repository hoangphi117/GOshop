"use client";

import React from "react";
import Link from "next/link";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { formatCurrency } from "@/lib/format";
import { getImageUrl } from "@/lib/utils";
import {
  X,
  Plus,
  Minus,
  Trash2,
  ShoppingCart,
  ArrowRight,
  Lock,
  RefreshCw,
  Truck,
  Sparkles,
} from "lucide-react";
import { useMounted } from "@/hooks/use-mounted";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

const FREE_SHIPPING_THRESHOLD = 500000; // 500,000 VND

export function CartDrawer() {
  const {
    items,
    isDrawerOpen,
    isSyncing,
    closeDrawer,
    fetchServerCart,
    updateQuantity,
    removeItem,
    toggleSelectItem,
    selectAllItems,
  } = useCartStore();
  const { user, accessToken } = useAuthStore();

  const mounted = useMounted();

  if (!mounted) return null;

  const isAuthenticated = Boolean(user && accessToken);
  const availableItems = isAuthenticated
    ? items.filter(
        (item) => item.isAvailable !== false && item.stockQuantity > 0,
      )
    : [];
  const selectedItems = availableItems.filter(
    (item) => item.selected !== false,
  );
  const selectedCount = selectedItems.length;
  const allSelected =
    availableItems.length > 0 && selectedCount === availableItems.length;
  const subtotal = selectedItems.reduce(
    (acc, item) => acc + item.price * item.quantity,
    0,
  );

  const freeShippingProgress = Math.min(
    100,
    Math.round((subtotal / FREE_SHIPPING_THRESHOLD) * 100),
  );
  const remainingForFreeShipping = Math.max(
    0,
    FREE_SHIPPING_THRESHOLD - subtotal,
  );

  return (
    <>
      {/* Backdrop */}
      {isDrawerOpen && (
        <div
          onClick={closeDrawer}
          className="fixed inset-0 bg-slate-950/60 backdrop-blur-xs z-50 transition-opacity duration-300 animate-in fade-in-0"
          aria-hidden="true"
        />
      )}

      {/* Slide-over Drawer Panel */}
      <aside
        aria-label="Shopping Cart Drawer"
        className={`fixed top-0 right-0 bottom-0 w-full sm:w-[450px] bg-white z-50 shadow-2xl flex flex-col border-l border-slate-200 transition-transform duration-300 ease-in-out ${
          isDrawerOpen ? "translate-x-0" : "translate-x-full"
        }`}
      >
        {/* Drawer Header */}
        <div className="p-5 border-b border-slate-100 flex items-center justify-between bg-slate-50/70">
          <div className="flex items-center gap-2.5">
            <div className="w-9 h-9 rounded-xl bg-[#E6F7FB] text-[#007D9C] flex items-center justify-center">
              <ShoppingCart className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-bold text-slate-900 font-heading">
                Shopping Cart
              </h2>
              <p className="text-[11px] text-slate-500 font-body">
                {isAuthenticated
                  ? `${items.length} ${items.length === 1 ? "item" : "items"}`
                  : "Guest Session"}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-1.5">
            {isAuthenticated && (
              <button
                type="button"
                onClick={() => fetchServerCart()}
                disabled={isSyncing}
                className="p-2 rounded-xl text-slate-400 hover:text-slate-700 hover:bg-white border border-transparent hover:border-slate-200 transition-all cursor-pointer"
                title="Sync cart items"
                aria-label="Sync cart items"
              >
                <RefreshCw
                  className={`w-4 h-4 ${isSyncing ? "animate-spin text-[#00ADD8]" : ""}`}
                />
              </button>
            )}
            <button
              type="button"
              onClick={closeDrawer}
              className="p-2 rounded-xl text-slate-400 hover:text-slate-700 hover:bg-white border border-transparent hover:border-slate-200 transition-all cursor-pointer"
              aria-label="Close cart drawer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Free Shipping Progress Indicator */}
        {isAuthenticated && items.length > 0 && (
          <div className="px-5 py-3 bg-[#E6F7FB]/60 border-b border-[#00ADD8]/20">
            <div className="flex items-center justify-between text-xs font-semibold text-[#007D9C] mb-1.5 font-body">
              <span className="flex items-center gap-1.5">
                <Truck className="w-3.5 h-3.5 text-[#00ADD8]" />
                {freeShippingProgress >= 100
                  ? "You unlocked FREE Express Shipping!"
                  : `Add ${formatCurrency(remainingForFreeShipping)} for FREE Express Shipping`}
              </span>
              <span className="text-[11px] font-bold">
                {freeShippingProgress}%
              </span>
            </div>
            <div className="h-1.5 w-full bg-slate-200/80 rounded-md overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-[#00ADD8] to-[#007D9C] rounded-md transition-all duration-500"
                style={{ width: `${freeShippingProgress}%` }}
              />
            </div>
          </div>
        )}

        {/* Subheader: Select All */}
        {isAuthenticated && availableItems.length > 0 && (
          <div className="px-5 py-2.5 bg-slate-50 border-b border-slate-100 flex items-center justify-between text-xs">
            <label className="flex items-center gap-2 cursor-pointer select-none font-semibold text-slate-700">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={(e) => selectAllItems(e.target.checked)}
                className="w-4 h-4 rounded text-[#007D9C] accent-[#007D9C] cursor-pointer"
              />
              <span>
                Select All ({selectedCount}/{availableItems.length} ready)
              </span>
            </label>
          </div>
        )}

        {/* Drawer Scrollable Content */}
        <div className="flex-1 overflow-y-auto p-5 space-y-4">
          {!isAuthenticated ? (
            <div className="h-full flex flex-col items-center justify-center text-center p-6 space-y-3">
              <div className="w-16 h-16 rounded-md bg-[#E6F7FB] flex items-center justify-center text-[#007D9C] mb-2">
                <Lock className="w-8 h-8" />
              </div>
              <h3 className="text-base font-bold text-slate-900 font-heading">
                Sign In Required
              </h3>
              <p className="text-xs text-slate-500 max-w-xs font-body leading-relaxed">
                Please sign in to your GOshop account to sync your saved loot,
                track orders, and checkout.
              </p>
              <Button
                asChild
                variant="default"
                size="lg"
                onClick={closeDrawer}
                className="mt-2"
              >
                <Link href="/login?redirect=/cart">Sign In to Continue</Link>
              </Button>
            </div>
          ) : items.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center p-6 space-y-3">
              <div className="w-16 h-16 rounded-md bg-slate-100 flex items-center justify-center text-slate-400 mb-2">
                <ShoppingCart className="w-8 h-8" />
              </div>
              <h3 className="text-base font-bold text-slate-900 font-heading">
                Your loot box is empty
              </h3>
              <p className="text-xs text-slate-500 max-w-xs font-body leading-relaxed">
                Looks like you haven&apos;t added any boss plushies or game gear
                to your cart yet.
              </p>
              <Button
                variant="cyan"
                size="lg"
                onClick={closeDrawer}
                className="mt-2"
              >
                Explore Products Now
              </Button>
            </div>
          ) : (
            <ul className="divide-y divide-slate-100">
              {items.map((item) => {
                const isOutOfStock =
                  item.isAvailable === false || item.stockQuantity <= 0;
                const isLowStock =
                  !isOutOfStock &&
                  item.stockQuantity > 0 &&
                  item.stockQuantity <= 5;
                const isSelected = item.selected !== false && !isOutOfStock;

                return (
                  <li
                    key={item.id}
                    className={`py-4 flex items-center gap-3.5 first:pt-0 last:pb-0 transition-opacity ${
                      isOutOfStock
                        ? "opacity-50"
                        : !isSelected
                          ? "opacity-60"
                          : ""
                    }`}
                  >
                    {/* Checkbox */}
                    <input
                      type="checkbox"
                      checked={isSelected}
                      disabled={isOutOfStock}
                      onChange={() => toggleSelectItem(item.id)}
                      aria-label={`Select ${item.name}`}
                      className="w-4 h-4 rounded text-[#007D9C] accent-[#007D9C] cursor-pointer shrink-0 disabled:opacity-30 disabled:cursor-not-allowed"
                    />

                    {/* Thumbnail */}
                    <div className="relative w-18 h-18 rounded-xl bg-slate-100 overflow-hidden shrink-0 border border-slate-200/80 flex items-center justify-center">
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
                        <div className="absolute inset-0 bg-slate-950/70 flex items-center justify-center p-0.5 text-center">
                          <span className="text-[9px] font-black uppercase text-white">
                            Sold Out
                          </span>
                        </div>
                      )}
                    </div>

                    {/* Info */}
                    <div className="flex-1 flex flex-col justify-between min-w-0">
                      <div>
                        <div className="flex justify-between items-start gap-2">
                          <h4 className="text-sm font-bold text-slate-900 truncate font-heading">
                            {item.name}
                          </h4>
                          <button
                            type="button"
                            onClick={() => removeItem(item.id)}
                            className="text-slate-400 hover:text-red-500 p-1 transition-colors cursor-pointer"
                            title="Remove item"
                            aria-label={`Remove ${item.name}`}
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>

                        <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                          {(item.selectedSize || item.selectedColor) && (
                            <p className="text-xs text-slate-500 font-body">
                              {item.selectedSize &&
                                `Size: ${item.selectedSize}`}
                              {item.selectedSize && item.selectedColor && " | "}
                              {item.selectedColor &&
                                `Color: ${item.selectedColor}`}
                            </p>
                          )}
                          {isOutOfStock ? (
                            <Badge variant="destructive">Out of stock</Badge>
                          ) : isLowStock ? (
                            <Badge variant="cyan">
                              Only {item.stockQuantity} left
                            </Badge>
                          ) : null}
                        </div>
                      </div>

                      <div className="flex items-center justify-between mt-2.5">
                        <span className="text-sm font-extrabold text-[#007D9C] font-heading">
                          {formatCurrency(item.price)}
                        </span>

                        {/* Quantity Controller */}
                        <div className="flex items-center border border-slate-200 rounded-lg bg-slate-50 overflow-hidden shadow-2xs">
                          <button
                            type="button"
                            onClick={() =>
                              updateQuantity(item.id, item.quantity - 1)
                            }
                            disabled={isOutOfStock || item.quantity <= 1}
                            className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600 cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
                            aria-label="Decrease quantity"
                          >
                            <Minus className="w-3 h-3" />
                          </button>
                          <span className="px-2.5 text-xs font-bold text-slate-800 min-w-[24px] text-center font-body">
                            {item.quantity}
                          </span>
                          <button
                            type="button"
                            onClick={() =>
                              updateQuantity(item.id, item.quantity + 1)
                            }
                            disabled={
                              isOutOfStock ||
                              (item.stockQuantity > 0 &&
                                item.quantity >= item.stockQuantity)
                            }
                            className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600 cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
                            aria-label="Increase quantity"
                          >
                            <Plus className="w-3 h-3" />
                          </button>
                        </div>
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {/* Drawer Footer */}
        {isAuthenticated && items.length > 0 && (
          <div className="p-5 border-t border-slate-200 bg-slate-50/90 space-y-3">
            <div className="flex items-center justify-between text-sm">
              <span className="text-slate-600 font-body">
                Subtotal ({selectedCount}{" "}
                {selectedCount === 1 ? "item" : "items"})
              </span>
              <span className="text-base font-extrabold text-slate-900 font-heading">
                {formatCurrency(subtotal)}
              </span>
            </div>
            <p className="text-[11px] text-slate-500 font-body">
              Discounts, shipping fees, and taxes are finalized at checkout.
            </p>

            <div className="grid grid-cols-2 gap-2.5 pt-1">
              <Button
                asChild
                variant="outline"
                size="default"
                onClick={closeDrawer}
              >
                <Link href="/cart">View Cart</Link>
              </Button>
              {selectedCount > 0 ? (
                <Button
                  asChild
                  variant="accent"
                  size="default"
                  onClick={closeDrawer}
                >
                  <Link href="/checkout">
                    <span>Checkout ({selectedCount})</span>
                    <ArrowRight className="w-4 h-4 ml-1" />
                  </Link>
                </Button>
              ) : (
                <Button variant="accent" size="default" disabled>
                  Checkout
                </Button>
              )}
            </div>
          </div>
        )}
      </aside>
    </>
  );
}
