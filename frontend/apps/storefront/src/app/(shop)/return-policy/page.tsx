import React from 'react'
import type { Metadata } from 'next'
import Link from 'next/link'
import { RotateCcw, CheckCircle2, AlertCircle, Clock, ShieldCheck, ArrowRight } from 'lucide-react'

export const metadata: Metadata = {
  title: 'Return & Refund Policy — GOshop',
  description:
    'Read our 7-day return policy, guidelines for exchanges, return reasons, and refund timelines.',
}

export default function ReturnPolicyPage() {
  return (
    <div className="bg-[#FAFBFC] min-h-screen py-12">
      <div className="max-w-4xl mx-auto px-4 sm:px-6 space-y-10">
        {/* Header */}
        <div className="space-y-3">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-rose-50 text-rose-700 text-xs font-semibold">
            <RotateCcw className="w-3.5 h-3.5 text-rose-600" />
            <span>7-Day Return Guarantee</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold text-[#0D1B2A] tracking-tight">
            Return & Refund Policy
          </h1>
          <p className="text-xs text-slate-500">
            Hassle-free returns and prompt refunds to protect your shopping experience.
          </p>
        </div>

        {/* 4 Step Process Card */}
        <div className="p-8 sm:p-10 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-6">
          <h2 className="text-lg font-bold text-[#0D1B2A]">
            How to Return an Item in 4 Easy Steps
          </h2>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {[
              {
                step: '01',
                title: 'Submit Request',
                desc: 'Open Order Details in your Account and click "Request Return".',
              },
              {
                step: '02',
                title: 'Staff Approval',
                desc: 'Our team verifies your reason and images within 24 hours.',
              },
              {
                step: '03',
                title: 'Courier Pickup',
                desc: 'GHN collects the parcel from your address free of charge.',
              },
              {
                step: '04',
                title: 'Fast Refund',
                desc: 'Receive full refund to your MoMo or bank account.',
              },
            ].map((item) => (
              <div
                key={item.step}
                className="p-5 rounded-2xl bg-slate-50 border border-slate-200/70 space-y-2 relative"
              >
                <span className="text-2xl font-black text-[#2196F3] opacity-60">
                  {item.step}
                </span>
                <h3 className="text-xs font-bold text-[#0D1B2A]">{item.title}</h3>
                <p className="text-[11px] text-slate-500 leading-relaxed">{item.desc}</p>
              </div>
            ))}
          </div>
        </div>

        {/* Content Body */}
        <div className="p-8 sm:p-12 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-8 text-slate-700 text-xs sm:text-sm leading-relaxed">
          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              1. Eligible Conditions for Return
            </h2>
            <p>
              You are eligible for a free return and full refund if your item meets any of the following criteria
              within <strong>7 days</strong> from the delivery timestamp:
            </p>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li><strong>Manufacturing Defect:</strong> Stitching tears, non-functional electronics, or broken joints.</li>
              <li><strong>Incorrect Item / Variant:</strong> Wrong size, color, or model shipped compared to your order.</li>
              <li><strong>Transit Damage:</strong> Product arrived broken, severely crushed, or missing components.</li>
              <li><strong>Not as Described:</strong> Product materially deviates from photos or specifications.</li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              2. Return Requirements
            </h2>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li>Merchandise must be unused, unwashed, and in original condition.</li>
              <li>Original product boxes, accessories, warranty cards, and tags must be included.</li>
              <li>Clear photo or video evidence of the defect must be uploaded in the return form.</li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              3. Refund Timelines & Channels
            </h2>
            <div className="space-y-3">
              <div className="p-4 rounded-xl bg-slate-50 border border-slate-200/70">
                <p className="font-bold text-[#0D1B2A] text-xs">MoMo E-Wallet Orders</p>
                <p className="text-xs text-slate-600 mt-0.5">
                  Refunded directly to your connected MoMo wallet balance within <strong>1–2 business days</strong> after inspection.
                </p>
              </div>

              <div className="p-4 rounded-xl bg-slate-50 border border-slate-200/70">
                <p className="font-bold text-[#0D1B2A] text-xs">Cash on Delivery (COD) Orders</p>
                <p className="text-xs text-slate-600 mt-0.5">
                  Refunded via bank transfer (NAPAS 24/7) within <strong>2–3 business days</strong> once bank details are confirmed.
                </p>
              </div>
            </div>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              4. Return Shipping Costs
            </h2>
            <p>
              For any eligible return due to defects, shipping damage, or seller errors, <strong>GOshop covers 100% of return shipping fees</strong>.
              GHN courier will pick up the package directly from your doorstep.
            </p>
          </section>
        </div>
      </div>
    </div>
  )
}
