"use client";

import React, { useState, useEffect, useRef } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Search, X, Loader2, Tag, ArrowRight, Sparkles } from "lucide-react";
import { ProductSuggestion } from "@/lib/api-server";
import { API_BASE_URL, API_KEY, MEILISEARCH_URL, MEILISEARCH_SEARCH_KEY } from "@/lib/api";
import { formatCurrency } from "@/lib/format";

import { getImageUrl } from "@/lib/utils";

interface SearchAutocompleteProps {
  isTransparent?: boolean;
  onClose?: () => void;
  isMobile?: boolean;
  variant?: "navbar" | "compact";
  placeholder?: string;
  initialValue?: string;
  preserveParams?: {
    category?: string;
    sort?: string;
    min_price?: number;
    max_price?: number;
    min_rating?: number;
  };
  className?: string;
}

export function SearchAutocomplete({
  isTransparent = false,
  onClose,
  isMobile = false,
  variant = "navbar",
  placeholder,
  initialValue = "",
  preserveParams,
  className = "",
}: SearchAutocompleteProps) {
  const router = useRouter();
  const [query, setQuery] = useState(initialValue);
  const [suggestions, setSuggestions] = useState<ProductSuggestion[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Sync state if initialValue prop changes
  useEffect(() => {
    setQuery(initialValue);
  }, [initialValue]);

  // Debounced search suggestion query to Meilisearch
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
        // 1. Direct Meilisearch query (offloads PostgreSQL Master & Go Backend API)
        if (MEILISEARCH_URL) {
          try {
            const meiliRes = await fetch(
              `${MEILISEARCH_URL}/indexes/products/search`,
              {
                method: "POST",
                headers: {
                  "Content-Type": "application/json",
                  ...(MEILISEARCH_SEARCH_KEY
                    ? { Authorization: `Bearer ${MEILISEARCH_SEARCH_KEY}` }
                    : {}),
                },
                body: JSON.stringify({
                  q: trimmed,
                  limit: 6,
                  attributesToRetrieve: [
                    "id",
                    "product_id",
                    "product_uuid",
                    "product_name",
                    "product_slug",
                    "product_price",
                    "product_image_url",
                    "category_name",
                  ],
                }),
              },
            );
            if (meiliRes.ok) {
              const meiliJson = await meiliRes.json();
              const hits: ProductSuggestion[] = (meiliJson?.hits || []).map(
                (hit: any) => ({
                  uuid: hit.id || hit.product_uuid || String(hit.product_id),
                  name: hit.product_name,
                  slug: hit.product_slug,
                  price: hit.product_price,
                  image_url: hit.product_image_url,
                  category_name: hit.category_name,
                }),
              );
              setSuggestions(hits);
              return;
            }
          } catch (e) {
            console.warn("Direct Meilisearch search failed, falling back to backend API:", e);
          }
        }

        // 2. Fallback to Go Backend API proxy
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
    }, 150); // 150ms debounce for ultra-fast response

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

  const buildUrl = (searchVal: string) => {
    const q = new URLSearchParams();
    if (preserveParams?.category) q.set("category", preserveParams.category);
    if (searchVal.trim()) q.set("search", searchVal.trim());
    if (preserveParams?.sort) q.set("sort", preserveParams.sort);
    if (preserveParams?.min_price !== undefined)
      q.set("min_price", preserveParams.min_price.toString());
    if (preserveParams?.max_price !== undefined)
      q.set("max_price", preserveParams.max_price.toString());
    if (preserveParams?.min_rating !== undefined)
      q.set("min_rating", preserveParams.min_rating.toString());

    const qs = q.toString();
    return `/products${qs ? `?${qs}` : ""}`;
  };

  const handleSelectProduct = (slug: string) => {
    router.push(`/products/${slug}`);
    setIsOpen(false);
    if (onClose) onClose();
  };

  const handleSubmit = (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    setIsOpen(false);
    if (onClose) onClose();
    router.push(buildUrl(query));
  };

  const isCompact = variant === "compact";

  return (
    <div
      ref={containerRef}
      className={`relative w-full ${
        isCompact
          ? className
          : isMobile
            ? "mb-3"
            : `max-w-xl mx-auto ${className}`
      }`}
    >
      <form onSubmit={handleSubmit} className="relative w-full group">
        <div className="relative flex items-center w-full">
          <Search
            className={`${
              isCompact
                ? "w-3.5 h-3.5 text-slate-400 left-3"
                : "w-4 h-4 text-[#007D9C] left-3.5"
            } absolute top-1/2 -translate-y-1/2 pointer-events-none transition-colors`}
          />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setIsOpen(true);
            }}
            onFocus={() => {
              if (query.trim().length > 0) setIsOpen(true);
            }}
            onKeyDown={(e) => {
              if (e.key === "Escape") setIsOpen(false);
            }}
            placeholder={
              placeholder ||
              (isCompact ? "Filter within results..." : "Search GOshop")
            }
            className={
              isCompact
                ? "w-full pl-9 pr-8 py-1.5 rounded-xs border border-slate-300 bg-white text-xs text-slate-900 placeholder-slate-400 outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] font-body transition-colors"
                : "w-full pl-10 pr-28 py-2 rounded-md text-xs text-slate-900 outline-none border-0 bg-white shadow-xs focus:ring-2 focus:ring-[#00ADD8] transition-all font-body placeholder:text-slate-400"
            }
          />

          {isLoading ? (
            <Loader2
              className={`${
                isCompact
                  ? "w-3.5 h-3.5 right-2.5 text-[#00ADD8]"
                  : "w-3.5 h-3.5 right-3.5 text-[#00ADD8]"
              } absolute top-1/2 -translate-y-1/2 animate-spin`}
            />
          ) : query ? (
            <button
              type="button"
              onClick={() => {
                setQuery("");
                setSuggestions([]);
                setIsOpen(false);
                inputRef.current?.focus();
                if (isCompact) {
                  router.push(buildUrl(""));
                }
              }}
              className={`${
                isCompact
                  ? "p-1 text-slate-400 hover:text-slate-600 rounded-full hover:bg-slate-100 right-1.5"
                  : "p-1 text-slate-400 hover:text-slate-600 rounded-full hover:bg-slate-200/60 right-2.5"
              } absolute top-1/2 -translate-y-1/2 cursor-pointer transition-colors`}
              aria-label="Clear search query"
            >
              <X className={isCompact ? "w-3 h-3" : "w-3.5 h-3.5"} />
            </button>
          ) : null}
        </div>
      </form>

      {/* Suggestions Dropdown */}
      {isOpen && query.trim().length > 0 && (
        <div
          className={`absolute ${
            isCompact
              ? "left-0 top-full mt-1.5 w-full"
              : "left-0 right-0 top-full mt-1"
          } bg-white rounded-md shadow-2xl border border-slate-200/90 overflow-hidden z-50 animate-in fade-in-50 zoom-in-95`}
        >
          {suggestions.length > 0 ? (
            <div>
              <div className="px-3 py-1.5 bg-slate-50 border-b border-slate-100 flex items-center justify-between text-[11px] font-semibold text-slate-500 font-heading">
                <span>{suggestions.length} found</span>
              </div>

              <div className="max-h-80 overflow-y-auto divide-y divide-slate-100">
                {suggestions.map((item) => (
                  <button
                    key={item.uuid}
                    type="button"
                    onClick={() => handleSelectProduct(item.slug)}
                    className="w-full px-3.5 py-2 flex items-center gap-3 hover:bg-[#E6F7FB] text-left cursor-pointer transition-colors group"
                  >
                    <div className="relative w-9 h-9 rounded-md bg-slate-100 overflow-hidden border border-slate-200/80 shrink-0">
                      {item.image_url ? (
                        <Image
                          src={getImageUrl(item.image_url)}
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
                onClick={() => handleSubmit()}
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
          ) : (
            <div className="py-5 flex items-center justify-center gap-2 text-xs text-slate-500">
              <Loader2 className="w-3.5 h-3.5 animate-spin text-[#007D9C]" />
              <span>Searching products...</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
