'use client'

import React from 'react'
import Link from 'next/link'
import { useCartStore } from '@/stores/cart-store'
import { formatCurrency } from '@/lib/format'
import { getImageUrl } from '@/lib/utils'
import { X, Plus, Minus, Trash2, ShoppingCart, ArrowRight } from 'lucide-react'

import { useMounted } from '@/hooks/use-mounted'

export function CartDrawer() {
  const {
    items,
    isDrawerOpen,
    closeDrawer,
    updateQuantity,
    removeItem,
    getSubtotal,
  } = useCartStore()

  const mounted = useMounted()

  if (!mounted) return null

  const subtotal = getSubtotal()

  return (
    <>
      {/* Backdrop */}
      {isDrawerOpen && (
        <div
          onClick={closeDrawer}
          className="fixed inset-0 bg-black/40 backdrop-blur-xs z-50 transition-opacity duration-300"
          aria-hidden="true"
        />
      )}

      {/* Slide-over Drawer Panel */}
      <div
        className={`fixed top-0 right-0 bottom-0 w-full sm:w-[440px] bg-white z-50 shadow-2xl flex flex-col transition-transform duration-300 ease-in-out ${
          isDrawerOpen ? 'translate-x-0' : 'translate-x-full'
        }`}
      >
        {/* Drawer Header */}
        <div className="p-5 border-b border-slate-100 flex items-center justify-between bg-slate-50/50">
          <div className="flex items-center gap-2">
            <ShoppingCart className="w-5 h-5 text-[#2196F3]" />
            <h2 className="text-lg font-bold text-[#0D1B2A]">Your Shopping Cart</h2>
            <span className="bg-[#E3F2FD] text-[#0D47A1] text-xs font-semibold px-2 py-0.5 rounded-full">
              {items.length}
            </span>
          </div>
          <button
            onClick={closeDrawer}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-700 hover:bg-slate-100 transition-colors"
            aria-label="Close cart"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Drawer Content */}
        <div className="flex-1 overflow-y-auto p-5 space-y-4">
          {items.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center p-6 space-y-3">
              <div className="w-16 h-16 rounded-full bg-[#E3F2FD] flex items-center justify-center text-[#2196F3]">
                <ShoppingCart className="w-8 h-8" />
              </div>
              <h3 className="text-base font-semibold text-[#0D1B2A]">Your cart is empty</h3>
              <p className="text-sm text-slate-500 max-w-xs">
                Looks like you haven&apos;t added any items to your shopping cart yet.
              </p>
              <button
                onClick={closeDrawer}
                className="mt-2 inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-[#2196F3] text-white text-sm font-medium hover:bg-[#0D47A1] transition-colors shadow-sm"
              >
                Start Shopping
              </button>
            </div>
          ) : (
            <ul className="divide-y divide-slate-100">
              {items.map((item) => (
                <li key={item.id} className="py-4 flex gap-3 first:pt-0 last:pb-0">
                  {/* Thumbnail */}
                  <div className="w-20 h-20 rounded-xl bg-slate-100 overflow-hidden shrink-0 border border-slate-200/60 flex items-center justify-center">
                    {item.image ? (
                      <img
                        src={getImageUrl(item.image)}
                        alt={item.name}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <ShoppingCart className="w-6 h-6 text-slate-400" />
                    )}
                  </div>

                  {/* Info */}
                  <div className="flex-1 flex flex-col justify-between min-w-0">
                    <div>
                      <div className="flex justify-between items-start gap-2">
                        <h4 className="text-sm font-semibold text-[#0D1B2A] truncate">
                          {item.name}
                        </h4>
                        <button
                          onClick={() => removeItem(item.id)}
                          className="text-slate-400 hover:text-red-500 p-0.5 transition-colors"
                          title="Remove item"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>

                      {(item.selectedSize || item.selectedColor) && (
                        <p className="text-xs text-slate-500 mt-0.5">
                          {item.selectedSize && `Size: ${item.selectedSize}`}
                          {item.selectedSize && item.selectedColor && ' | '}
                          {item.selectedColor && `Color: ${item.selectedColor}`}
                        </p>
                      )}
                    </div>

                    <div className="flex items-center justify-between mt-2">
                      <span className="text-sm font-bold text-[#0D47A1]">
                        {formatCurrency(item.price)}
                      </span>

                      {/* Quantity Controller */}
                      <div className="flex items-center border border-slate-200 rounded-lg bg-slate-50 overflow-hidden">
                        <button
                          onClick={() => updateQuantity(item.id, item.quantity - 1)}
                          className="p-1 hover:bg-slate-200 transition-colors text-slate-600"
                          aria-label="Decrease quantity"
                        >
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                        <span className="px-2.5 text-xs font-semibold text-slate-700 min-w-[24px] text-center">
                          {item.quantity}
                        </span>
                        <button
                          onClick={() => updateQuantity(item.id, item.quantity + 1)}
                          className="p-1 hover:bg-slate-200 transition-colors text-slate-600"
                          aria-label="Increase quantity"
                        >
                          <Plus className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>

        {/* Drawer Footer */}
        {items.length > 0 && (
          <div className="p-5 border-t border-slate-100 bg-slate-50/70 space-y-3">
            <div className="flex items-center justify-between text-sm">
              <span className="text-slate-600">Subtotal</span>
              <span className="text-base font-bold text-[#0D1B2A]">
                {formatCurrency(subtotal)}
              </span>
            </div>
            <p className="text-xs text-slate-500">
              Shipping, taxes, and coupons calculated at checkout.
            </p>

            <div className="grid grid-cols-2 gap-2 pt-1">
              <Link
                href="/cart"
                onClick={closeDrawer}
                className="w-full inline-flex items-center justify-center px-4 py-2.5 rounded-xl border border-slate-300 text-slate-700 text-sm font-medium hover:bg-white hover:border-slate-400 transition-colors text-center"
              >
                View Cart
              </Link>
              <Link
                href="/checkout"
                onClick={closeDrawer}
                className="w-full inline-flex items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl bg-[#2196F3] text-white text-sm font-semibold hover:bg-[#0D47A1] transition-colors shadow-sm text-center"
              >
                Checkout <ArrowRight className="w-4 h-4" />
              </Link>
            </div>
          </div>
        )}
      </div>
    </>
  )
}
