'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import { useCartStore } from '@/stores/cart-store'
import { formatCurrency } from '@/lib/format'
import {
  ShoppingCart,
  Trash2,
  Plus,
  Minus,
  ArrowRight,
  ArrowLeft,
  ShieldCheck,
  Tag,
} from 'lucide-react'
import { toast } from 'sonner'
import { useMounted } from '@/hooks/use-mounted'

export default function CartPage() {
  const {
    items,
    updateQuantity,
    removeItem,
    clearCart,
    getSubtotal,
  } = useCartStore()

  const mounted = useMounted()
  const [couponCode, setCouponCode] = useState('')
  const [discountAmount, setDiscountAmount] = useState(0)
  const [appliedCoupon, setAppliedCoupon] = useState<string | null>(null)

  if (!mounted) {
    return (
      <div className="min-h-[60vh] flex items-center justify-center">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    )
  }

  const subtotal = getSubtotal()
  const shippingFee = subtotal > 500000 || items.length === 0 ? 0 : 30000
  const grandTotal = Math.max(0, subtotal - discountAmount + shippingFee)

  const handleApplyCoupon = (e: React.FormEvent) => {
    e.preventDefault()
    if (!couponCode.trim()) return

    const code = couponCode.trim().toUpperCase()
    if (code === 'WELCOME10') {
      const disc = Math.round(subtotal * 0.1)
      setDiscountAmount(disc)
      setAppliedCoupon(code)
      toast.success('Coupon WELCOME10 applied! You saved 10% on your order.')
    } else {
      toast.error('Invalid or expired coupon code. Try WELCOME10.')
    }
  }

  const handleRemoveCoupon = () => {
    setDiscountAmount(0)
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
            className="text-xs font-semibold text-rose-500 hover:text-rose-700 transition-colors"
          >
            Clear Cart
          </button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          {/* Items List (8 cols) */}
          <div className="lg:col-span-8 space-y-4">
            <div className="rounded-2xl bg-white border border-slate-200/80 shadow-xs overflow-hidden divide-y divide-slate-100">
              {items.map((item) => (
                <div
                  key={item.id}
                  className="p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 hover:bg-slate-50/50 transition-colors"
                >
                  {/* Thumbnail & Info */}
                  <div className="flex items-center gap-4 min-w-0">
                    <div className="w-20 h-20 rounded-xl bg-slate-100 overflow-hidden shrink-0 border border-slate-200/60 flex items-center justify-center">
                      {item.image ? (
                        <img
                          src={item.image}
                          alt={item.name}
                          className="w-full h-full object-cover"
                        />
                      ) : (
                        <ShoppingCart className="w-6 h-6 text-slate-400" />
                      )}
                    </div>

                    <div className="min-w-0">
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
                        className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600"
                        aria-label="Decrease quantity"
                      >
                        <Minus className="w-3.5 h-3.5" />
                      </button>
                      <span className="px-3 text-xs font-bold text-slate-700 min-w-[28px] text-center">
                        {item.quantity}
                      </span>
                      <button
                        onClick={() => updateQuantity(item.id, item.quantity + 1)}
                        className="p-1.5 hover:bg-slate-200 transition-colors text-slate-600"
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
                      className="p-1.5 text-slate-400 hover:text-rose-500 rounded-lg transition-colors"
                      title="Remove item"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              ))}
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
                      placeholder="Promo Code (e.g. WELCOME10)"
                      className="w-full pl-8 pr-3 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs uppercase text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                    <Tag className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                  </div>
                  <button
                    type="submit"
                    className="px-3 py-2 rounded-xl bg-[#0D47A1] text-white text-xs font-semibold hover:bg-[#2196F3] transition-colors"
                  >
                    Apply
                  </button>
                </form>

                {appliedCoupon && (
                  <div className="flex items-center justify-between text-xs px-3 py-1.5 rounded-lg bg-emerald-50 text-emerald-700 border border-emerald-200">
                    <span>Applied: {appliedCoupon}</span>
                    <button
                      onClick={handleRemoveCoupon}
                      className="font-bold hover:underline"
                    >
                      Remove
                    </button>
                  </div>
                )}
              </div>

              {/* Breakdown */}
              <div className="space-y-2.5 text-xs text-slate-600 pt-2 border-t border-slate-100">
                <div className="flex justify-between">
                  <span>Subtotal</span>
                  <span className="font-semibold text-[#0D1B2A]">
                    {formatCurrency(subtotal)}
                  </span>
                </div>

                {discountAmount > 0 && (
                  <div className="flex justify-between text-emerald-600 font-semibold">
                    <span>Discount</span>
                    <span>-{formatCurrency(discountAmount)}</span>
                  </div>
                )}

                <div className="flex justify-between">
                  <span>Estimated Shipping</span>
                  <span className="font-semibold text-[#0D1B2A]">
                    {shippingFee === 0 ? (
                      <span className="text-emerald-600 font-bold">FREE</span>
                    ) : (
                      formatCurrency(shippingFee)
                    )}
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
              <Link
                href="/checkout"
                className="w-full py-3.5 px-6 rounded-2xl bg-[#2196F3] text-white text-sm font-bold flex items-center justify-center gap-2 hover:bg-[#0D47A1] transition-all shadow-md active:scale-98"
              >
                <span>Proceed to Checkout</span>
                <ArrowRight className="w-4 h-4" />
              </Link>

              {/* Trust Badge */}
              <div className="flex items-center gap-2 text-xs text-slate-500 pt-2">
                <ShieldCheck className="w-4 h-4 text-emerald-500 shrink-0" />
                <span>SSL Encrypted Checkout with VNPay & COD</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
