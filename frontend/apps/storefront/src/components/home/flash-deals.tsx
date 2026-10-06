"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import Image from "next/image";
import { ArrowRight, Zap, ShoppingBag } from "lucide-react";
import type { Product } from "@/lib/api-server";
import { formatCurrency } from "@/lib/format";

interface FlashDealsProps {
  products?: Product[];
}

export function FlashDeals({ products = [] }: FlashDealsProps) {
  const [timeLeft, setTimeLeft] = useState({
    hours: 2,
    minutes: 45,
    seconds: 18,
  });

  useEffect(() => {
    const timer = setInterval(() => {
      setTimeLeft((prev) => {
        if (prev.seconds > 0) {
          return { ...prev, seconds: prev.seconds - 1 };
        }
        if (prev.minutes > 0) {
          return { ...prev, minutes: 59, seconds: 59 };
        }
        if (prev.hours > 0) {
          return { hours: prev.hours - 1, minutes: 59, seconds: 59 };
        }
        return { hours: 2, minutes: 59, seconds: 59 };
      });
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  const pad = (n: number) => n.toString().padStart(2, "0");

  // Filter or take top 4 products for flash deals
  const dealProducts = products.slice(0, 4);

  if (dealProducts.length === 0) {
    return null;
  }

  return (
    <section
      id="deals"
      className="max-w-7xl mx-auto px-4 sm:px-6 mb-12 sm:mb-16"
    >
      <div className="bg-gradient-to-r from-white via-slate-50/80 to-white border-2 border-[#F26522]/20 rounded-3xl p-6 sm:p-8 shadow-sm">
        {/* Header with Countdown */}
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4 pb-6 border-b border-slate-200/80">
          <div className="flex items-center gap-3">
            <div className="w-11 h-11 rounded-2xl bg-[#F26522] text-white flex items-center justify-center shadow-md">
              <Zap className="w-6 h-6 fill-white text-white" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-xl sm:text-2xl font-extrabold text-slate-900 uppercase tracking-tight font-heading">
                  Flash Deals of the Day
                </h2>
                <span className="bg-[#FFE2D5] text-[#F26522] text-[11px] font-black px-2.5 py-0.5 rounded-full font-heading">
                  HOT DEALS
                </span>
              </div>
              <p className="text-xs sm:text-sm text-slate-500 font-body mt-0.5">
                Instant checkout discounts. Refreshes twice daily.
              </p>
            </div>
          </div>

          {/* Dynamic Countdown Clock */}
          <div className="flex items-center gap-2 bg-[#071F2C] text-white px-4 py-2 rounded-2xl shadow-xs font-body">
            <span className="text-[11px] font-bold text-[#E0F7FC] mr-1 uppercase tracking-wider">
              ENDS IN:
            </span>
            <div className="flex items-center gap-1.5 font-bold text-sm font-heading">
              <span className="bg-white/10 px-2 py-1 rounded text-[#00ADD8]">
                {pad(timeLeft.hours)}
              </span>
              <span>:</span>
              <span className="bg-white/10 px-2 py-1 rounded text-[#00ADD8]">
                {pad(timeLeft.minutes)}
              </span>
              <span>:</span>
              <span className="bg-[#F26522] px-2 py-1 rounded text-white">
                {pad(timeLeft.seconds)}
              </span>
            </div>
          </div>
        </div>

        {/* 4 Hot Deals Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-5 pt-6">
          {dealProducts.map((product, idx) => {
            const originalPrice =
              product.compare_at_price &&
              product.compare_at_price > product.price
                ? product.compare_at_price
                : Math.round(product.price * 1.25);
            const discountPercent = Math.round(
              ((originalPrice - product.price) / originalPrice) * 100,
            );
            const imageUrl =
              product.image_url ||
              product.images?.[0]?.image_url ||
              "https://images.unsplash.com/photo-1550009158-9ebf69173e03?w=800";
            const soldPercent = Math.min(
              95,
              Math.max(
                45,
                100 - (product.stock > 0 ? (product.stock % 40) * 2 : 10),
              ),
            );

            return (
              <Link
                key={product.uuid || idx}
                href={`/products/${product.slug || product.uuid}`}
                className="bg-white rounded-2xl p-4 border border-slate-200/90 hover:border-[#F26522]/50 hover:shadow-xl transition-all duration-300 flex flex-col justify-between group cursor-pointer"
              >
                <div>
                  <div className="relative w-full aspect-square bg-slate-50 rounded-xl overflow-hidden mb-3.5 border border-slate-100">
                    <span className="absolute top-2.5 left-2.5 bg-[#F26522] text-white text-[11px] font-black px-2 py-0.5 rounded-md z-10 shadow-xs font-heading">
                      -{discountPercent}%
                    </span>
                    <Image
                      src={imageUrl}
                      alt={product.name}
                      fill
                      sizes="(max-width: 640px) 100vw, (max-width: 1024px) 50vw, 25vw"
                      className="object-cover group-hover:scale-105 transition-transform duration-300"
                    />
                  </div>

                  <span className="text-[11px] font-bold text-[#007D9C] uppercase tracking-wider font-heading">
                    {product.category_name || "Official Merch"}
                  </span>
                  <h3 className="text-sm font-bold text-slate-900 line-clamp-1 group-hover:text-[#007D9C] transition-colors font-heading mt-0.5">
                    {product.name}
                  </h3>
                  <div className="flex items-baseline gap-2 mt-1.5">
                    <span className="text-base font-extrabold text-[#F26522] font-heading">
                      {formatCurrency(product.price)}
                    </span>
                    <span className="text-xs text-slate-400 line-through font-body">
                      {formatCurrency(originalPrice)}
                    </span>
                  </div>
                </div>

                {/* Stock Bar */}
                <div className="mt-4 pt-3 border-t border-slate-100">
                  <div className="flex justify-between text-[11px] font-bold mb-1 font-heading">
                    <span className="text-[#F26522]">
                      Only {product.stock} in stock!
                    </span>
                    <span className="text-slate-500">{soldPercent}% sold</span>
                  </div>
                  <div className="w-full h-2 bg-slate-100 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-gradient-to-r from-[#00ADD8] to-[#F26522] rounded-full"
                      style={{ width: `${soldPercent}%` }}
                    />
                  </div>
                </div>
              </Link>
            );
          })}
        </div>
      </div>
    </section>
  );
}
