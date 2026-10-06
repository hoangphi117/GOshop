import React from "react";
import { Truck, ShieldCheck, RefreshCw, Headphones } from "lucide-react";

export function TrustBadges() {
  const badges = [
    {
      icon: Truck,
      title: "Free Express Shipping",
      description: "On all orders nationwide over 500,000₫",
    },
    {
      icon: ShieldCheck,
      title: "100% Genuine Guaranteed",
      description: "Directly licensed with authentic merchandise guarantee",
    },
    {
      icon: RefreshCw,
      title: "30-Day Easy Returns",
      description: "Hassle-free replacement policy if any defect is found",
    },
    {
      icon: Headphones,
      title: "24/7 Gamer Support",
      description: "Dedicated gaming support team always on standby",
    },
  ];

  return (
    <section aria-label="Benefits" className="bg-slate-50/80 py-10 border-b border-slate-200/60">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 sm:gap-6">
          {badges.map((badge, idx) => {
            const Icon = badge.icon;
            return (
              <div
                key={idx}
                className="group flex items-start gap-4 p-5 rounded-2xl bg-white border border-slate-200/80 shadow-xs hover:shadow-md hover:border-emerald-500/30 transition-all duration-200"
              >
                <div className="w-11 h-11 rounded-xl bg-emerald-50 text-emerald-700 flex items-center justify-center shrink-0 group-hover:bg-emerald-600 group-hover:text-white transition-colors duration-200">
                  <Icon className="w-5 h-5 transition-transform duration-200 group-hover:scale-110" />
                </div>
                <div>
                  <h4 className="text-sm font-bold text-slate-900 font-heading">
                    {badge.title}
                  </h4>
                  <p className="text-xs text-slate-500 mt-1 leading-relaxed font-body">
                    {badge.description}
                  </p>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
