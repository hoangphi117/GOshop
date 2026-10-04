"use client";

import React, { useState, useEffect, useRef } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Search, X, Loader2, Tag, ArrowRight, Sparkles } from "lucide-react";
import { ProductSuggestion } from "@/lib/api-server";
import { API_BASE_URL, API_KEY } from "@/lib/api";

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
  const [isExpanded, setIsExpanded] = useState(false);
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
    }, 150); // 150ms debounce for ultra-responsive feel

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
        if (!isMobile && !query) {
          setIsExpanded(false);
        }
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isMobile, query]);

  const handleSelectProduct = (slug: string) => {
    router.push(`/products/${slug}`);
    setIsOpen(false);
    setIsExpanded(false);
    if (onClose) onClose();
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (query.trim()) {
      router.push(`/products?search=${encodeURIComponent(query.trim())}`);
      setIsOpen(false);
      setIsExpanded(false);
      if (onClose) onClose();
    }
  };

  // Mobile layout
  if (isMobile) {
    return (
      <div ref={containerRef} className="relative w-full mb-3">
        <form onSubmit={handleSubmit} className="relative">
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setIsOpen(true);
            }}
            onFocus={() => setIsOpen(true)}
            placeholder="Search products (with typo tolerance)..."
            className="w-full pl-9 pr-8 py-2.5 rounded-xl bg-slate-100 text-sm text-[#0D1B2A] outline-none border border-slate-200/80 focus:border-[#2196F3] focus:bg-white transition-all shadow-xs"
          />
          <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          {isLoading ? (
            <Loader2 className="w-4 h-4 text-slate-400 absolute right-3 top-1/2 -translate-y-1/2 animate-spin" />
          ) : query ? (
            <button
              type="button"
              onClick={() => {
                setQuery("");
                setSuggestions([]);
                inputRef.current?.focus();
              }}
              className="p-1 text-slate-400 hover:text-slate-600 absolute right-2.5 top-1/2 -translate-y-1/2"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          ) : null}
        </form>

        {/* Mobile Suggestions Dropdown */}
        {isOpen && query.trim().length > 0 && (
          <div className="absolute left-0 right-0 top-full mt-1.5 bg-white rounded-2xl shadow-xl border border-slate-200/80 overflow-hidden z-50">
            {suggestions.length > 0 ? (
              <div className="py-2">
                <div className="px-3 py-1 text-[11px] font-semibold text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                  <Sparkles className="w-3 h-3 text-[#2196F3]" />
                  <span>Instant Suggestions</span>
                </div>
                {suggestions.map((item) => (
                  <button
                    key={item.uuid}
                    type="button"
                    onClick={() => handleSelectProduct(item.slug)}
                    className="w-full px-3 py-2 flex items-center gap-3 hover:bg-slate-50 transition-colors text-left"
                  >
                    <div className="relative w-9 h-9 rounded-lg bg-slate-100 shrink-0 overflow-hidden border border-slate-200/60">
                      {item.image_url ? (
                        <Image
                          src={item.image_url}
                          alt={item.name}
                          fill
                          sizes="36px"
                          className="object-cover"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-[10px] text-slate-400">
                          N/A
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-medium text-[#0D1B2A] truncate">
                        {item.name}
                      </p>
                      <div className="flex items-center gap-2 mt-0.5">
                        <span className="text-xs font-bold text-[#0D47A1]">
                          ${item.price.toFixed(2)}
                        </span>
                        {item.category_name && (
                          <span className="text-[10px] text-slate-500 bg-slate-100 px-1.5 py-0.5 rounded-sm">
                            {item.category_name}
                          </span>
                        )}
                      </div>
                    </div>
                  </button>
                ))}
                <button
                  type="button"
                  onClick={handleSubmit}
                  className="w-full mt-1 px-3 py-2 bg-slate-50 border-t border-slate-100 text-xs text-[#2196F3] font-semibold flex items-center justify-between hover:bg-[#E3F2FD] transition-colors"
                >
                  <span>See all results for &quot;{query}&quot;</span>
                  <ArrowRight className="w-3.5 h-3.5" />
                </button>
              </div>
            ) : !isLoading ? (
              <div className="p-4 text-center text-xs text-slate-500">
                No matching products found. Press Enter to search.
              </div>
            ) : null}
          </div>
        )}
      </div>
    );
  }

  // Desktop Expandable Search Bar
  return (
    <div ref={containerRef} className="relative">
      {isExpanded ? (
        <form
          onSubmit={handleSubmit}
          className="flex items-center absolute right-0 top-1/2 -translate-y-1/2 bg-white rounded-full shadow-lg border border-slate-200 px-3.5 py-2 w-80 z-20 transition-all duration-200 animate-in fade-in zoom-in-95"
        >
          <Search className="w-4 h-4 text-[#2196F3] shrink-0 mr-2" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setIsOpen(true);
            }}
            onFocus={() => setIsOpen(true)}
            placeholder="Search (typo-tolerant)..."
            className="w-full text-xs text-[#0D1B2A] bg-transparent outline-none placeholder:text-slate-400"
            autoFocus
          />
          {isLoading ? (
            <Loader2 className="w-3.5 h-3.5 text-slate-400 animate-spin mr-1" />
          ) : query ? (
            <button
              type="button"
              onClick={() => {
                setQuery("");
                setSuggestions([]);
                inputRef.current?.focus();
              }}
              className="p-0.5 text-slate-400 hover:text-slate-600 mr-1"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          ) : null}
          <button
            type="button"
            onClick={() => {
              setIsExpanded(false);
              setIsOpen(false);
            }}
            className="p-1 text-slate-400 hover:text-slate-600 rounded-full hover:bg-slate-100 ml-0.5"
            aria-label="Close search"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </form>
      ) : (
        <button
          onClick={() => {
            setIsExpanded(true);
            setIsOpen(true);
          }}
          className={`w-9 h-9 rounded-full flex items-center justify-center transition-colors cursor-pointer ${
            isTransparent
              ? "text-white/90 hover:text-white hover:bg-white/15"
              : "text-slate-600 hover:text-[#0D1B2A] hover:bg-slate-100"
          }`}
          aria-label="Open instant search"
        >
          <Search className="w-4 h-4" />
        </button>
      )}

      {/* Desktop Suggestions Popup */}
      {isExpanded && isOpen && query.trim().length > 0 && (
        <div className="absolute right-0 top-full mt-3 w-84 bg-white rounded-2xl shadow-2xl border border-slate-200/90 overflow-hidden z-30 animate-in fade-in slide-in-from-top-2 duration-150">
          {suggestions.length > 0 ? (
            <div>
              <div className="max-h-80 overflow-y-auto py-1 divide-y divide-slate-50">
                {suggestions.map((item) => (
                  <button
                    key={item.uuid}
                    type="button"
                    onClick={() => handleSelectProduct(item.slug)}
                    className="w-full px-3.5 py-2 flex items-center gap-3 hover:bg-[#F0F7FF] transition-colors text-left group cursor-pointer"
                  >
                    <div className="relative w-10 h-10 rounded-lg bg-slate-100 shrink-0 overflow-hidden border border-slate-200/80">
                      {item.image_url ? (
                        <Image
                          src={item.image_url}
                          alt={item.name}
                          fill
                          sizes="40px"
                          className="object-cover group-hover:scale-105 transition-transform duration-200"
                        />
                      ) : (
                        <div className="w-full h-full flex items-center justify-center text-[10px] text-slate-400">
                          N/A
                        </div>
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-semibold text-[#0D1B2A] group-hover:text-[#0D47A1] truncate transition-colors">
                        {item.name}
                      </p>
                      <div className="flex items-center gap-2 mt-0.5">
                        <span className="text-xs font-bold text-[#0D47A1]">
                          ${item.price.toFixed(2)}
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
                className="w-full px-3.5 py-2.5 bg-slate-50 border-t border-slate-100 text-xs text-[#2196F3] font-semibold flex items-center justify-between hover:bg-[#E3F2FD] hover:text-[#0D47A1] transition-colors cursor-pointer"
              >
                <span>View all products for &quot;{query}&quot;</span>
                <ArrowRight className="w-3.5 h-3.5" />
              </button>
            </div>
          ) : !isLoading ? (
            <div className="p-4 text-center">
              <p className="text-xs text-slate-600 font-medium">
                No instant matches found for &quot;{query}&quot;
              </p>
              <p className="text-[11px] text-slate-400 mt-1">
                Press Enter to search the full catalog
              </p>
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
