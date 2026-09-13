import React from 'react'
import type { Metadata } from 'next'
import Link from 'next/link'
import { Truck, Clock, ShieldCheck, MapPin, Box, CheckCircle2 } from 'lucide-react'

export const metadata: Metadata = {
  title: 'Shipping & Delivery Policy — GOshop',
  description:
    'Learn about GOshop shipping times, delivery rates, GHN Express logistics, and order tracking across Vietnam.',
}

export default function ShippingPolicyPage() {
  return (
    <div className="bg-[#FAFBFC] min-h-screen py-12">
      <div className="max-w-4xl mx-auto px-4 sm:px-6 space-y-10">
        {/* Header */}
        <div className="space-y-3">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-semibold">
            <Truck className="w-3.5 h-3.5 text-[#2196F3]" />
            <span>Fast & Reliable Delivery</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold text-[#0D1B2A] tracking-tight">
            Shipping & Delivery Policy
          </h1>
          <p className="text-xs text-slate-500">
            Powered by Giao Hàng Nhanh (GHN Express) • Serving all 63 provinces in Vietnam
          </p>
        </div>

        {/* Timeline Table Card */}
        <div className="p-8 sm:p-10 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
          <h2 className="text-lg font-bold text-[#0D1B2A]">
            Estimated Delivery Timeframes
          </h2>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className="p-5 rounded-2xl bg-slate-50 border border-slate-200/70 space-y-2">
              <div className="flex items-center gap-2 text-[#0D47A1] font-bold text-sm">
                <Clock className="w-4 h-4" />
                <span>1 – 2 Business Days</span>
              </div>
              <h3 className="text-xs font-bold text-[#0D1B2A]">Major Metropolitans</h3>
              <p className="text-[11px] text-slate-500">
                Ho Chi Minh City, Hanoi, Da Nang, Can Tho, Hai Phong.
              </p>
            </div>

            <div className="p-5 rounded-2xl bg-slate-50 border border-slate-200/70 space-y-2">
              <div className="flex items-center gap-2 text-[#0D47A1] font-bold text-sm">
                <Clock className="w-4 h-4" />
                <span>2 – 4 Business Days</span>
              </div>
              <h3 className="text-xs font-bold text-[#0D1B2A]">Provincial Centers</h3>
              <p className="text-[11px] text-slate-500">
                Central provincial cities and suburban district areas nationwide.
              </p>
            </div>

            <div className="p-5 rounded-2xl bg-slate-50 border border-slate-200/70 space-y-2">
              <div className="flex items-center gap-2 text-[#0D47A1] font-bold text-sm">
                <Clock className="w-4 h-4" />
                <span>3 – 5 Business Days</span>
              </div>
              <h3 className="text-xs font-bold text-[#0D1B2A]">Remote & Island Areas</h3>
              <p className="text-[11px] text-slate-500">
                Remote communes, mountainous regions, and island districts.
              </p>
            </div>
          </div>
        </div>

        {/* Content Body */}
        <div className="p-8 sm:p-12 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-8 text-slate-700 text-xs sm:text-sm leading-relaxed">
          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              1. Shipping Fee Calculation
            </h2>
            <p>
              Shipping rates are calculated dynamically in real-time through the GHN Express API during checkout.
              Factors determining the shipping fee include:
            </p>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li>The geographical distance from our central fulfillment hub in Ho Chi Minh City to your delivery address.</li>
              <li>The total volumetric weight and dimensions of the packaged items.</li>
              <li>Any active free-shipping promotions or discount vouchers applied at checkout.</li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              2. Packaging & Product Protection
            </h2>
            <p>
              Every GOshop order is handled with maximum care. Boss plushies, apparel, and collectible figures
              are wrapped in waterproof protective polybags, secured with bubble wrap cushions, and boxed in
              high-strength corrugated cardboard to prevent transit crushing.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              3. Order Tracking & Status Updates
            </h2>
            <p>
              Once your order is picked, packed, and handed over to GHN, an official Tracking Code is assigned.
              You can track your package milestone updates by:
            </p>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li>Visiting <strong>Account &gt; My Orders &gt; Order Details</strong>.</li>
              <li>Clicking the <strong>&quot;Live Tracking&quot;</strong> button to view the interactive delivery timeline.</li>
              <li>Clicking <strong>&quot;Sync Tracking&quot;</strong> for instant courier sync.</li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              4. Inspection on Delivery (Co-check)
            </h2>
            <p>
              Customers are encouraged to visually inspect the external packaging before accepting the parcel
              from the GHN courier. If the outer box is significantly torn, wet, or opened, you may refuse delivery
              and contact our support team for an immediate replacement.
            </p>
          </section>
        </div>
      </div>
    </div>
  )
}
