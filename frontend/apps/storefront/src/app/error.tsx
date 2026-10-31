"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { AlertTriangle, RefreshCw, ArrowLeft, HelpCircle } from "lucide-react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("App Error Boundary caught:", error);
  }, [error]);

  return (
    <div
      className="w-full max-w-4xl mx-auto p-4 sm:p-6 min-h-[75vh] flex items-center justify-center"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      <div className="w-full grid grid-cols-1 lg:grid-cols-12 gap-4 items-stretch">
        {/* SECTION 1: MAIN ERROR DISPLAY (LEFT - 7 COLS) */}
        <div className="lg:col-span-7 flex flex-col bg-white rounded-2xl shadow-xl border border-[#E2E8F0] overflow-hidden">
          {/* Top Solid Line Stripe */}
          <div className="h-1.5 w-full bg-[#9F3800]" />

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

            {/* Error Icon Avatar Node */}
            <div className="w-20 h-20 rounded-2xl bg-[#FFF1F0] flex items-center justify-center border border-[#FFCCC7] shadow-xs relative">
              <AlertTriangle className="w-10 h-10 text-[#D9381E]" />
            </div>

            {/* Error Status Pill */}
            <div className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-[#FFF1F0] text-[#D9381E] font-semibold text-xs shadow-2xs">
              <span className="w-2 h-2 rounded-full bg-[#D9381E]" />
              <span>500 • Unexpected Application Error</span>
            </div>

            {/* Title & Description */}
            <div className="space-y-2 max-w-sm">
              <h1
                className="text-2xl font-bold text-[#131B2A]"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                Something Went Wrong
              </h1>
              <p className="text-xs text-[#3E484D] leading-relaxed">
                An unexpected error occurred while loading this page. Our
                technical team has been notified.
              </p>
              {error?.digest && (
                <p className="text-[11px] font-mono text-[#6F797E] bg-[#F8FAFC] p-1.5 rounded border border-[#E2E8F0]">
                  Error Digest: {error.digest}
                </p>
              )}
            </div>

            {/* Retry Action Button */}
            <div className="pt-2 w-full max-w-xs">
              <button
                type="button"
                onClick={() => reset()}
                className="w-full inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-[#00637C] hover:bg-[#004D62] text-white text-xs font-bold transition-all shadow-md shadow-[#00637C]/25 cursor-pointer"
                style={{
                  fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                }}
              >
                <RefreshCw className="w-4 h-4" />
                <span>Try Again</span>
              </button>
            </div>
          </div>
        </div>

        {/* SECTION 2: HELPFUL NAVIGATION & SUPPORT (RIGHT - 5 COLS) */}
        <div className="lg:col-span-5 flex flex-col bg-white rounded-2xl shadow-xl border border-[#E2E8F0] overflow-hidden">
          {/* Top Solid Line Stripe */}
          <div className="h-1.5 w-full bg-[#00637C]" />

          <div className="p-6 sm:p-8 flex flex-col h-full bg-[#F8FAFC] justify-between space-y-6">
            <div className="space-y-4">
              <div className="flex items-center gap-2 pb-3 border-b border-[#E2E8F0]">
                <h3
                  className="text-xs font-bold text-[#131B2A] uppercase tracking-wide"
                  style={{
                    fontFamily: 'var(--font-heading, "Rubik", sans-serif)',
                  }}
                >
                  Recovery Options
                </h3>
              </div>

              <p className="text-xs text-[#3E484D] leading-relaxed">
                If retrying doesn&apos;t resolve the issue, try navigating back
                to safety or contacting support:
              </p>

              {/* Navigation Links */}
              <div className="space-y-2">
                <Link
                  href="/"
                  className="flex items-center justify-between p-3 rounded-lg bg-white border border-[#E2E8F0] hover:border-[#007D9C] text-xs font-bold text-[#131B2A] transition-all group"
                >
                  <div className="flex items-center gap-2.5">
                    <ArrowLeft className="w-4 h-4 text-[#00637C]" />
                    <span>Return to Homepage</span>
                  </div>
                  <span className="text-[#6F797E] group-hover:text-[#00637C] transition-colors">
                    →
                  </span>
                </Link>

                <Link
                  href="/help-center"
                  className="flex items-center justify-between p-3 rounded-lg bg-white border border-[#E2E8F0] hover:border-[#007D9C] text-xs font-bold text-[#131B2A] transition-all group"
                >
                  <div className="flex items-center gap-2.5">
                    <HelpCircle className="w-4 h-4 text-[#00637C]" />
                    <span>Contact Help Center</span>
                  </div>
                  <span className="text-[#6F797E] group-hover:text-[#00637C] transition-colors">
                    →
                  </span>
                </Link>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
