'use client'

import React, { useState } from 'react'
import Link from 'next/link'
import {
  Search,
  ChevronDown,
  HelpCircle,
  Package,
  Truck,
  CreditCard,
  RotateCcw,
  ShieldCheck,
  Headphones,
  ArrowRight,
} from 'lucide-react'

interface FAQItem {
  id: string
  category: 'ordering' | 'shipping' | 'payment' | 'returns' | 'account'
  question: string
  answer: string
}

const FAQ_DATA: FAQItem[] = [
  // Ordering
  {
    id: 'ord-1',
    category: 'ordering',
    question: 'How do I place an order on GOshop?',
    answer:
      'Browse our catalog, select your desired item and variant (size, color), and click "Add to Cart" or "Buy Now". When ready, proceed to Checkout, select your delivery address and payment method (MoMo E-Wallet or Cash on Delivery), and click "Place Order".',
  },
  {
    id: 'ord-2',
    category: 'ordering',
    question: 'Can I modify or cancel my order after placing it?',
    answer:
      'You can cancel your order directly from your Account > Orders page as long as the status is "Pending" or "Confirmed". Once the order moves to "Shipped" or "In Transit" with GHN, it cannot be canceled directly; you may request a return upon delivery.',
  },
  {
    id: 'ord-3',
    category: 'ordering',
    question: 'How do I apply coupon discount codes?',
    answer:
      'In your Shopping Cart or Checkout page, locate the "Order Summary" card, enter your promotional voucher code (e.g., SOLOLEVELING), and click "Apply". If valid, the discount amount will immediately deduct from your order total.',
  },

  // Shipping
  {
    id: 'shp-1',
    category: 'shipping',
    question: 'What shipping partners do you use and how fast is delivery?',
    answer:
      'We partner with Giao Hàng Nhanh (GHN Express) for nationwide delivery. Inner-city orders (Ho Chi Minh City, Hanoi) typically arrive in 1-2 business days. Provincial and other nationwide addresses arrive in 2-4 business days.',
  },
  {
    id: 'shp-2',
    category: 'shipping',
    question: 'How do I track my delivery in real-time?',
    answer:
      'Navigate to Account > My Orders and click "Live Tracking" on your order. You can view the live progress bar and click "Sync Tracking" to fetch latest milestone updates directly from GHN Express.',
  },
  {
    id: 'shp-3',
    category: 'shipping',
    question: 'How are shipping fees calculated?',
    answer:
      'Shipping rates are calculated in real-time via the GHN API based on your destination district/province and total package weight.',
  },

  // Payment
  {
    id: 'pay-1',
    category: 'payment',
    question: 'What payment methods does GOshop support?',
    answer:
      'We support MoMo E-Wallet (QR code scan & app gateway) and Cash on Delivery (COD). All electronic transactions are secured with bank-level SSL encryption.',
  },
  {
    id: 'pay-2',
    category: 'payment',
    question: 'Is my payment information safe?',
    answer:
      'Yes, absolutely. GOshop does not store your raw banking credentials or payment pins. All e-wallet interactions are processed directly through certified payment gateways.',
  },
  {
    id: 'pay-3',
    category: 'payment',
    question: 'What happens if my MoMo payment fails or is canceled?',
    answer:
      'If your online payment fails or is cancelled, your order status remains "Pending" and no amount is deducted. You can retry payment from the Order Detail screen or choose COD.',
  },

  // Returns
  {
    id: 'ret-1',
    category: 'returns',
    question: 'What is your return and exchange policy?',
    answer:
      'We offer a 7-day hassle-free return window for all delivered products if they are defective, damaged in transit, missing accessories, or not as described.',
  },
  {
    id: 'ret-2',
    category: 'returns',
    question: 'How do I submit a return request?',
    answer:
      'Go to Account > My Orders > Order Details, and click the "Request Return / Refund" button. Select your reason, provide details or photos, and our team will review and approve within 24 hours.',
  },
  {
    id: 'ret-3',
    category: 'returns',
    question: 'How soon will I receive my refund?',
    answer:
      'Once the returned package is received and inspected at our warehouse, refunds are disbursed to your original MoMo account within 1-2 business days, or via bank transfer for COD orders.',
  },

  // Account
  {
    id: 'acc-1',
    category: 'account',
    question: 'How do I reset my account password?',
    answer:
      'Click "Forgot Password" on the sign-in page, enter your registered email address, and check your inbox for a secure password reset link. The reset link is valid for 1 hour.',
  },
  {
    id: 'acc-2',
    category: 'account',
    question: 'How do I save multiple shipping addresses?',
    answer:
      'Visit Account > Saved Addresses to add, edit, or set default delivery addresses with full Province, District, and Ward selections.',
  },
]

const CATEGORIES = [
  { id: 'all', label: 'All Topics', icon: HelpCircle },
  { id: 'ordering', label: 'Ordering', icon: Package },
  { id: 'shipping', label: 'Shipping & Delivery', icon: Truck },
  { id: 'payment', label: 'Payments', icon: CreditCard },
  { id: 'returns', label: 'Returns & Refunds', icon: RotateCcw },
  { id: 'account', label: 'Account & Security', icon: ShieldCheck },
]

export default function FAQPage() {
  const [activeCategory, setActiveCategory] = useState<string>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [openIds, setOpenIds] = useState<Record<string, boolean>>({
    'ord-1': true,
    'shp-1': true,
  })

  const toggleAccordion = (id: string) => {
    setOpenIds((prev) => ({
      ...prev,
      [id]: !prev[id],
    }))
  }

  const filteredFAQs = FAQ_DATA.filter((item) => {
    const matchesCategory =
      activeCategory === 'all' || item.category === activeCategory
    const query = searchQuery.toLowerCase().trim()
    const matchesSearch =
      !query ||
      item.question.toLowerCase().includes(query) ||
      item.answer.toLowerCase().includes(query)
    return matchesCategory && matchesSearch
  })

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-12">
      <div className="max-w-4xl mx-auto px-4 sm:px-6 space-y-10">
        {/* Header & Search */}
        <div className="text-center space-y-4 max-w-2xl mx-auto">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-semibold">
            <HelpCircle className="w-3.5 h-3.5 text-[#2196F3]" />
            <span>Help Center & FAQs</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold text-[#0D1B2A] tracking-tight">
            Frequently Asked Questions
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 leading-relaxed">
            Find fast answers to common questions regarding orders, GHN shipping, MoMo payments, and returns.
          </p>

          {/* Search Box */}
          <div className="relative max-w-md mx-auto pt-2">
            <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search by keywords (e.g. shipping, return, momo)..."
              className="w-full pl-10 pr-4 py-3 rounded-2xl bg-white border border-slate-200/90 text-xs text-[#0D1B2A] shadow-xs focus:outline-none focus:border-[#2196F3]"
            />
          </div>
        </div>

        {/* Category Tabs */}
        <div className="flex items-center gap-2 overflow-x-auto pb-2 scrollbar-none justify-start sm:justify-center">
          {CATEGORIES.map((cat) => {
            const Icon = cat.icon
            const isActive = activeCategory === cat.id
            return (
              <button
                key={cat.id}
                onClick={() => setActiveCategory(cat.id)}
                className={`inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold whitespace-nowrap transition-all cursor-pointer ${
                  isActive
                    ? 'bg-[#2196F3] text-white shadow-xs'
                    : 'bg-white text-slate-600 border border-slate-200/80 hover:bg-slate-50'
                }`}
              >
                <Icon className="w-3.5 h-3.5" />
                <span>{cat.label}</span>
              </button>
            )
          })}
        </div>

        {/* FAQ Accordion List */}
        <div className="space-y-3">
          {filteredFAQs.length === 0 ? (
            <div className="p-12 text-center bg-white rounded-3xl border border-slate-200/80 shadow-xs space-y-3">
              <p className="text-sm font-bold text-[#0D1B2A]">
                No matching answers found
              </p>
              <p className="text-xs text-slate-500 max-w-xs mx-auto">
                Try searching for a different keyword or browse through the category tabs above.
              </p>
              <button
                onClick={() => {
                  setSearchQuery('')
                  setActiveCategory('all')
                }}
                className="mt-2 text-xs font-semibold text-[#2196F3] hover:underline"
              >
                Reset Search Filters
              </button>
            </div>
          ) : (
            filteredFAQs.map((item) => {
              const isOpen = Boolean(openIds[item.id])
              return (
                <div
                  key={item.id}
                  className="rounded-2xl bg-white border border-slate-200/80 shadow-xs overflow-hidden transition-all"
                >
                  <button
                    onClick={() => toggleAccordion(item.id)}
                    className="w-full p-5 text-left flex items-center justify-between gap-4 cursor-pointer hover:bg-slate-50/50 transition-colors"
                  >
                    <span className="text-sm font-bold text-[#0D1B2A]">
                      {item.question}
                    </span>
                    <div
                      className={`w-7 h-7 rounded-full bg-slate-100 flex items-center justify-center text-slate-500 shrink-0 transition-transform duration-200 ${
                        isOpen ? 'rotate-180 bg-[#E3F2FD] text-[#2196F3]' : ''
                      }`}
                    >
                      <ChevronDown className="w-4 h-4" />
                    </div>
                  </button>

                  {isOpen && (
                    <div className="px-5 pb-5 pt-1 text-xs sm:text-sm text-slate-600 leading-relaxed border-t border-slate-100">
                      {item.answer}
                    </div>
                  )}
                </div>
              )
            })
          )}
        </div>

        {/* Still Need Help Banner */}
        <div className="p-8 rounded-3xl bg-gradient-to-r from-[#0D47A1] to-[#2196F3] text-white flex flex-col sm:flex-row items-center justify-between gap-6 shadow-md">
          <div className="flex items-center gap-4 text-center sm:text-left">
            <div className="w-14 h-14 rounded-2xl bg-white/15 backdrop-blur-xs flex items-center justify-center shrink-0">
              <Headphones className="w-7 h-7 text-white" />
            </div>
            <div>
              <h3 className="text-base sm:text-lg font-bold">Still have questions?</h3>
              <p className="text-xs text-[#E3F2FD] mt-0.5">
                Our customer care team is online and ready to assist you 7 days a week.
              </p>
            </div>
          </div>

          <Link
            href="/contact"
            className="px-6 py-3 rounded-xl bg-white text-[#0D47A1] text-xs font-bold hover:bg-[#E3F2FD] transition-colors shadow-sm shrink-0 flex items-center gap-1.5"
          >
            <span>Contact Support</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </Link>
        </div>
      </div>
    </div>
  )
}
