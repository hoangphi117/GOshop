import React from "react";
import Link from "next/link";
import {
  ArrowLeft,
  ShoppingBag,
  Terminal,
  Zap,
  ShieldCheck,
} from "lucide-react";

export default function NotFound() {
  return (
    <div
      className="w-full max-w-4xl mx-auto p-4 sm:p-6 min-h-[75vh] flex items-center justify-center"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      <div className="w-full">
        {/* SECTION 1: MAIN NOT FOUND CONTENT (LEFT - 7 COLS) */}
        <div className="lg:col-span-7 flex flex-col bg-white rounded-2xl shadow-xl border border-[#E2E8F0] overflow-hidden">
          {/* Top Decorative Solid Line Stripe */}
          <div className="h-1.5 w-full bg-[#00637C]" />

          <div className="p-6 sm:p-8 flex flex-col items-center text-center justify-center flex-1 space-y-5">
            {/* Logo Brand Presence */}
            <div className="h-7 flex items-center justify-center">
              <span
                className="text-2xl font-bold text-[#007D9C] tracking-tight"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                GO<span className="text-[#F26522]">shop</span>
              </span>
            </div>

            {/* Mascot Avatar Node with "404" Badge instead of Compass */}
            <div className="w-20 h-20 rounded-2xl bg-[#EAEDFF] flex flex-col items-center justify-center shadow-xs relative group">
              <span
                className="text-2xl font-black text-[#00637C] tracking-tight"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                404
              </span>
            </div>

            {/* Title & Description */}
            <div className="space-y-2 max-w-sm">
              <h1
                className="text-2xl font-bold text-[#131B2A]"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                Page Not Found
              </h1>
              <p className="text-xs text-[#3E484D] leading-relaxed">
                Sorry, the page or item you are looking for doesn&apos;t exist,
                has been moved, or the URL might be mistyped.
              </p>
            </div>

            {/* Main Action Button */}
            <div className="pt-2 w-full max-w-xs">
              <Link
                href="/"
                className="w-full inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-[#00637C] hover:bg-[#004D62] text-white text-xs font-bold transition-all shadow-md shadow-[#00637C]/25"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                <ArrowLeft className="w-4 h-4" />
                <span>Return Home</span>
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
