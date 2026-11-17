"use client";

import React, { useState, useEffect, useRef } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Search, X, Loader2, Tag, ArrowRight, Sparkles, History } from "lucide-react";
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
  /**
   * Keywords the server rendered for this visitor. Rendered on first paint so
   * the dropdown is populated before any client fetch.
   */
  recentSearches?: string[];
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
  recentSearches = [],
}: SearchAutocompleteProps) {
  const router = useRouter();
  const [query, setQuery] = useState(initialValue);
  const [suggestions, setSuggestions] = useState<ProductSuggestion[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [recents, setRecents] = useState<string[]>(recentSearches);
  // -1 = nothing highlighted. Indexes into the combined row list (recents then
  // product suggestions) so arrow keys walk both blocks as one list.
  const [activeIndex, setActiveIndex] = useState(-1);
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Sync state if initialValue prop changes
  useEffect(() => {
    setQuery(initialValue);
  }, [initialValue]);

  useEffect(() => {
    setActiveIndex(-1);
  }, [query, isOpen]);

  // The server rendered the list for the first paint; keep it in sync when the
  // parent re-renders with fresher data (e.g. after a refresh).
  useEffect(() => {
    setRecents(recentSearches);
  }, [recentSearches]);

  const visibleRecents = query.trim() ? [] : recents;
  const rowCount = visibleRecents.length + (query.trim() ? suggestions.length : 0);

  const addRecent = (keyword: string) => {
    const q = keyword.trim();
    if (q.length < 2) return;
    setRecents((prev) =>
      [q, ...prev.filter((item) => item.toLowerCase() !== q.toLowerCase())].slice(0, 10)
    );
  };

  const removeRecent = async (keyword: string) => {
    setRecents((prev) => prev.filter((item) => item !== keyword));
    try {
      await fetch(
        `${API_BASE_URL}/api/v1/search/history?query=${encodeURIComponent(keyword)}`,
        {
          method: "DELETE",
          credentials: "include",
          headers: API_KEY ? { "X-API-KEY": API_KEY } : undefined,
        }
      );
    } catch {
      // The local list is already correct; the server catches up on next load.
    }
  };

  const clearRecents = async () => {
    setRecents([]);
    try {
      await fetch(`${API_BASE_URL}/api/v1/search/history`, {
        method: "DELETE",
        credentials: "include",
        headers: API_KEY ? { "X-API-KEY": API_KEY } : undefined,
      });
    } catch {
      // Same as above: optimistic UI, the server catches up on next load.
    }
  };

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
                (hit: { id?: string; product_uuid?: string; product_id?: number | string; [key: string]: unknown }) => ({
                  uuid: hit.id || hit.product_uuid || String(hit.product_id),
                  name: hit.product_name as string,
                  slug: hit.product_slug as string,
                  price: hit.product_price as number,
                  image_url: hit.product_image_url as string,
                  category_name: hit.category_name as string,
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
    // Optimistic: the page records the authoritative "search" event once it
    // knows the result count, so a query that finds nothing never lands here.
    addRecent(query);
    router.push(buildUrl(query));
  };

  const handleQuickSearch = (keyword: string) => {
    setQuery(keyword);
    setIsOpen(false);
    if (onClose) onClose();
    addRecent(keyword);
    router.push(buildUrl(keyword));
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      setIsOpen(false);
      return;
    }
    if (!isOpen || rowCount === 0) return;

    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => (i + 1) % rowCount);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => (i <= 0 ? rowCount - 1 : i - 1));
      return;
    }
    if (e.key === "Enter" && activeIndex >= 0) {
      e.preventDefault();
      if (visibleRecents.length > 0) {
        // Recents occupy the first block when the input is empty.
        handleQuickSearch(visibleRecents[activeIndex]);
      } else {
        const picked = suggestions[activeIndex];
        if (picked) handleSelectProduct(picked.slug);
      }
    }
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
              if (query.trim().length > 0 || visibleRecents.length > 0) setIsOpen(true);
            }}
            onKeyDown={handleKeyDown}
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

      {/* Suggestions Dropdown: recents when idle, product hits while typing */}
      {isOpen && (query.trim().length > 0 || visibleRecents.length > 0) && (
        <div
          className={`absolute ${
            isCompact
              ? "left-0 top-full mt-1.5 w-full"
              : "left-0 right-0 top-full mt-1"
          } bg-white rounded-md shadow-2xl border border-slate-200/90 overflow-hidden z-50 animate-in fade-in-50 zoom-in-95`}
        >
          {visibleRecents.length > 0 && (
            <div className="border-b border-slate-100">
              <div className="px-3 py-1.5 bg-slate-50 border-b border-slate-100 flex items-center justify-between text-[11px] font-semibold text-slate-500 font-heading">
                <span>Recent searches</span>
                <button
                  type="button"
                  onClick={clearRecents}
                  className="text-[11px] font-medium text-slate-400 hover:text-[#007D9C] underline underline-offset-2 cursor-pointer"
                >
                  Clear all
                </button>
              </div>
              <ul className="max-h-64 overflow-y-auto">
                {visibleRecents.map((keyword, index) => (
                  <li
                    key={keyword}
                    className={`flex items-center group ${
                      activeIndex === index ? "bg-[#E6F7FB]" : ""
                    }`}
                    onMouseEnter={() => setActiveIndex(index)}
                  >
                    <button
                      type="button"
                      onClick={() => handleQuickSearch(keyword)}
                      className="flex-1 min-w-0 px-3.5 py-2 flex items-center gap-2.5 text-left text-xs text-slate-700 hover:text-[#007D9C] cursor-pointer transition-colors"
                    >
                      <History className="w-3.5 h-3.5 text-slate-400 shrink-0" />
                      <span className="truncate font-body">{keyword}</span>
                    </button>
                    {/* Sibling button, never nested: a button inside a button is invalid HTML. */}
                    <button
                      type="button"
                      aria-label={`Remove ${keyword} from search history`}
                      onClick={(e) => {
                        e.stopPropagation();
                        void removeRecent(keyword);
                      }}
                      className="p-1.5 mr-2 rounded text-slate-300 hover:text-slate-700 hover:bg-slate-100 opacity-0 group-hover:opacity-100 focus:opacity-100 transition-opacity cursor-pointer"
                    >
                      <X className="w-3 h-3" />
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {query.trim().length > 0 && suggestions.length > 0 ? (
            <div>
              <div className="px-3 py-1.5 bg-slate-50 border-b border-slate-100 flex items-center justify-between text-[11px] font-semibold text-slate-500 font-heading">
                <span>{suggestions.length} found</span>
              </div>

              <div className="max-h-80 overflow-y-auto divide-y divide-slate-100">
                {suggestions.map((item, index) => (
                  <button
                    key={item.uuid}
                    type="button"
                    onMouseEnter={() => setActiveIndex(index)}
                    onClick={() => handleSelectProduct(item.slug)}
                    className={`w-full px-3.5 py-2 flex items-center gap-3 text-left cursor-pointer transition-colors group ${
                      activeIndex === index ? "bg-[#E6F7FB]" : "hover:bg-[#E6F7FB]"
                    }`}
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
          ) : query.trim().length === 0 ? null : !isLoading ? (
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
