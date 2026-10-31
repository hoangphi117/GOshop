import React from 'react'
import type { Metadata } from 'next'
import Link from 'next/link'
import { FileText, Shield, ArrowRight, HelpCircle } from 'lucide-react'

export const metadata: Metadata = {
  title: 'Terms of Service — GOshop',
  description:
    'Read the official Terms of Service governing the use of the GOshop online store and platform services.',
}

export default function TermsPage() {
  return (
    <div className="bg-[#FAFBFC] min-h-screen py-12">
      <div className="max-w-4xl mx-auto px-4 sm:px-6 space-y-10">
        {/* Header */}
        <div className="space-y-3">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-semibold">
            <FileText className="w-3.5 h-3.5 text-[#2196F3]" />
            <span>Legal Agreement</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold text-[#0D1B2A] tracking-tight">
            Terms of Service
          </h1>
          <p className="text-xs text-slate-500">
            Last Updated: September 13, 2026 • Effective Date: January 1, 2026
          </p>
        </div>

        {/* Content Body */}
        <div className="p-8 sm:p-12 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-8 text-slate-700 text-xs sm:text-sm leading-relaxed">
          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              1. Acceptance of Terms
            </h2>
            <p>
              By accessing, browsing, or purchasing products on <strong>GOshop</strong> (&quot;the Service&quot;),
              you acknowledge that you have read, understood, and agree to be bound by these Terms of Service.
              If you do not agree with any portion of these terms, please discontinue using the platform immediately.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              2. User Accounts & Security
            </h2>
            <p>
              When you create an account on GOshop, you must provide accurate and complete registration information.
              You are solely responsible for maintaining the confidentiality of your credentials and account session.
              Any activity occurring under your authenticated account is your responsibility. If you suspect unauthorized
              access, please notify us immediately at <span className="font-semibold text-[#0D47A1]">support@goshop.com</span>.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              3. Product Information & Pricing
            </h2>
            <p>
              We strive to display accurate descriptions, specifications, imagery, and prices for all items in our catalog.
              However, typographical errors or stock discrepancies may occasionally occur. GOshop reserves the right to
              correct any inaccuracies or cancel orders containing erroneous pricing before shipment, with a full prompt refund.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              4. Orders, Payments & Vouchers
            </h2>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li>
                <strong>Payment Methods:</strong> We accept MoMo E-Wallet and Cash on Delivery (COD).
              </li>
              <li>
                <strong>Promotional Codes:</strong> Coupons and promotional vouchers must be applied prior to completing checkout and cannot be redeemed for cash or transferred.
              </li>
              <li>
                <strong>Order Confirmation:</strong> An order confirmation email and account dashboard entry validate receipt of your order.
              </li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              5. Shipping & Delivery
            </h2>
            <p>
              Deliveries are carried out via certified logistics partners including Giao Hàng Nhanh (GHN Express).
              Estimated delivery times are guidelines and may vary based on weather, peak holidays, or remote addresses.
              For more details, please review our{' '}
              <Link href="/shipping-policy" className="text-[#2196F3] font-semibold hover:underline">
                Shipping & Delivery Policy
              </Link>.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              6. Returns, Exchanges & Refunds
            </h2>
            <p>
              We provide a 7-day return window for items that are defective, damaged in transit, or not matching their
              catalog description. Returned merchandise must include original packaging and tags. For complete steps, refer to our{' '}
              <Link href="/return-policy" className="text-[#2196F3] font-semibold hover:underline">
                Return & Refund Policy
              </Link>.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              7. Intellectual Property
            </h2>
            <p>
              All trademarks, logos, merchandise artwork, graphics, and interface designs on GOshop are the property
              of GOshop or its respective licensing partners. Unauthorized copying, modification, or redistribution is strictly prohibited.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              8. Contact & Legal Notices
            </h2>
            <p>
              If you have any questions or legal inquiries concerning these Terms of Service, please reach out to:
            </p>
            <div className="p-4 rounded-2xl bg-slate-50 border border-slate-200 text-xs space-y-1">
              <p className="font-bold text-[#0D1B2A]">GOshop Legal Department</p>
              <p>Email: legal@goshop.com / support@goshop.com</p>
              <p>Address: District 1, Ho Chi Minh City, Vietnam</p>
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}
