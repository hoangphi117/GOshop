import React from 'react'
import type { Metadata } from 'next'
import { ShieldCheck, Lock, Eye, Database, KeyRound } from 'lucide-react'

export const metadata: Metadata = {
  title: 'Privacy Policy — GOshop',
  description:
    'Learn how GOshop collects, uses, protects, and handles your personal information, credentials, and data.',
}

export default function PrivacyPage() {
  return (
    <div className="bg-[#FAFBFC] min-h-screen py-12">
      <div className="max-w-4xl mx-auto px-4 sm:px-6 space-y-10">
        {/* Header */}
        <div className="space-y-3">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-50 text-emerald-700 text-xs font-semibold">
            <ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />
            <span>Data Protection & Privacy</span>
          </div>
          <h1 className="text-3xl sm:text-4xl font-extrabold text-[#0D1B2A] tracking-tight">
            Privacy Policy
          </h1>
          <p className="text-xs text-slate-500">
            Last Updated: September 13, 2026 • Compliant with international data privacy standards
          </p>
        </div>

        {/* Content Body */}
        <div className="p-8 sm:p-12 rounded-3xl bg-white border border-slate-200/80 shadow-xs space-y-8 text-slate-700 text-xs sm:text-sm leading-relaxed">
          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              1. Overview & Commitment
            </h2>
            <p>
              At <strong>GOshop</strong>, we are committed to upholding your trust and safeguarding your personal
              information. This Privacy Policy details how we collect, use, store, and protect your data when you visit
              our website or make purchases through our storefront.
            </p>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              2. Information We Collect
            </h2>
            <div className="space-y-2">
              <div className="p-4 rounded-xl bg-slate-50 border border-slate-200/70 space-y-1">
                <p className="font-bold text-[#0D1B2A]">Personal & Contact Details</p>
                <p className="text-xs text-slate-600">
                  Full name, email address, phone number, and physical shipping address provided during checkout or account registration.
                </p>
              </div>

              <div className="p-4 rounded-xl bg-slate-50 border border-slate-200/70 space-y-1">
                <p className="font-bold text-[#0D1B2A]">Order & Transaction Records</p>
                <p className="text-xs text-slate-600">
                  Purchased products, items in cart, order totals, chosen payment methods (MoMo / COD), and invoice history.
                </p>
              </div>

              <div className="p-4 rounded-xl bg-slate-50 border border-slate-200/70 space-y-1">
                <p className="font-bold text-[#0D1B2A]">Technical & Device Logs</p>
                <p className="text-xs text-slate-600">
                  IP address, browser type, operating system, and session tokens strictly used for security authentication and fraud prevention.
                </p>
              </div>
            </div>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              3. How We Use Your Information
            </h2>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li>To fulfill and dispatch your orders via our courier partners (GHN Express).</li>
              <li>To process secure electronic payments and generate digital VAT invoices.</li>
              <li>To provide order tracking notifications, live updates, and customer support.</li>
              <li>To detect and prevent fraudulent activities, duplicate orders, or unauthorized account access.</li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              4. Data Security & Storage Architecture
            </h2>
            <p>
              We implement enterprise security standards to protect your data across all layers:
            </p>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li>
                <strong>Encrypted Token Payloads:</strong> User session payloads are encrypted using <strong>AES-256</strong> before being embedded in signed JWT tokens.
              </li>
              <li>
                <strong>HttpOnly Cookies:</strong> Sensitive Refresh Tokens are stored in isolated, secure, <code>HttpOnly</code> cookies inaccessible to malicious JavaScript or XSS vectors.
              </li>
              <li>
                <strong>SSL/TLS Transport:</strong> All data transmissions between your browser and our servers are encrypted via HTTPS with modern TLS protocols.
              </li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              5. Third-Party Disclosures
            </h2>
            <p>
              GOshop does not sell, rent, or trade your personal information. We only share essential data with verified
              operational partners:
            </p>
            <ul className="list-disc pl-5 space-y-1.5 text-slate-600">
              <li><strong>GHN Express:</strong> Recipient name, phone number, and delivery address to deliver packages.</li>
              <li><strong>MoMo Payment Gateway:</strong> Order ID and subtotal to generate secure payment URLs.</li>
            </ul>
          </section>

          <section className="space-y-3">
            <h2 className="text-base sm:text-lg font-bold text-[#0D1B2A]">
              6. Your Privacy Rights
            </h2>
            <p>
              You have the right to access, update, export, or request deletion of your personal data at any time.
              You can manage your saved addresses and profile details directly within your Account Settings or contact
              our Data Protection Officer at <span className="font-semibold text-[#0D47A1]">privacy@goshop.com</span>.
            </p>
          </section>
        </div>
      </div>
    </div>
  )
}
