import React from 'react'
import { Truck, ShieldCheck, RefreshCw, Headphones } from 'lucide-react'

export function TrustBadges() {
  const badges = [
    {
      icon: Truck,
      title: 'Free Shipping',
      description: 'On all orders nationwide over 500,000₫',
    },
    {
      icon: ShieldCheck,
      title: '100% Genuine',
      description: 'Directly sourced with authentic manufacturer warranty',
    },
    {
      icon: RefreshCw,
      title: '30-Day Free Return',
      description: 'Hassle-free return policy if any defect is found',
    },
    {
      icon: Headphones,
      title: '24/7 Dedicated Support',
      description: 'Our customer support team is always ready to assist',
    },
  ]

  return (
    <section className="bg-[#E3F2FD]/50 py-10 border-b border-slate-200/60">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          {badges.map((badge, idx) => {
            const Icon = badge.icon
            return (
              <div
                key={idx}
                className="flex items-start gap-4 p-4 rounded-2xl bg-white border border-slate-200/60 shadow-xs hover:shadow-md transition-shadow"
              >
                <div className="w-12 h-12 rounded-xl bg-[#E3F2FD] text-[#0D47A1] flex items-center justify-center shrink-0">
                  <Icon className="w-6 h-6 text-[#2196F3]" />
                </div>
                <div>
                  <h4 className="text-sm font-bold text-[#0D1B2A]">
                    {badge.title}
                  </h4>
                  <p className="text-xs text-slate-500 mt-1 leading-relaxed">
                    {badge.description}
                  </p>
                </div>
              </div>
            )
          })}
        </div>
      </div>
    </section>
  )
}
