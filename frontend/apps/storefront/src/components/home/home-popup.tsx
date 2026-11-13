"use client";

import { useCallback, useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { X } from "lucide-react";
import type { Banner } from "@/lib/api-server";

interface HomePopupProps {
  banners?: Banner[];
}

// sessionStorage key, per popup so a changed promo can still be shown.
const SEEN_PREFIX = "goshop_popup_seen:";

/**
 * Home-page promo modal (position "home_popup").
 *
 * Shown once per browser session: hiding it and reloading must not bring it
 * back, and it must never block the page for a keyboard or screen-reader user,
 * so Escape and the explicit close button are both wired.
 */
export function HomePopup({ banners = [] }: HomePopupProps) {
  const banner = banners[0] ?? null;
  const [open, setOpen] = useState(false);

  const close = useCallback(() => {
    setOpen(false);
    if (!banner) return;
    try {
      sessionStorage.setItem(`${SEEN_PREFIX}${banner.uuid}`, "1");
    } catch {
      // Ignore: the popup just reappears next session.
    }
  }, [banner]);

  useEffect(() => {
    if (!banner) return;
    const key = `${SEEN_PREFIX}${banner.uuid}`;
    try {
      if (sessionStorage.getItem(key)) return;
    } catch {
      // Private mode can throw on storage access; showing the popup is harmless.
    }
    setOpen(true);
  }, [banner]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    document.addEventListener("keydown", onKey);
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previousOverflow;
    };
  }, [open, close]);

  if (!open || !banner) return null;

  const href = banner.link_url || "/products";
  const target = href.startsWith("/category/")
    ? `/products?category=${href.replace("/category/", "").trim()}`
    : href;

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 animate-fade-up"
      role="dialog"
      aria-modal="true"
      aria-labelledby="home-popup-title"
      onClick={close}
    >
      <div
        className="relative w-full max-w-md bg-white rounded-md overflow-hidden shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <button
          type="button"
          onClick={close}
          aria-label="Close promotion"
          className="absolute top-2.5 right-2.5 z-10 w-9 h-9 rounded-md bg-black/50 hover:bg-black/70 text-white flex items-center justify-center transition-colors cursor-pointer"
        >
          <X className="w-4 h-4" aria-hidden="true" />
        </button>

        <Link href={target} onClick={close} className="block">
          <div className="relative w-full aspect-[4/3] bg-slate-100">
            <Image
              src={banner.image_url}
              alt={banner.title || "Promotional offer"}
              fill
              sizes="(max-width: 640px) 100vw, 448px"
              className="object-cover"
              priority
            />
          </div>
        </Link>
      </div>
    </div>
  );
}
