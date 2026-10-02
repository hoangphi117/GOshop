"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import Link from "next/link";
import {
  MessageSquare,
  X,
  Send,
  User,
  Check,
  CheckCheck,
  Paperclip,
  ShoppingBag,
  Tag,
  Search,
  Loader2,
  Package,
  ChevronRight,
  ChevronLeft,
  Download,
  ZoomIn,
  ClipboardMinus,
  Undo2,
  MoreHorizontal,
  Reply,
  Copy,
} from "lucide-react";
import {
  useChat,
  uploadChatFile,
  useRecentOrders,
  useChatProducts,
} from "@/hooks/use-chat";
import type { OrderRef, ProductRef } from "@/hooks/use-chat";
import { useChatUIStore } from "@/stores/chat-ui-store";
import { requestNotificationPermission } from "@/lib/chat-notifications";

// ─── Helpers ─────────────────────────────────────────────────────────────────

function fmtCurrency(n: number) {
  return new Intl.NumberFormat("vi-VN", {
    style: "currency",
    currency: "VND",
  }).format(n);
}

function fmtDate(isoString: string) {
  try {
    const d = new Date(isoString);
    return d.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  } catch {
    return "";
  }
}

function parseReplyContent(content: string): {
  replyQuote: string | null;
  body: string;
} {
  if (content && content.startsWith("> ")) {
    const doubleNewlineIdx = content.indexOf("\n\n");
    if (doubleNewlineIdx !== -1) {
      return {
        replyQuote: content.slice(2, doubleNewlineIdx).trim(),
        body: content.slice(doubleNewlineIdx + 2).trim(),
      };
    }
    const singleNewlineIdx = content.indexOf("\n");
    if (singleNewlineIdx !== -1) {
      return {
        replyQuote: content.slice(2, singleNewlineIdx).trim(),
        body: content.slice(singleNewlineIdx + 1).trim(),
      };
    }
  }
  return { replyQuote: null, body: content };
}

const ORDER_STATUS_LABEL: Record<string, string> = {
  pending: "Pending",
  confirmed: "Confirmed",
  processing: "Processing",
  shipping: "Shipping",
  shipped: "Shipped",
  delivered: "Delivered",
  completed: "Completed",
  cancelled: "Cancelled",
  returned: "Returned",
};

// ─── Sub-components ──────────────────────────────────────────────────────────

function OrderStatusBadge({
  status,
  isDark = false,
}: {
  status: string;
  isDark?: boolean;
}) {
  const normalized = status?.toLowerCase() || "";
  const label =
    ORDER_STATUS_LABEL[normalized] ||
    (status ? status.charAt(0).toUpperCase() + status.slice(1) : "Unknown");

  return (
    <span
      className={`text-[10.5px] font-medium shrink-0 ${
        isDark ? "text-slate-300" : "text-slate-500"
      }`}
    >
      {label}
    </span>
  );
}

function resolveMediaUrl(url?: string) {
  if (!url) return "";
  if (
    url.startsWith("http://") ||
    url.startsWith("https://") ||
    url.startsWith("blob:") ||
    url.startsWith("data:")
  ) {
    return url;
  }
  const baseUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  return `${baseUrl.replace(/\/$/, "")}${url.startsWith("/") ? "" : "/"}${url}`;
}

interface MediaItem {
  url: string;
  name?: string;
  type?: string;
}

function parseMediaItems(
  contentType: string,
  attachmentUrl?: string,
  attachmentName?: string,
): MediaItem[] {
  if (!attachmentUrl) return [];
  if (contentType === "media_group") {
    try {
      const parsed = JSON.parse(attachmentUrl);
      if (Array.isArray(parsed)) {
        return parsed.map((it) => ({
          url: resolveMediaUrl(it.url),
          name: it.name,
          type: it.type || "image",
        }));
      }
    } catch {
      // fallback
    }
  }
  return [
    {
      url: resolveMediaUrl(attachmentUrl),
      name: attachmentName,
      type: contentType === "video" ? "video" : "image",
    },
  ];
}

/** Messenger-style cluster grid for 1, 2, 3, 4, 5+ media items */
function MediaGroupGrid({
  items,
  onImageClick,
}: {
  items: MediaItem[];
  onImageClick?: (items: MediaItem[], index: number) => void;
}) {
  if (items.length === 0) return null;

  if (items.length === 1) {
    const it = items[0];
    if (it.type === "video") {
      return (
        // eslint-disable-next-line jsx-a11y/media-has-caption
        <video
          src={it.url}
          controls
          className="max-h-56 max-w-full rounded-xl shadow-xs"
        />
      );
    }
    return (
      <button
        type="button"
        onClick={() => onImageClick?.(items, 0)}
        className="group relative block overflow-hidden rounded-xl cursor-zoom-in text-left focus:outline-hidden"
      >
        <img
          src={it.url}
          alt={it.name || "Shared image"}
          className="max-h-56 max-w-full rounded-xl object-cover shadow-xs transition-transform duration-200 group-hover:scale-[1.02]"
        />
        <div className="absolute inset-0 bg-black/0 group-hover:bg-black/20 transition-colors flex items-center justify-center">
          <div className="opacity-0 group-hover:opacity-100 transition-opacity bg-black/60 text-white rounded-full p-1.5 backdrop-blur-xs shadow-md">
            <ZoomIn className="w-4 h-4" />
          </div>
        </div>
      </button>
    );
  }

  const displayItems = items.slice(0, 4);
  const remainingCount = items.length - 4;

  if (items.length === 2) {
    return (
      <div className="grid grid-cols-2 gap-1 w-[240px] sm:w-[270px] rounded-xl overflow-hidden shadow-2xs">
        {displayItems.map((it, idx) => (
          <button
            key={idx}
            type="button"
            onClick={() => onImageClick?.(items, idx)}
            className="group relative aspect-square bg-black/5 overflow-hidden cursor-zoom-in"
          >
            {it.type === "video" ? (
              <video src={it.url} className="w-full h-full object-cover" />
            ) : (
              <img
                src={it.url}
                alt={it.name || "Attachment"}
                className="w-full h-full object-cover transition-transform duration-200 group-hover:scale-105"
              />
            )}
          </button>
        ))}
      </div>
    );
  }

  if (items.length === 3) {
    return (
      <div className="grid grid-cols-2 gap-1 w-[240px] sm:w-[270px] rounded-xl overflow-hidden shadow-2xs">
        <button
          type="button"
          onClick={() => onImageClick?.(items, 0)}
          className="group relative col-span-2 h-28 bg-black/5 overflow-hidden cursor-zoom-in"
        >
          {items[0].type === "video" ? (
            <video src={items[0].url} className="w-full h-full object-cover" />
          ) : (
            <img
              src={items[0].url}
              alt={items[0].name || "Attachment"}
              className="w-full h-full object-cover transition-transform duration-200 group-hover:scale-105"
            />
          )}
        </button>
        {items.slice(1).map((it, idx) => (
          <button
            key={idx + 1}
            type="button"
            onClick={() => onImageClick?.(items, idx + 1)}
            className="group relative aspect-square bg-black/5 overflow-hidden cursor-zoom-in"
          >
            {it.type === "video" ? (
              <video src={it.url} className="w-full h-full object-cover" />
            ) : (
              <img
                src={it.url}
                alt={it.name || "Attachment"}
                className="w-full h-full object-cover transition-transform duration-200 group-hover:scale-105"
              />
            )}
          </button>
        ))}
      </div>
    );
  }

  return (
    <div className="grid grid-cols-2 gap-1 w-[240px] sm:w-[270px] rounded-xl overflow-hidden shadow-2xs">
      {displayItems.map((it, idx) => {
        const isLastAndMore = idx === 3 && remainingCount > 0;
        return (
          <button
            key={idx}
            type="button"
            onClick={() => onImageClick?.(items, idx)}
            className="group relative aspect-square bg-black/5 overflow-hidden cursor-zoom-in"
          >
            {it.type === "video" ? (
              <video src={it.url} className="w-full h-full object-cover" />
            ) : (
              <img
                src={it.url}
                alt={it.name || "Attachment"}
                className="w-full h-full object-cover transition-transform duration-200 group-hover:scale-105"
              />
            )}
            {isLastAndMore && (
              <div className="absolute inset-0 bg-black/65 backdrop-blur-[1px] flex items-center justify-center text-white text-base font-extrabold tracking-wide">
                +{remainingCount + 1}
              </div>
            )}
          </button>
        );
      })}
    </div>
  );
}

/** Renders the body of a single message bubble based on content_type */
function MessageContent({
  content,
  content_type,
  attachment_url,
  attachment_name,
  isUser = false,
  onImageClick,
}: {
  content: string;
  content_type: string;
  attachment_url?: string;
  attachment_name?: string;
  isUser?: boolean;
  onImageClick?: (items: MediaItem[], index: number) => void;
}) {
  if (content_type === "recalled") {
    return (
      <div className="flex items-center gap-1.5 py-0.5 text-xs italic opacity-75 select-none">
        <Undo2 className="w-3.5 h-3.5 shrink-0" />
        <span>This message was unsent</span>
      </div>
    );
  }
  if (
    content_type === "image" ||
    content_type === "video" ||
    content_type === "media_group"
  ) {
    const items = parseMediaItems(
      content_type,
      attachment_url,
      attachment_name,
    );
    const isSpecialTag =
      content === "[Images]" ||
      content === "[Image]" ||
      content === "[Video]" ||
      /^\d+\s+(attachments|photos|files)$/i.test(content.trim());

    return (
      <div className="flex flex-col gap-1.5">
        <MediaGroupGrid items={items} onImageClick={onImageClick} />
        {content && !isSpecialTag && (
          <p className="text-xs leading-relaxed px-1 pt-0.5">{content}</p>
        )}
      </div>
    );
  }
  if (content_type === "order_ref") {
    // Packed format: "order_number|status|total|product_name|image_url|item_count"
    const parts = (attachment_name || "").split("|");
    const orderNumber = parts[0] || content.slice(0, 8);
    const status = parts[1] || "";
    const total = parts[2] ? Number(parts[2]) : null;
    const productName = parts[3] || "";
    const imageUrl = parts[4] || "";
    const itemCount = parts[5] ? Number(parts[5]) : null;
    const orderUuid = content;

    return (
      <Link
        href={`/account/orders?order=${orderUuid}`}
        className="group flex flex-col gap-1.5 min-w-[200px] max-w-[250px] p-0.5 cursor-pointer text-inherit hover:opacity-95"
      >
        {/* Order Header: small order number code + status badge + navigation arrow */}
        <div className="flex items-center justify-between gap-2 border-b border-current/10 pb-1.5">
          <div className="flex items-center gap-1 text-[10.5px] font-mono opacity-75 truncate">
            <ShoppingBag className="h-3 w-3 shrink-0" />
            <span className="truncate">#{orderNumber}</span>
          </div>
          <div className="flex items-center gap-1">
            {status && <OrderStatusBadge status={status} isDark={isUser} />}
            <ChevronRight className="w-3 h-3 opacity-60 group-hover:opacity-100 group-hover:translate-x-0.5 transition-all" />
          </div>
        </div>

        {/* Product preview if present */}
        {(productName || imageUrl) && (
          <div className="flex items-center gap-2 py-0.5">
            {imageUrl ? (
              <img
                src={imageUrl}
                alt={productName || "product"}
                className="w-10 h-10 rounded-md object-cover shrink-0 bg-black/5 border border-current/10"
              />
            ) : (
              <div className="w-10 h-10 rounded-md bg-black/5 border border-current/10 flex items-center justify-center shrink-0 opacity-60">
                <Package className="w-5 h-5 stroke-[1.5]" />
              </div>
            )}
            <div className="flex-1 min-w-0">
              <p
                className="text-xs font-medium truncate leading-tight group-hover:underline"
                title={productName}
              >
                {productName}
              </p>
              {itemCount && itemCount > 1 && (
                <p className="text-[10px] opacity-70 mt-0.5">
                  +{itemCount - 1} other{" "}
                  {itemCount - 1 === 1 ? "item" : "items"}
                </p>
              )}
            </div>
          </div>
        )}

        {/* Total amount and item count */}
        {total !== null && (
          <div className="flex items-center justify-between text-[11px] opacity-90 pt-1 border-t border-current/10">
            <span className="opacity-75">
              {itemCount
                ? `${itemCount} ${itemCount === 1 ? "item" : "items"}`
                : "Total:"}
            </span>
            <span className="font-semibold">{fmtCurrency(total)}</span>
          </div>
        )}
      </Link>
    );
  }
  if (content_type === "product_ref") {
    // Packed format: "product_name|price|stock|slug|category_name|stock_status"
    const parts = (attachment_name || "").split("|");
    const productName = parts[0] || content;
    const price = parts[1] ? Number(parts[1]) : null;
    const stock = parts[2] ? Number(parts[2]) : null;
    const slug = parts[3] || content;
    const isOutOfStock =
      parts[5] === "out_of_stock" || (stock !== null && stock <= 0);
    const imageUrl = attachment_url || "";

    return (
      <Link
        href={`/products/${slug}`}
        className="group flex flex-col gap-1.5 min-w-[200px] max-w-[250px] p-0.5 cursor-pointer text-inherit hover:opacity-95"
      >
        {/* Header: Tag + Product Inquiry */}
        <div className="flex items-center justify-between gap-2 border-b border-current/10 pb-1.5">
          <div className="flex items-center gap-1 text-[10.5px] font-medium opacity-75 truncate">
            <span>Product </span>
          </div>
          <ChevronRight className="w-3 h-3 opacity-60 group-hover:opacity-100 group-hover:translate-x-0.5 transition-all" />
        </div>

        {/* Product preview */}
        <div className="flex items-center gap-2 py-0.5">
          <div className="relative w-11 h-11 rounded-md overflow-hidden shrink-0 bg-black/5 border border-current/10">
            {imageUrl ? (
              <img
                src={imageUrl}
                alt={productName}
                className="w-full h-full object-cover"
              />
            ) : (
              <div className="w-full h-full flex items-center justify-center opacity-60">
                <Package className="w-5 h-5 stroke-[1.5]" />
              </div>
            )}
            {isOutOfStock && (
              <div className="absolute inset-0 bg-black/65 backdrop-blur-[0.5px] flex items-center justify-center p-0.5 text-center">
                <span className="text-[7px] font-bold uppercase tracking-wider text-white leading-tight">
                  Out of stock
                </span>
              </div>
            )}
          </div>

          <div className="flex-1 min-w-0">
            <p
              className="text-xs font-semibold truncate leading-tight group-hover:underline"
              title={productName}
            >
              {productName}
            </p>
            {price !== null && (
              <p className="text-xs font-bold mt-0.5">{fmtCurrency(price)}</p>
            )}
            <div className="flex items-center gap-1.5 mt-0.5 text-[10px] opacity-70">
              {isOutOfStock ? (
                <span className="text-red-500 font-medium">Out of stock</span>
              ) : (
                <span>In stock {stock !== null ? `(${stock})` : ""}</span>
              )}
            </div>
          </div>
        </div>
      </Link>
    );
  }
  return <>{content}</>;
}

/** Fullscreen Gallery Lightbox Modal with Next / Prev Navigation and Thumbnails */
function MediaLightbox({
  mediaGroup,
  onClose,
}: {
  mediaGroup: { items: MediaItem[]; initialIndex: number } | null;
  onClose: () => void;
}) {
  const [currentIndex, setCurrentIndex] = useState(0);

  useEffect(() => {
    if (mediaGroup) {
      setCurrentIndex(mediaGroup.initialIndex || 0);
    }
  }, [mediaGroup]);

  const items = mediaGroup?.items || [];
  const currentItem = items[currentIndex];

  const handlePrev = useCallback(() => {
    if (items.length <= 1) return;
    setCurrentIndex((prev) => (prev - 1 + items.length) % items.length);
  }, [items.length]);

  const handleNext = useCallback(() => {
    if (items.length <= 1) return;
    setCurrentIndex((prev) => (prev + 1) % items.length);
  }, [items.length]);

  useEffect(() => {
    if (!mediaGroup) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      if (e.key === "ArrowLeft") handlePrev();
      if (e.key === "ArrowRight") handleNext();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [mediaGroup, onClose, handlePrev, handleNext]);

  if (!mediaGroup || !currentItem) return null;

  return (
    <div
      className="fixed inset-0 z-[100] bg-slate-950/92 backdrop-blur-sm flex flex-col items-center justify-between p-4 sm:p-6 animate-in fade-in duration-150 select-none"
      onClick={onClose}
    >
      {/* Top action toolbar */}
      <div
        className="w-full flex items-center justify-between text-white z-20 shrink-0"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2">
          {items.length > 1 && (
            <span className="px-2.5 py-1 rounded-full bg-white/15 text-xs font-mono font-medium backdrop-blur-xs">
              {currentIndex + 1} / {items.length}
            </span>
          )}
          {currentItem.name && (
            <span className="text-xs text-slate-300 truncate max-w-[180px] sm:max-w-md">
              {currentItem.name}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <a
            href={currentItem.url}
            target="_blank"
            rel="noopener noreferrer"
            download={currentItem.name || "media"}
            className="p-2.5 rounded-full bg-white/10 hover:bg-white/20 active:bg-white/30 text-white transition-colors cursor-pointer shadow-md"
            title="Download / Open original"
          >
            <Download className="w-4 h-4" />
          </a>
          <button
            type="button"
            onClick={onClose}
            className="p-2.5 rounded-full bg-white/10 hover:bg-white/20 active:bg-white/30 text-white transition-colors cursor-pointer shadow-md"
            title="Close (Esc)"
            aria-label="Close image preview"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Main Preview with Navigation Arrows */}
      <div className="relative w-full flex-1 flex items-center justify-center min-h-0 my-2">
        {/* Left Arrow Button */}
        {items.length > 1 && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              handlePrev();
            }}
            className="absolute left-2 sm:left-6 z-20 p-3 rounded-full bg-black/50 hover:bg-black/80 active:scale-95 text-white transition-all cursor-pointer shadow-xl border border-white/10 backdrop-blur-xs"
            title="Previous (Left Arrow)"
            aria-label="Previous image"
          >
            <ChevronLeft className="w-6 h-6" />
          </button>
        )}

        {/* Media Content */}
        <div
          className="relative max-w-5xl max-h-full flex items-center justify-center cursor-default z-10"
          onClick={(e) => e.stopPropagation()}
        >
          {currentItem.type === "video" ? (
            // eslint-disable-next-line jsx-a11y/media-has-caption
            <video
              key={currentItem.url}
              src={currentItem.url}
              controls
              autoPlay
              className="max-h-[75vh] max-w-[85vw] object-contain rounded-xl shadow-2xl"
            />
          ) : (
            <img
              key={currentItem.url}
              src={currentItem.url}
              alt={currentItem.name || "Preview"}
              className="max-h-[75vh] max-w-[85vw] object-contain rounded-xl shadow-2xl select-none animate-in zoom-in-95 duration-150"
            />
          )}
        </div>

        {/* Right Arrow Button */}
        {items.length > 1 && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              handleNext();
            }}
            className="absolute right-2 sm:right-6 z-20 p-3 rounded-full bg-black/50 hover:bg-black/80 active:scale-95 text-white transition-all cursor-pointer shadow-xl border border-white/10 backdrop-blur-xs"
            title="Next (Right Arrow)"
            aria-label="Next image"
          >
            <ChevronRight className="w-6 h-6" />
          </button>
        )}
      </div>

      {/* Bottom Thumbnail Navigation Strip (if > 1 item) */}
      {items.length > 1 && (
        <div
          className="flex items-center gap-1.5 overflow-x-auto max-w-xl px-2.5 py-1.5 bg-black/45 rounded-xl backdrop-blur-xs z-20 shrink-0"
          onClick={(e) => e.stopPropagation()}
        >
          {items.map((it, idx) => (
            <button
              key={idx}
              type="button"
              onClick={() => setCurrentIndex(idx)}
              className={`relative w-11 h-11 rounded-lg overflow-hidden shrink-0 border-2 transition-all cursor-pointer ${
                idx === currentIndex
                  ? "border-emerald-500 scale-105 opacity-100 shadow-md ring-1 ring-emerald-500/50"
                  : "border-transparent opacity-50 hover:opacity-80"
              }`}
            >
              {it.type === "video" ? (
                <div className="w-full h-full bg-slate-800 flex items-center justify-center text-[9px] text-white font-bold">
                  VID
                </div>
              ) : (
                <img
                  src={it.url}
                  alt="Thumb"
                  className="w-full h-full object-cover"
                />
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/** Modal dialog with fixed frame centered in the chat container and debounced search */
function OrderPickerModal({
  isOpen,
  onSelect,
  onClose,
}: {
  isOpen: boolean;
  onSelect: (o: OrderRef) => void;
  onClose: () => void;
}) {
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchTerm);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  const { data: orders = [], isLoading } = useRecentOrders(
    isOpen,
    debouncedSearch,
  );

  // Client-side fallback filter
  const filteredOrders = orders.filter((o) => {
    if (!searchTerm.trim()) return true;
    const query = searchTerm.toLowerCase().trim();
    const orderNum = o.order_number?.toLowerCase() || "";
    const status = o.status?.toLowerCase() || "";
    const firstItemName = o.items?.[0]?.product_name?.toLowerCase() || "";
    return (
      orderNum.includes(query) ||
      status.includes(query) ||
      firstItemName.includes(query)
    );
  });

  const modalRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  if (!isOpen) return null;

  return (
    <div className="absolute inset-0 z-30 bg-slate-900/50 backdrop-blur-[2px] flex items-center justify-center p-3 animate-in fade-in duration-150">
      <div
        ref={modalRef}
        className="w-full h-[430px] max-h-[430px] bg-white rounded-xl shadow-2xl border border-slate-200 flex flex-col overflow-hidden animate-in zoom-in-95 duration-150"
      >
        {/* Fixed Header */}
        <div className="px-3.5 py-2.5 border-b border-slate-100 flex items-center justify-between bg-slate-50/80 shrink-0">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-slate-900 text-white shadow-xs">
              <ClipboardMinus className="w-3.5 h-3.5" />
            </div>
            <div>
              <h4 className="text-xs font-semibold text-slate-900 leading-tight">
                Select Order
              </h4>
              <p className="text-[10px] text-slate-500 leading-tight">
                Choose an order to share in chat
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 rounded-full transition-colors"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Fixed Search Bar with Debounce */}
        <div className="p-2.5 border-b border-slate-100 bg-white shrink-0">
          <div className="relative flex items-center">
            <Search className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 pointer-events-none" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Search by order code, product or status..."
              className="w-full pl-8 pr-7 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg focus:outline-none focus:border-slate-900 focus:bg-white transition-all placeholder:text-slate-400"
              autoFocus
            />
            {searchTerm && (
              <button
                type="button"
                onClick={() => setSearchTerm("")}
                className="absolute right-2 text-slate-400 hover:text-slate-600 p-0.5"
                aria-label="Clear search"
              >
                <X className="w-3 h-3" />
              </button>
            )}
          </div>
        </div>

        {/* Scrollable Orders List with Fixed Container Height */}
        <div className="flex-1 min-h-0 overflow-y-auto p-2.5 space-y-2">
          {isLoading ? (
            <div className="h-full flex flex-col items-center justify-center text-slate-400 space-y-2 py-8">
              <Loader2 className="w-5 h-5 animate-spin text-slate-500" />
              <p className="text-xs">Loading orders...</p>
            </div>
          ) : filteredOrders.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center p-4 space-y-1.5 text-slate-400 py-8">
              <ShoppingBag className="w-7 h-7 text-slate-300 stroke-[1.5]" />
              <p className="text-xs font-medium text-slate-600">
                {searchTerm ? "No matching orders found" : "No orders found"}
              </p>
              <p className="text-[11px] text-slate-400 max-w-[200px]">
                {searchTerm
                  ? "Try searching with a different order code or keyword."
                  : "You haven't placed any orders yet."}
              </p>
            </div>
          ) : (
            filteredOrders.map((o) => {
              const firstItem = o.items?.[0];
              const totalItems =
                o.items?.reduce((acc, it) => acc + (it.quantity || 1), 0) ??
                o.items?.length ??
                0;

              return (
                <button
                  key={o.uuid}
                  type="button"
                  onClick={() => onSelect(o)}
                  className="w-full group flex items-center gap-2.5 p-2 bg-white hover:bg-slate-50/90 active:bg-slate-100 border border-slate-200/90 hover:border-slate-300 rounded-lg transition-all text-left shadow-2xs cursor-pointer"
                >
                  {/* Product Thumbnail */}
                  {firstItem?.image_url ? (
                    <img
                      src={firstItem.image_url}
                      alt={firstItem.product_name || "product"}
                      className="w-12 h-12 rounded-md object-cover border border-slate-200 shrink-0 bg-slate-100"
                    />
                  ) : (
                    <div className="w-12 h-12 rounded-md bg-slate-100 border border-slate-200 flex items-center justify-center text-slate-400 shrink-0">
                      <Package className="w-5 h-5 stroke-[1.5]" />
                    </div>
                  )}

                  {/* Order & Product Details */}
                  <div className="flex-1 min-w-0 flex flex-col justify-between py-0.5">
                    {/* Top Row: Small Order Code + Status Badge */}
                    <div className="flex items-center justify-between gap-1.5 mb-0.5">
                      <div className="flex items-center gap-1.5 min-w-0">
                        <span className="text-[10px] font-mono text-black truncate">
                          #{o.order_number}
                        </span>
                        {totalItems > 0 && (
                          <span className="text-[9.5px] font-medium text-slate-500 bg-slate-100 px-1.5 py-0.2 rounded shrink-0">
                            {totalItems} {totalItems === 1 ? "item" : "items"}
                          </span>
                        )}
                      </div>
                      <OrderStatusBadge status={o.status} />
                    </div>

                    {/* Middle Row: Truncated First Product Name */}
                    <p
                      className="text-xs font-medium text-slate-800 truncate leading-snug group-hover:text-slate-900"
                      title={
                        firstItem?.product_name || `Order #${o.order_number}`
                      }
                    >
                      {firstItem?.product_name || `Order #${o.order_number}`}
                    </p>

                    {/* Bottom Row: Total Price + Date */}
                    <div className="flex items-center justify-between text-[11px] text-slate-500 mt-0.5">
                      <span className="font-semibold text-slate-800">
                        {fmtCurrency(o.total_amount)}
                      </span>
                      <span className="text-[10px] text-slate-400">
                        {fmtDate(o.created_at)}
                      </span>
                    </div>
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}

/** Modal dialog for selecting a product to consult about */
function ProductPickerModal({
  isOpen,
  onSelect,
  onClose,
}: {
  isOpen: boolean;
  onSelect: (p: ProductRef) => void;
  onClose: () => void;
}) {
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchTerm);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  const { data: products = [], isLoading } = useChatProducts(
    isOpen,
    debouncedSearch,
  );

  const filteredProducts = products.filter((p) => {
    if (!searchTerm.trim()) return true;
    const query = searchTerm.toLowerCase().trim();
    return (
      p.name?.toLowerCase().includes(query) ||
      p.category_name?.toLowerCase().includes(query) ||
      p.slug?.toLowerCase().includes(query)
    );
  });

  const modalRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  if (!isOpen) return null;

  return (
    <div className="absolute inset-0 z-30 bg-slate-900/50 backdrop-blur-[2px] flex items-center justify-center p-3 animate-in fade-in duration-150">
      <div
        ref={modalRef}
        className="w-full h-[430px] max-h-[430px] bg-white rounded-xl shadow-2xl border border-slate-200 flex flex-col overflow-hidden animate-in zoom-in-95 duration-150"
      >
        {/* Fixed Header */}
        <div className="px-3.5 py-2.5 border-b border-slate-100 flex items-center justify-between bg-slate-50/80 shrink-0">
          <div className="flex items-center gap-2">
            <div className="p-1.5 rounded-lg bg-slate-900 text-white shadow-xs">
              <ClipboardMinus className="w-3.5 h-3.5" />
            </div>
            <div>
              <h4 className="text-xs font-semibold text-slate-900 leading-tight">
                Select Product
              </h4>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 text-slate-400 hover:text-slate-700 hover:bg-slate-200/60 rounded-full transition-colors"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Fixed Search Bar with Debounce */}
        <div className="p-2.5 border-b border-slate-100 bg-white shrink-0">
          <div className="relative flex items-center">
            <Search className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 pointer-events-none" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Search products by name or category..."
              className="w-full pl-8 pr-7 py-1.5 text-xs bg-slate-50 border border-slate-200 rounded-lg focus:outline-none focus:border-slate-900 focus:bg-white transition-all placeholder:text-slate-400"
              autoFocus
            />
            {searchTerm && (
              <button
                type="button"
                onClick={() => setSearchTerm("")}
                className="absolute right-2 text-slate-400 hover:text-slate-600 p-0.5"
                aria-label="Clear search"
              >
                <X className="w-3 h-3" />
              </button>
            )}
          </div>
        </div>

        {/* Scrollable Products List */}
        <div className="flex-1 min-h-0 overflow-y-auto p-2.5 space-y-2">
          {isLoading ? (
            <div className="h-full flex flex-col items-center justify-center text-slate-400 space-y-2 py-8">
              <Loader2 className="w-5 h-5 animate-spin text-slate-500" />
              <p className="text-xs">Loading products...</p>
            </div>
          ) : filteredProducts.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center p-4 space-y-1.5 text-slate-400 py-8">
              <Package className="w-7 h-7 text-slate-300 stroke-[1.5]" />
              <p className="text-xs font-medium text-slate-600">
                {searchTerm
                  ? "No matching products found"
                  : "No products available"}
              </p>
              <p className="text-[11px] text-slate-400 max-w-[200px]">
                {searchTerm
                  ? "Try searching with a different keyword."
                  : "Products catalog is currently empty."}
              </p>
            </div>
          ) : (
            filteredProducts.map((p) => {
              const isOutOfStock =
                (p.stock ?? 0) <= 0 ||
                p.status === "out_of_stock" ||
                p.status === "inactive";

              return (
                <button
                  key={p.uuid}
                  type="button"
                  onClick={() => onSelect(p)}
                  className="w-full group flex items-center gap-2.5 p-2 bg-white hover:bg-slate-50/90 active:bg-slate-100 border border-slate-200/90 hover:border-slate-300 rounded-lg transition-all text-left shadow-2xs cursor-pointer"
                >
                  {/* Product Thumbnail with Out of stock overlay */}
                  <div className="relative w-12 h-12 rounded-md overflow-hidden shrink-0 border border-slate-200 bg-slate-100">
                    {p.image_url ? (
                      <img
                        src={p.image_url}
                        alt={p.name}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center text-slate-400">
                        <Package className="w-5 h-5 stroke-[1.5]" />
                      </div>
                    )}
                    {isOutOfStock && (
                      <div className="absolute inset-0 bg-black/60 backdrop-blur-[0.5px] flex items-center justify-center p-0.5 text-center">
                        <span className="text-[8px] font-bold uppercase tracking-wider text-white leading-tight">
                          Out of stock
                        </span>
                      </div>
                    )}
                  </div>

                  {/* Product Details: Name, Category/Stock, Price */}
                  <div className="flex-1 min-w-0 flex flex-col justify-between py-0.5">
                    <p
                      className="text-xs font-semibold text-slate-800 truncate leading-snug group-hover:text-slate-900"
                      title={p.name}
                    >
                      {p.name}
                    </p>

                    <div className="flex items-center justify-between text-[11px] mt-1">
                      <span className="font-bold text-slate-900">
                        {fmtCurrency(p.price)}
                      </span>
                      {isOutOfStock ? (
                        <span className="text-[9.5px] font-medium text-red-600 bg-red-50 border border-red-200/80 px-1.5 py-0.2 rounded">
                          Out of stock
                        </span>
                      ) : (
                        <span className="text-[9.5px] font-medium text-emerald-700 bg-emerald-50 border border-emerald-200/80 px-1.5 py-0.2 rounded">
                          In stock ({p.stock})
                        </span>
                      )}
                    </div>
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
}

// ─── Main Component ───────────────────────────────────────────────────────────

export function ChatWidget() {
  const {
    isOpen,
    setIsOpen,
    pendingOrder,
    setPendingOrder,
    pendingProduct,
    setPendingProduct,
  } = useChatUIStore();
  const [inputValue, setInputValue] = useState("");
  const [replyingTo, setReplyingTo] = useState<{
    uuid: string;
    sender: string;
    content: string;
  } | null>(null);
  const [activeMenuMsgUuid, setActiveMenuMsgUuid] = useState<string | null>(
    null,
  );
  const [pendingFiles, setPendingFiles] = useState<File[]>([]);
  const [isUploading, setIsUploading] = useState(false);
  const [showOrderPicker, setShowOrderPicker] = useState(false);
  const [showProductPicker, setShowProductPicker] = useState(false);
  const [previewMediaGroup, setPreviewMediaGroup] = useState<{
    items: MediaItem[];
    initialIndex: number;
  } | null>(null);

  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const typingTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const {
    isAuthenticated,
    accessToken,
    conversation,
    messages,
    unreadCount,
    isShopOnline,
    isAdminTyping,
    sendTyping,
    sendMessage,
    isSending,
    recallMessage,
    isRecalling,
    markAsRead,
  } = useChat();

  // Auto scroll to bottom when new messages arrive or when widget opens
  useEffect(() => {
    if (isOpen) {
      messagesEndRef.current?.scrollIntoView({ behavior: "auto" });
      if (unreadCount > 0) markAsRead();
    }
  }, [isOpen, messages, unreadCount, markAsRead]);

  // Scroll to bottom when agent starts typing
  useEffect(() => {
    if (isOpen && isAdminTyping) {
      messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [isOpen, isAdminTyping]);

  // Focus input when chat opens
  useEffect(() => {
    if (isOpen && isAuthenticated) inputRef.current?.focus();
  }, [isOpen, isAuthenticated]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setInputValue(val);
    if (val.trim()) {
      sendTyping(true);
      if (typingTimeoutRef.current) clearTimeout(typingTimeoutRef.current);
      typingTimeoutRef.current = setTimeout(() => sendTyping(false), 2000);
    } else {
      if (typingTimeoutRef.current) clearTimeout(typingTimeoutRef.current);
      sendTyping(false);
    }
  };

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files ? Array.from(e.target.files) : [];
    if (files.length > 0) {
      setPendingFiles((prev) => [...prev, ...files].slice(0, 10));
    }
    e.target.value = "";
  };

  const handleRemoveFile = (index: number) => {
    setPendingFiles((prev) => prev.filter((_, i) => i !== index));
  };

  const handleOrderSelect = useCallback(
    async (order: OrderRef) => {
      setShowOrderPicker(false);
      try {
        const firstItem = order.items?.[0];
        const totalItems =
          order.items?.reduce((acc, it) => acc + (it.quantity || 1), 0) ??
          order.items?.length ??
          0;

        // Pack display info into attachment_name: "order_number|status|total|product_name|image_url|item_count"
        await sendMessage({
          content: order.uuid,
          content_type: "order_ref",
          order_uuid: order.uuid,
          attachment_name: `${order.order_number}|${order.status}|${order.total_amount}|${firstItem?.product_name || ""}|${firstItem?.image_url || ""}|${totalItems}`,
        });
      } catch {
        // silent
      }
    },
    [sendMessage],
  );

  const handleProductSelect = useCallback(
    async (product: ProductRef) => {
      setShowProductPicker(false);
      try {
        const isOutOfStock =
          (product.stock ?? 0) <= 0 ||
          product.status === "out_of_stock" ||
          product.status === "inactive";

        // Pack display info into attachment_name: "product_name|price|stock|slug|category_name|stock_status"
        await sendMessage({
          content: product.uuid || product.name,
          content_type: "product_ref",
          product_uuid: product.uuid,
          attachment_url: product.image_url || "",
          attachment_name: `${product.name}|${product.price}|${product.stock}|${product.slug}|${product.category_name || ""}|${isOutOfStock ? "out_of_stock" : "in_stock"}|${product.uuid || ""}`,
        });
      } catch {
        // silent
      }
    },
    [sendMessage],
  );

  // Auto-send order when opened via "Chat about this order" from order details page
  useEffect(() => {
    if (pendingOrder && isAuthenticated) {
      handleOrderSelect(pendingOrder);
      setPendingOrder(null);
    }
  }, [pendingOrder, isAuthenticated, handleOrderSelect, setPendingOrder]);

  // Auto-send product when opened via "Chat for consultation" from product detail page
  useEffect(() => {
    if (pendingProduct && isAuthenticated) {
      handleProductSelect(pendingProduct);
      setPendingProduct(null);
    }
  }, [pendingProduct, isAuthenticated, handleProductSelect, setPendingProduct]);

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    const content = inputValue.trim();
    if ((!content && pendingFiles.length === 0) || isSending || isUploading)
      return;

    if (typingTimeoutRef.current) clearTimeout(typingTimeoutRef.current);
    sendTyping(false);
    setInputValue("");
    const filesToSend = [...pendingFiles];
    setPendingFiles([]);

    try {
      if (filesToSend.length > 0) {
        setIsUploading(true);
        const uploadResults = await Promise.all(
          filesToSend.map((file) => uploadChatFile(file, accessToken)),
        );
        setIsUploading(false);

        if (filesToSend.length === 1) {
          const file = filesToSend[0];
          const uploaded = uploadResults[0];
          const isVideo = file.type.startsWith("video/");
          await sendMessage({
            content: content || uploaded.name || file.name,
            content_type: isVideo ? "video" : "image",
            attachment_url: uploaded.url,
            attachment_name: uploaded.name,
            attachment_size: uploaded.size,
          });
        } else {
          // Send all attachments grouped in 1 message (like Messenger)
          const mediaItems = uploadResults.map((uploaded, idx) => ({
            url: uploaded.url,
            name: uploaded.name || filesToSend[idx].name,
            type: filesToSend[idx].type.startsWith("video/")
              ? "video"
              : "image",
          }));

          await sendMessage({
            content: content || `${filesToSend.length} attachments`,
            content_type: "media_group",
            attachment_url: JSON.stringify(mediaItems),
            attachment_name: `${filesToSend.length} attachments`,
          });
        }
      } else if (content) {
        let textToSend = content;
        if (replyingTo) {
          const quoteSnippet =
            replyingTo.content.length > 80
              ? replyingTo.content.slice(0, 80) + "..."
              : replyingTo.content;
          textToSend = `> ${quoteSnippet}\n\n${content}`;
        }
        await sendMessage({ content: textToSend });
        setReplyingTo(null);
      }
    } catch {
      setInputValue(content);
      setPendingFiles(filesToSend);
      setIsUploading(false);
    }
  };

  const formatTime = (isoString: string) => {
    try {
      return new Date(isoString).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return "";
    }
  };

  const formatDividerTime = (isoString: string) => {
    try {
      const d = new Date(isoString);
      const now = new Date();
      const isThisYear = d.getFullYear() === now.getFullYear();
      const isSameDay =
        d.getDate() === now.getDate() &&
        d.getMonth() === now.getMonth() &&
        isThisYear;
      const hh = String(d.getHours()).padStart(2, "0");
      const mm = String(d.getMinutes()).padStart(2, "0");
      const dd = String(d.getDate()).padStart(2, "0");
      const mo = String(d.getMonth() + 1).padStart(2, "0");
      const yy = String(d.getFullYear()).slice(-2);
      if (isSameDay) return `${hh}:${mm}`;
      if (isThisYear) return `${hh}:${mm} ${dd}/${mo}`;
      return `${hh}:${mm} ${dd}/${mo}/${yy}`;
    } catch {
      return "";
    }
  };

  const shouldShowTimeDivider = (curr: string, prev?: string) => {
    if (!prev) return true;
    try {
      return (
        (new Date(curr).getTime() - new Date(prev).getTime()) / 3600000 >= 1
      );
    } catch {
      return false;
    }
  };

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="fixed bottom-6 right-6 z-50 flex flex-col items-end">
      {isOpen && (
        <div className="relative mb-3 flex flex-col w-[360px] sm:w-[390px] h-[520px] bg-white border border-slate-200 rounded-lg shadow-xl overflow-hidden text-slate-900">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 bg-slate-900 text-white">
            <div className="flex items-center gap-2.5">
              {isShopOnline ? (
                <div className="relative flex items-center justify-center w-2.5 h-2.5">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
                  <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-400" />
                </div>
              ) : (
                <div className="flex items-center justify-center w-2.5 h-2.5">
                  <span className="inline-flex h-2 w-2 rounded-full bg-amber-400/80" />
                </div>
              )}
              <div>
                <h3 className="text-sm font-semibold leading-tight">
                  Customer Support
                </h3>
                <p className="text-xs text-slate-400">
                  {isShopOnline ? "Active now" : "Replies in a few minutes"}
                </p>
              </div>
            </div>
            <button
              onClick={() => setIsOpen(false)}
              className="p-1 text-slate-400 hover:text-white rounded transition-colors"
              aria-label="Close chat"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* Body */}
          <div className="flex-1 p-4 overflow-y-auto bg-slate-50 space-y-3">
            {!isAuthenticated ? (
              <div className="h-full flex flex-col items-center justify-center text-center p-6 space-y-3">
                <div className="w-12 h-12 rounded-full bg-slate-200 flex items-center justify-center text-slate-600">
                  <User className="w-6 h-6" />
                </div>
                <div className="space-y-1">
                  <h4 className="text-sm font-medium text-slate-800">
                    Sign in to chat
                  </h4>
                  <p className="text-xs text-slate-500">
                    Please log in to contact support and view your order
                    conversations.
                  </p>
                </div>
                <Link
                  href="/login"
                  className="inline-flex items-center justify-center px-4 py-2 text-xs font-medium text-white bg-slate-900 rounded hover:bg-slate-800 transition-colors"
                  onClick={() => setIsOpen(false)}
                >
                  Log In
                </Link>
              </div>
            ) : messages.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-center p-6 space-y-2 text-slate-500">
                <MessageSquare className="w-8 h-8 text-slate-400" />
                <p className="text-sm font-medium text-slate-700">
                  How can we help you today?
                </p>
                <p className="text-xs">
                  Send a message below and an agent will assist you shortly.
                </p>
              </div>
            ) : (
              <>
                {messages.map((msg, index) => {
                  const prevMsg = index > 0 ? messages[index - 1] : undefined;
                  const showDivider = shouldShowTimeDivider(
                    msg.created_at,
                    prevMsg?.created_at,
                  );
                  const isUser = msg.sender_role === "user";
                  const isRecalled = msg.content_type === "recalled";
                  const canRecall =
                    isUser &&
                    !isRecalled &&
                    Date.now() - new Date(msg.created_at).getTime() <=
                      60 * 60 * 1000;
                  return (
                    <div key={msg.uuid} className="flex flex-col">
                      {showDivider && (
                        <div className="flex items-center justify-center my-2">
                          <span className="rounded-full bg-slate-200/80 border border-slate-300/60 px-2.5 py-0.5 text-[11px] font-medium text-slate-600 shadow-xs">
                            {formatDividerTime(msg.created_at)}
                          </span>
                        </div>
                      )}
                      <div
                        className={`flex flex-col w-full ${isUser ? "items-end" : "items-start"}`}
                      >
                        {(() => {
                          const isMedia =
                            msg.content_type === "image" ||
                            msg.content_type === "video" ||
                            msg.content_type === "media_group";
                          const isMenuOpen = activeMenuMsgUuid === msg.uuid;
                          const { replyQuote, body: cleanContent } =
                            !isRecalled
                              ? parseReplyContent(msg.content)
                              : { replyQuote: null, body: msg.content };

                          return (
                            <div
                              className={`group/msg relative flex items-center gap-1 ${
                                isUser ? "self-end justify-end" : "self-start justify-start"
                              }`}
                            >
                              {/* Menu Trigger Button for User message */}
                              {isUser && (
                                <div className="relative">
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setActiveMenuMsgUuid(
                                        isMenuOpen ? null : msg.uuid,
                                      )
                                    }
                                    className={`p-1 rounded-full text-slate-400 hover:text-slate-700 hover:bg-slate-200/70 shadow-2xs transition-opacity cursor-pointer ${
                                      isMenuOpen
                                        ? "opacity-100 bg-slate-200/80"
                                        : "opacity-0 group-hover/msg:opacity-100"
                                    }`}
                                    title="More actions"
                                    aria-label="More actions"
                                  >
                                    <MoreHorizontal className="w-3.5 h-3.5" />
                                  </button>

                                  {/* Action Menu Popup */}
                                  {isMenuOpen && (
                                    <>
                                      <div
                                        className="fixed inset-0 z-40"
                                        onClick={() =>
                                          setActiveMenuMsgUuid(null)
                                        }
                                      />
                                      <div className="absolute right-0 bottom-full mb-1 z-50 min-w-32 bg-white rounded-lg shadow-lg border border-slate-200 py-1 text-xs text-slate-700 animate-in fade-in zoom-in-95">
                                        <button
                                          type="button"
                                          onClick={() => {
                                            setReplyingTo({
                                              uuid: msg.uuid,
                                              sender: "You",
                                              content:
                                                msg.content_type === "image"
                                                  ? "[Image]"
                                                  : msg.content_type ===
                                                      "video"
                                                    ? "[Video]"
                                                    : msg.content_type ===
                                                        "order_ref"
                                                      ? "[Order]"
                                                      : msg.content_type ===
                                                          "product_ref"
                                                        ? "[Product]"
                                                        : cleanContent,
                                            });
                                            setActiveMenuMsgUuid(null);
                                            inputRef.current?.focus();
                                          }}
                                          className="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-100 cursor-pointer"
                                        >
                                          <Reply className="w-3.5 h-3.5 text-slate-500" />
                                          <span>Reply</span>
                                        </button>

                                        {!isRecalled && msg.content && (
                                          <button
                                            type="button"
                                            onClick={() => {
                                              navigator.clipboard.writeText(
                                                cleanContent,
                                              );
                                              setActiveMenuMsgUuid(null);
                                            }}
                                            className="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-100 cursor-pointer"
                                          >
                                            <Copy className="w-3.5 h-3.5 text-slate-500" />
                                            <span>Copy text</span>
                                          </button>
                                        )}

                                        {canRecall && (
                                          <button
                                            type="button"
                                            disabled={isRecalling}
                                            onClick={() => {
                                              recallMessage(msg.uuid);
                                              setActiveMenuMsgUuid(null);
                                            }}
                                            className="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-red-50 text-red-600 cursor-pointer border-t border-slate-100"
                                          >
                                            <Undo2 className="w-3.5 h-3.5 text-red-500" />
                                            <span>Recall</span>
                                          </button>
                                        )}
                                      </div>
                                    </>
                                  )}
                                </div>
                              )}

                              <div
                                className={`max-w-[80%] text-sm break-words flex flex-col overflow-hidden shadow-xs ${
                                  isRecalled
                                    ? isUser
                                      ? "px-3 py-1.5 rounded-lg rounded-br-none border border-dashed border-slate-300 bg-slate-100 text-slate-500 text-xs italic shadow-none"
                                      : "px-3 py-1.5 rounded-lg rounded-bl-none border border-dashed border-slate-300 bg-slate-100 text-slate-500 text-xs italic shadow-none"
                                    : isMedia
                                      ? "p-0 bg-transparent border-0 shadow-none overflow-hidden"
                                      : isUser
                                        ? "rounded-lg rounded-br-none bg-slate-900 text-white"
                                        : "rounded-lg rounded-bl-none bg-white text-slate-900 border border-slate-200"
                                }`}
                              >
                                {/* Darker attached top sub-bubble for reply */}
                                {replyQuote && (
                                  <div
                                    className={`flex items-center gap-1.5 px-3 py-1 text-[11px] border-b select-none ${
                                      isUser
                                        ? "bg-black/35 text-slate-200 border-white/15"
                                        : "bg-slate-100 text-slate-600 border-slate-200"
                                    }`}
                                  >
                                    <Reply className="w-3 h-3 shrink-0 opacity-75" />
                                    <span className="truncate italic">{replyQuote}</span>
                                  </div>
                                )}

                                <div className={!isRecalled && !isMedia ? "px-3.5 py-2" : ""}>
                                  <MessageContent
                                    content={cleanContent}
                                    content_type={msg.content_type}
                                    attachment_url={msg.attachment_url}
                                    attachment_name={msg.attachment_name}
                                    isUser={isUser}
                                    onImageClick={(items, idx) =>
                                      setPreviewMediaGroup({
                                        items,
                                        initialIndex: idx,
                                      })
                                    }
                                  />
                                </div>
                              </div>

                              {/* Menu Trigger Button for Admin/Support message */}
                              {!isUser && (
                                <div className="relative">
                                  <button
                                    type="button"
                                    onClick={() =>
                                      setActiveMenuMsgUuid(
                                        isMenuOpen ? null : msg.uuid,
                                      )
                                    }
                                    className={`p-1 rounded-full text-slate-400 hover:text-slate-700 hover:bg-slate-200/70 shadow-2xs transition-opacity cursor-pointer ${
                                      isMenuOpen
                                        ? "opacity-100 bg-slate-200/80"
                                        : "opacity-0 group-hover/msg:opacity-100"
                                    }`}
                                    title="More actions"
                                    aria-label="More actions"
                                  >
                                    <MoreHorizontal className="w-3.5 h-3.5" />
                                  </button>

                                  {/* Action Menu Popup */}
                                  {isMenuOpen && (
                                    <>
                                      <div
                                        className="fixed inset-0 z-40"
                                        onClick={() =>
                                          setActiveMenuMsgUuid(null)
                                        }
                                      />
                                      <div className="absolute left-0 bottom-full mb-1 z-50 min-w-32 bg-white rounded-lg shadow-lg border border-slate-200 py-1 text-xs text-slate-700 animate-in fade-in zoom-in-95">
                                        <button
                                          type="button"
                                          onClick={() => {
                                            setReplyingTo({
                                              uuid: msg.uuid,
                                              sender: "Support",
                                              content:
                                                msg.content_type === "image"
                                                  ? "[Image]"
                                                  : msg.content_type ===
                                                      "video"
                                                    ? "[Video]"
                                                    : msg.content_type ===
                                                        "order_ref"
                                                      ? "[Order]"
                                                      : msg.content_type ===
                                                          "product_ref"
                                                        ? "[Product]"
                                                        : cleanContent,
                                            });
                                            setActiveMenuMsgUuid(null);
                                            inputRef.current?.focus();
                                          }}
                                          className="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-100 cursor-pointer"
                                        >
                                          <Reply className="w-3.5 h-3.5 text-slate-500" />
                                          <span>Reply</span>
                                        </button>

                                        {!isRecalled && msg.content && (
                                          <button
                                            type="button"
                                            onClick={() => {
                                              navigator.clipboard.writeText(
                                                cleanContent,
                                              );
                                              setActiveMenuMsgUuid(null);
                                            }}
                                            className="w-full px-3 py-1.5 text-left flex items-center gap-2 hover:bg-slate-100 cursor-pointer"
                                          >
                                            <Copy className="w-3.5 h-3.5 text-slate-500" />
                                            <span>Copy text</span>
                                          </button>
                                        )}
                                      </div>
                                    </>
                                  )}
                                </div>
                              )}
                            </div>
                          );
                        })()}
                        <div
                          className={`flex items-center gap-1 text-[10px] text-slate-400 mt-1 px-1 ${
                            isUser ? "justify-end" : "justify-start"
                          }`}
                        >
                          <span>{formatTime(msg.created_at)}</span>
                          {isUser &&
                            (msg.is_read ? (
                              <CheckCheck className="h-3 w-3 text-emerald-600" />
                            ) : (
                              <Check className="w-3 h-3 text-slate-400" />
                            ))}
                        </div>
                      </div>
                    </div>
                  );
                })}
                {isAdminTyping && (
                  <div className="flex flex-col items-start">
                    <div className="flex items-center gap-1.5 px-3.5 py-2.5 bg-white border border-slate-200 rounded-lg rounded-bl-none shadow-sm">
                      <span className="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce [animation-delay:-0.3s]" />
                      <span className="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce [animation-delay:-0.15s]" />
                      <span className="w-1.5 h-1.5 rounded-full bg-slate-400 animate-bounce" />
                    </div>
                  </div>
                )}
                <div ref={messagesEndRef} />
              </>
            )}
          </div>

          {/* Order Selection Modal (Centered Overlay with fixed frame) */}
          <OrderPickerModal
            isOpen={showOrderPicker}
            onSelect={handleOrderSelect}
            onClose={() => setShowOrderPicker(false)}
          />

          {/* Product Selection Modal (Centered Overlay with fixed frame) */}
          <ProductPickerModal
            isOpen={showProductPicker}
            onSelect={handleProductSelect}
            onClose={() => setShowProductPicker(false)}
          />

          {/* Footer Input */}
          {isAuthenticated && (
            <div className="bg-white border-t border-slate-200">
              {/* Replying banner */}
              {replyingTo && (
                <div className="flex items-center justify-between px-3 py-1.5 bg-slate-100 border-b border-slate-200 text-xs">
                  <div className="flex items-center gap-1.5 min-w-0">
                    <Reply className="w-3.5 h-3.5 text-slate-700 shrink-0" />
                    <div className="min-w-0">
                      <span className="font-semibold text-[11px] text-slate-800 block">
                        Replying to {replyingTo.sender}
                      </span>
                      <span className="text-slate-500 text-[11px] truncate block max-w-[260px]">
                        {replyingTo.content}
                      </span>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => setReplyingTo(null)}
                    className="text-slate-400 hover:text-slate-700 p-0.5 rounded cursor-pointer"
                    aria-label="Cancel reply"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>
              )}

              {/* Pending files preview strip */}
              {pendingFiles.length > 0 && (
                <div className="flex items-center gap-2 p-2.5 pb-1 overflow-x-auto border-b border-slate-100 bg-slate-50/50">
                  {pendingFiles.map((file, idx) => {
                    const isImg = file.type.startsWith("image/");
                    const isVid = file.type.startsWith("video/");
                    return (
                      <div
                        key={idx}
                        className="relative group shrink-0 w-13 h-13 rounded-lg bg-white border border-slate-200 overflow-hidden flex items-center justify-center text-slate-500 shadow-2xs"
                      >
                        {isImg ? (
                          <img
                            src={URL.createObjectURL(file)}
                            alt={file.name}
                            className="w-full h-full object-cover"
                          />
                        ) : isVid ? (
                          <span className="text-[9px] font-bold text-slate-600 bg-slate-100 px-1 py-0.5 rounded">
                            VIDEO
                          </span>
                        ) : (
                          <Paperclip className="w-4 h-4 text-slate-400" />
                        )}
                        <button
                          type="button"
                          onClick={() => handleRemoveFile(idx)}
                          className="absolute top-0.5 right-0.5 p-0.5 rounded-full bg-slate-900/80 text-white hover:bg-slate-900 transition-colors cursor-pointer"
                          title="Remove file"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      </div>
                    );
                  })}
                  <div className="text-[11px] text-slate-400 shrink-0 font-medium px-1">
                    {pendingFiles.length}{" "}
                    {pendingFiles.length === 1 ? "file" : "files"}
                  </div>
                </div>
              )}

              <form onSubmit={handleSend} className="p-3 flex items-center ">
                {/* Hidden file input supporting multiple files */}
                <input
                  ref={fileInputRef}
                  type="file"
                  multiple
                  accept="image/*,video/*"
                  className="hidden"
                  onChange={handleFileSelect}
                />

                {/* File attach button */}
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  disabled={isSending || isUploading}
                  className="p-2 text-slate-400 hover:text-slate-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                  aria-label="Attach file"
                  title="Attach file"
                >
                  <Paperclip className="w-4 h-4" />
                </button>

                {/* Product attach button */}
                <button
                  type="button"
                  onClick={() => {
                    setShowProductPicker((v) => !v);
                    setShowOrderPicker(false);
                  }}
                  disabled={isSending || isUploading}
                  className={`p-2 transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${
                    showProductPicker
                      ? "text-slate-900 bg-slate-100 rounded"
                      : "text-slate-400 hover:text-slate-700"
                  }`}
                  aria-label="Product"
                  title="Product"
                >
                  <ShoppingBag className="w-4 h-4" />
                </button>

                {/* Order attach button */}
                <button
                  type="button"
                  onClick={() => {
                    setShowOrderPicker((v) => !v);
                    setShowProductPicker(false);
                  }}
                  disabled={isSending || isUploading}
                  className={`p-2 transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${
                    showOrderPicker
                      ? "text-slate-900 bg-slate-100 rounded"
                      : "text-slate-400 hover:text-slate-700"
                  }`}
                  aria-label="Order"
                  title="Order"
                >
                  <ClipboardMinus className="w-4 h-4" />
                </button>

                <input
                  ref={inputRef}
                  type="text"
                  value={inputValue}
                  onChange={handleInputChange}
                  placeholder={
                    pendingFiles.length > 0
                      ? `Add a caption for ${pendingFiles.length} ${
                          pendingFiles.length === 1 ? "file" : "files"
                        }...`
                      : "Type your message..."
                  }
                  className="flex-1 text-sm px-3 py-2 border border-slate-300 rounded focus:outline-none focus:border-slate-900"
                  disabled={isSending || isUploading}
                />

                <button
                  type="submit"
                  disabled={
                    (!inputValue.trim() && pendingFiles.length === 0) ||
                    isSending ||
                    isUploading
                  }
                  className="p-2 bg-slate-900 text-white rounded hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                  aria-label="Send message"
                >
                  <Send className="w-4 h-4" />
                </button>
              </form>
            </div>
          )}
        </div>
      )}

      {/* Floating Toggle Button */}
      <button
        onClick={() => {
          const next = !isOpen;
          setIsOpen(next);
          if (next) {
            requestNotificationPermission().catch(() => {});
          }
        }}
        className="relative flex items-center justify-center w-12 h-12 rounded-full bg-slate-900 text-white shadow-lg hover:bg-slate-800 focus:outline-none transition-colors"
        aria-label={isOpen ? "Close chat" : "Open chat"}
      >
        {isOpen ? (
          <X className="w-5 h-5" />
        ) : (
          <MessageSquare className="w-5 h-5" />
        )}
        {!isOpen && unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 flex items-center justify-center min-w-[20px] h-5 px-1 text-[11px] font-bold text-white bg-red-600 rounded-full border-2 border-white">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        )}
      </button>

      {/* Media Lightbox Gallery Modal with Next/Prev Arrows */}
      <MediaLightbox
        mediaGroup={previewMediaGroup}
        onClose={() => setPreviewMediaGroup(null)}
      />
    </div>
  );
}
