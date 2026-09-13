import React from 'react'
import Link from 'next/link'
import { Store, Compass, ArrowLeft, Search, ShoppingBag } from 'lucide-react'

export default function NotFound() {
  return (
    <div className="min-h-[75vh] flex items-center justify-center py-16 px-4 sm:px-6 bg-[#FAFBFC]">
      <div className="max-w-md w-full text-center space-y-6">
        <div className="relative mx-auto w-28 h-28 rounded-3xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center shadow-md">
          <Compass className="w-14 h-14 animate-pulse text-[#0D47A1]" />
          <span className="absolute -bottom-2 -right-2 px-2.5 py-0.5 rounded-full bg-[#0D1B2A] text-white text-xs font-black">
            404
          </span>
        </div>

        <div className="space-y-2">
          <h1 className="text-2xl sm:text-3xl font-black text-[#0D1B2A] tracking-tight">
            Page Not Found
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 leading-relaxed max-w-sm mx-auto">
            Sorry, the page or item you are looking for doesn&apos;t exist, has been removed,
            or the URL might be mistyped.
          </p>
        </div>

        <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
          <Link
            href="/"
            className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-6 py-3 rounded-2xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-md active:scale-98"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Return Home</span>
          </Link>
          <Link
            href="/products"
            className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-6 py-3 rounded-2xl border border-slate-200 bg-white text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-all shadow-xs"
          >
            <ShoppingBag className="w-4 h-4 text-[#2196F3]" />
            <span>Browse Products</span>
          </Link>
        </div>
      </div>
    </div>
  )
}
