import React from 'react'
import type { Product } from '@/lib/api-server'
import { ProductCard } from './product-card'
import { PackageOpen } from 'lucide-react'
import Link from 'next/link'

interface ProductGridProps {
  products: Product[]
  emptyMessage?: string
  columnsClassName?: string
}

export function ProductGrid({
  products,
  emptyMessage = 'No products found.',
  columnsClassName = 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4',
}: ProductGridProps) {
  if (products.length === 0) {
    return (
      <div className="rounded-3xl border border-dashed border-slate-300 bg-white p-12 text-center my-6 flex flex-col items-center justify-center">
        <div className="w-16 h-16 rounded-full bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mb-4">
          <PackageOpen className="w-8 h-8" />
        </div>
        <h3 className="text-base font-bold text-[#0D1B2A]">{emptyMessage}</h3>
        <p className="text-sm text-slate-500 max-w-sm mt-1 mb-5">
          Try adjusting your search criteria or explore our featured departments.
        </p>
        <Link
          href="/products"
          className="px-5 py-2.5 rounded-full bg-[#2196F3] text-white text-xs font-semibold hover:bg-[#0D47A1] transition-colors"
        >
          View All Products
        </Link>
      </div>
    )
  }

  return (
    <div className={`grid ${columnsClassName} gap-6`}>
      {products.map((product) => (
        <ProductCard key={product.uuid} product={product} />
      ))}
    </div>
  )
}
