"use client";

import React, { useState, useEffect, useRef } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Search, X, Loader2, Tag, ArrowRight, Sparkles } from "lucide-react";
import { ProductSuggestion } from "@/lib/api-server";
import { API_BASE_URL, API_KEY } from "@/lib/api";
import { formatCurrency } from "@/lib/format";

interface SearchAutocompleteProps {
  isTransparent?: boolean;
  onClose?: () => void;
  isMobile?: boolean;
}

export function SearchAutocomplete({
  isTransparent = false,
  onClose,
  isMobile = false,
}: SearchAutocompleteProps) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [suggestions, setSuggestions] = useState<ProductSuggestion[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Debounced search suggestion query
  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed) {
      setSuggestions([]);
      setIsLoading(false);
      return;
    }

    setIsLoading(true);
    const timer = setTimeout(async () => {
      try {
        const res = await fetch(
          `${API_BASE_URL}/api/v1/products/suggest?q=${encodeURIComponent(trimmed)}&limit=6`,
          {
            headers: API_KEY ? { "X-API-KEY": API_KEY } : undefined,
          },
        );
        if (res.ok) {
          const json = await res.json();
          setSuggestions(json?.data || []);
        }
      } catch (err) {
        console.error("Failed to fetch product suggestions:", err);
      } finally {
        setIsLoading(false);
      }
    }, 150); // 150ms debounce for fast responsive feel

    return () => clearTimeout(timer);
  }, [query]);

  // Click outside to close dropdown
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const handleSelectProduct = (slug: string) => {
    router.push(`/products/${slug}`);
    setIsOpen(false);
    if (onClose) onClose();
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (query.trim()) {
      router.push(`/products?search=${encodeURIComponent(query.trim())}`);
      setIsOpen(false);
      if (onClose) onClose();
    }
  };

  return (
    <div
      ref={containerRef}
      className={`relative w-full ${isMobile ? "mb-3" : "max-w-xl mx-auto"}`}
    >
      <form onSubmit={handleSubmit} className="relative w-full group">
        <div className="relative flex items-center w-full">
          <Search className="w-4 h-4 text-[#007D9C] absolute left-3.5 top-1/2 -translate-y-1/2 pointer-events-none transition-colors" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setIsOpen(true);
            }}
            onFocus={() => setIsOpen(true)}
            placeholder="Search GOshop"
            className="w-full pl-10 pr-28 py-2 rounded-md text-xs text-slate-900 outline-none border-0 bg-white shadow-xs focus:ring-2 focus:ring-[#00ADD8] transition-all font-body placeholder:text-slate-400"
          />

          {isLoading ? (
            <Loader2 className="w-3.5 h-3.5 text-[#00ADD8] absolute right-3.5 top-1/2 -translate-y-1/2 animate-spin" />
          ) : query ? (
            <button
              type="button"
              onClick={() => {
                setQuery("");
                setSuggestions([]);
                inputRef.current?.focus();
              }}
              className="p-1 text-slate-400 hover:text-slate-600 rounded-full hover:bg-slate-200/60 absolute right-2.5 top-1/2 -translate-y-1/2 cursor-pointer transition-colors"
              aria-label="Clear search query"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          ) : null}
        </div>
      </form>

      {/* Suggestions Dropdown */}
      {isOpen && query.trim().length > 0 && (
        <div className="absolute left-0 right-0 top-full mt-2 bg-white rounded-2xl shadow-2xl border border-slate-200/90 overflow-hidden z-50">
          {suggestions.length > 0 ? (
            <div>
              <div className="max-h-80 overflow-y-auto divide-y divide-slate-50">
                {suggestions.map((item) => (
                  <button
                    key={item.uuid}
                    type="button"
                    onClick={() => handleSelectProduct(item.slug)}
                    className="w-full px-3.5 py-2 flex items-center gap-3 hover:bg-[#E6F7FB] text-left  cursor-pointer"
                  >
                    <div className="relative w-10 h-10 rounded-lg bg-slate-100 overflow-hidden border border-slate-200/80">
                      {item.image_url ? (
                        <Image
                          src={item.image_url}
                          alt={item.name}
                          fill
                          sizes="40px"
                          className="object-cover"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-[10px] text-slate-400">
                          N/A
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-semibold text-slate-900 group-hover:text-[#007D9C] truncate font-heading">
                        {item.name}
                      </p>
                      <div className="flex items-center gap-2 mt-0.5 font-body">
                        <span className="text-xs font-extrabold text-[#007D9C]">
                          {formatCurrency(item.price)}
                        </span>
                        {item.category_name && (
                          <span className="text-[10px] text-slate-500 bg-slate-100 px-1.5 py-0.5 rounded-md flex items-center gap-0.5">
                            <Tag className="w-2.5 h-2.5" />
                            {item.category_name}
                          </span>
                        )}
                      </div>
                    </div>
                  </button>
                ))}
              </div>
              <button
                type="button"
                onClick={handleSubmit}
                className="w-full px-3.5 py-2.5 bg-slate-50 border-t border-slate-100 text-xs text-[#007D9C] font-bold flex items-center justify-between hover:bg-[#E6F7FB] transition-colors cursor-pointer font-heading"
              >
                <span>View all results for &quot;{query}&quot;</span>
                <ArrowRight className="w-3.5 h-3.5 text-[#00ADD8]" />
              </button>
            </div>
          ) : !isLoading ? (
            <div className="p-4 text-center">
              <p className="text-xs text-slate-600 font-medium font-body">
                No instant matches found for &quot;{query}&quot;
              </p>
              <p className="text-[11px] text-slate-400 mt-1 font-body">
                Press Enter to search the full catalog
              </p>
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
