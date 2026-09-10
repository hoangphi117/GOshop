'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import { useCartStore } from '@/stores/cart-store'
import { useAuthStore } from '@/stores/auth-store'
import { formatCurrency } from '@/lib/format'
import { getImageUrl } from '@/lib/utils'
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
} from 'lucide-react'
import { toast } from 'sonner'
import { useMounted } from '@/hooks/use-mounted'

export default function CartPage() {
  const {
    items,
    updateQuantity,
    removeItem,
    clearCart,
    toggleSelectItem,
    selectAllItems,
    removeSelectedItems,
    appliedCoupon,
    setAppliedCoupon,
  } = useCartStore()
  const { user, accessToken } = useAuthStore()

  const mounted = useMounted()
  const [couponCode, setCouponCode] = useState('')
  const [isApplyingCoupon, setIsApplyingCoupon] = useState(false)

  if (!mounted) {
    return (
      <div className="min-h-[60vh] flex items-center justify-center">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    )
  }

  if (!user || !accessToken) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-20 h-20 rounded-3xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto shadow-xs">
            <Lock className="w-10 h-10" />
          </div>
          <h1 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
            Sign In Required
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            Please sign in to your GOshop account to view and manage your shopping cart.
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              href="/login?redirect=/cart"
              className="w-full sm:w-auto px-6 py-3 rounded-2xl bg-[#2196F3] text-white text-sm font-semibold hover:bg-[#0D47A1] transition-all shadow-md text-center"
            >
              Sign In Now
            </Link>
            <Link
              href="/products"
              className="w-full sm:w-auto px-6 py-3 rounded-2xl border border-slate-200 bg-white text-slate-700 text-sm font-semibold hover:bg-slate-50 transition-all text-center"
            >
              Continue Shopping
            </Link>
          </div>
        </div>
      </div>
    )
  }

  const selectedItems = items.filter((item) => item.selected !== false)
  const selectedCount = selectedItems.length
  const allSelected = items.length > 0 && selectedCount === items.length

  const subtotal = selectedItems.reduce(
    (total, item) => total + item.price * item.quantity,
    0
  )
  const discountAmount = appliedCoupon ? appliedCoupon.discountAmount : 0
  const grandTotal = Math.max(0, subtotal - discountAmount)

  const handleApplyCoupon = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!couponCode.trim()) return

    const code = couponCode.trim().toUpperCase()

    if (!accessToken) {
      toast.error('Please sign in to apply coupon codes.')
      return
    }

    if (subtotal <= 0) {
      toast.error('Please select items before applying a coupon.')
      return
    }

    setIsApplyingCoupon(true)
    try {
      const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
      const res = await fetch(`${baseUrl}/api/v1/coupons/preview`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-API-KEY': process.env.NEXT_PUBLIC_API_KEY || 'abcd-1234-1111',
          'Authorization': `Bearer ${accessToken}`,
        },
        body: JSON.stringify({
          code,
          subtotal_amount: subtotal,
        }),
      })

      const data = await res.json().catch(() => null)

      if (res.ok && data?.data) {
        setAppliedCoupon({
          code: data.data.coupon_code,
          name: data.data.coupon_name,
          discountAmount: data.data.discount_amount,
        })
        toast.success(`Coupon ${data.data.coupon_code} applied! Saved ${formatCurrency(data.data.discount_amount)}.`)
        setCouponCode('')
      } else {
        const errMsg = data?.detail || data?.error || data?.message || 'Invalid or expired coupon code.'
        toast.error(errMsg)
      }
    } catch {
      toast.error('Network error while verifying coupon.')
    } finally {
      setIsApplyingCoupon(false)
    }
  }

  const handleRemoveCoupon = () => {
    setAppliedCoupon(null)
    setCouponCode('')
    toast.info('Coupon removed.')
  }

  if (items.length === 0) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-20 h-20 rounded-3xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto shadow-xs">
            <ShoppingCart className="w-10 h-10" />
          </div>
          <h1 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
            Your Cart is Empty
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            You don&apos;t have any products in your cart yet. Explore our latest arrivals
            and best selling items now!
          </p>
          <div className="pt-2">
            <Link
              href="/products"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-2xl bg-[#2196F3] text-white text-sm font-semibold hover:bg-[#0D47A1] transition-all shadow-md"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>Continue Shopping</span>
            </Link>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-10">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        <div className="flex items-center justify-between pb-6 border-b border-slate-200/80 mb-8">
          <h1 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
            Shopping Cart ({items.length} {items.length === 1 ? 'item' : 'items'})
          </h1>
          <button
            onClick={() => {
              if (window.confirm('Are you sure you want to clear your cart?')) {
                clearCart()
                toast.info('Cart cleared.')
              }
            }}
            className="text-xs font-semibold text-rose-500 hover:text-rose-700 transition-colors cursor-pointer"
          >
            Clear Cart
          </button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          {/* Items List (8 cols) */}
          <div className="lg:col-span-8 space-y-4">
            <div className="rounded-2xl bg-white border border-slate-200/80 shadow-xs overflow-hidden divide-y divide-slate-100">
              {/* Select All Action Bar */}
              <div className="px-5 py-3.5 bg-slate-50/90 flex items-center justify-between">
                <label className="flex items-center gap-3 cursor-pointer select-none text-xs font-bold text-[#0D1B2A]">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    onChange={(e) => selectAllItems(e.target.checked)}
                    className="w-4 h-4 rounded text-[#2196F3] focus:ring-[#2196F3] border-slate-300 accent-[#2196F3] cursor-pointer"
                  />
                  <span>Select All ({selectedCount}/{items.length} items)</span>
                </label>

                {selectedCount > 0 && (
                  <button
                    onClick={() => {
                      if (window.confirm(`Remove ${selectedCount} selected items from your cart?`)) {
                        removeSelectedItems()
                        toast.info('Selected items removed.')
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
                const isSelected = item.selected !== false
                return (
                  <div
                    key={item.id}
                    className={`p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 transition-colors ${
                      !isSelected ? 'bg-slate-50/40 opacity-70' : 'hover:bg-slate-50/50'
                    }`}
                  >
                    {/* Checkbox, Thumbnail & Info */}
                    <div className="flex items-center gap-3.5 min-w-0 flex-1">
                      <input
                        type="checkbox"
                        checked={isSelected}
                        onChange={() => toggleSelectItem(item.id)}
                        aria-label={`Select ${item.name}`}
                        className="w-4 h-4 rounded text-[#2196F3] focus:ring-[#2196F3] border-slate-300 accent-[#2196F3] cursor-pointer shrink-0"
                      />

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

                      <div className="min-w-0 flex-1">
                        <Link
                          href={`/products/${item.productUuid}`}
                          className="text-sm font-bold text-[#0D1B2A] hover:text-[#2196F3] transition-colors truncate block"
                        >
                          {item.name}
                        </Link>
                        {(item.selectedSize || item.selectedColor) && (
                          <p className="text-xs text-slate-500 mt-0.5">
                            {item.selectedSize && `Size: ${item.selectedSize}`}
                            {item.selectedSize && item.selectedColor && ' • '}
                            {item.selectedColor && `Color: ${item.selectedColor}`}
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
                          onClick={() => updateQuantity(item.id, item.quantity - 1)}
                          className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600 cursor-pointer"
                          aria-label="Decrease quantity"
                        >
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                        <span className="px-3 text-xs font-bold text-slate-700 min-w-[28px] text-center">
                          {item.quantity}
                        </span>
                        <button
                          onClick={() => updateQuantity(item.id, item.quantity + 1)}
                          className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600 cursor-pointer"
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
                )
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
              <h2 className="text-base font-bold text-[#0D1B2A]">Order Summary</h2>

              {/* Coupon Form */}
              <div className="space-y-2">
                <form onSubmit={handleApplyCoupon} className="flex gap-2">
                  <div className="relative flex-1">
                    <input
                      type="text"
                      value={couponCode}
                      onChange={(e) => setCouponCode(e.target.value)}
                      placeholder="Promo Code (e.g. SOLOLEVELING)"
                      disabled={isApplyingCoupon}
                      className="w-full pl-8 pr-3 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs uppercase text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] disabled:opacity-60"
                    />
                    <Tag className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                  </div>
                  <button
                    type="submit"
                    disabled={isApplyingCoupon || !couponCode.trim()}
                    className="px-3 py-2 rounded-xl bg-[#0D47A1] text-white text-xs font-semibold hover:bg-[#2196F3] transition-colors cursor-pointer disabled:opacity-60"
                  >
                    {isApplyingCoupon ? 'Verifying...' : 'Apply'}
                  </button>
                </form>

                {appliedCoupon && (
                  <div className="flex items-center justify-between text-xs px-3 py-2 rounded-lg bg-emerald-50 text-emerald-700 border border-emerald-200">
                    <div className="flex flex-col">
                      <span className="font-bold">Code: {appliedCoupon.code}</span>
                      {appliedCoupon.name && (
                        <span className="text-[11px] text-emerald-600 font-normal">{appliedCoupon.name}</span>
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
                  <span>Subtotal ({selectedCount} {selectedCount === 1 ? 'item' : 'items'})</span>
                  <span className="font-semibold text-[#0D1B2A]">
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
                    <p className="text-[10px] text-slate-400">GHN Express live rate</p>
                  </div>
                  <span className="font-semibold text-slate-500 text-xs">
                    {selectedCount === 0 ? '—' : 'Calculated at checkout'}
                  </span>
                </div>

                <div className="flex justify-between text-base font-extrabold text-[#0D1B2A] pt-3 border-t border-slate-200">
                  <span>Total</span>
                  <span className="text-[#0D47A1] font-black">
                    {formatCurrency(grandTotal)}
                  </span>
                </div>
              </div>

              {/* Checkout Button */}
              {selectedCount > 0 ? (
                <Link
                  href="/checkout"
                  className="w-full py-3.5 px-6 rounded-2xl bg-[#2196F3] text-white text-sm font-bold flex items-center justify-center gap-2 hover:bg-[#0D47A1] transition-all shadow-md active:scale-98 text-center"
                >
                  <span>Proceed to Checkout ({selectedCount})</span>
                  <ArrowRight className="w-4 h-4" />
                </Link>
              ) : (
                <button
                  type="button"
                  disabled
                  className="w-full py-3.5 px-6 rounded-2xl bg-slate-200 text-slate-400 text-sm font-bold flex items-center justify-center gap-2 cursor-not-allowed"
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
  )
}
