import React from "react";
import type { Product } from "@/lib/api-server";
import { ProductCard } from "./product-card";
import { PackageOpen } from "lucide-react";
import Link from "next/link";

interface ProductGridProps {
  products: Product[];
  emptyMessage?: string;
  columnsClassName?: string;
}

export function ProductGrid({
  products,
  emptyMessage = "No products found.",
  columnsClassName = "grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-4 xl:grid-cols-5",
}: ProductGridProps) {
  if (products.length === 0) {
    return (
      <div className="rounded-xs border border-dashed border-slate-300 bg-white p-10 text-center my-4 flex flex-col items-center justify-center shadow-xs">
        <div className="w-12 h-12 rounded-xs bg-[#E0F7FC] text-[#007D9C] flex items-center justify-center mb-3">
          <PackageOpen className="w-6 h-6" />
        </div>
        <h3 className="text-sm font-bold text-slate-900 font-heading">
          {emptyMessage}
        </h3>
        <p className="text-xs text-slate-500 max-w-sm mt-1 mb-4 font-body">
          Try adjusting your search criteria or explore other official departments.
        </p>
        <Link
          href="/products"
          className="px-4 py-2 rounded-xs bg-[#007D9C] text-white text-xs font-bold hover:bg-[#00637C] font-heading shadow-xs"
        >
          View All Products
        </Link>
      </div>
    );
  }

  return (
    <div className={`grid ${columnsClassName} gap-5`}>
      {products.map((product) => (
        <ProductCard key={product.uuid} product={product} />
      ))}
    </div>
  );
}
