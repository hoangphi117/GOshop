import React from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import Image from "next/image";

export default function AuthLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div
      className="min-h-screen bg-[#F8FAFC] text-[#131B2A] antialiased flex flex-col justify-between"
      style={{ fontFamily: 'var(--font-body, "Nunito Sans", sans-serif)' }}
    >
      <header className="w-full bg-white/80 backdrop-blur-xl shadow-xs z-50">
        <div className="h-16 max-w-7xl mx-auto px-4 sm:px-8 flex items-center justify-between">
          <div className="flex items-center gap-3 shrink-0">
            <Link
              href="/"
              className="flex items-center gap-2 group bg-white/95 hover:bg-white rounded-xl px-2.5 py-1 shadow-xs"
            >
              <Image
                src="/Logo.svg"
                alt="GOshop — Official E-commerce Store for Tech Collectors and Developers"
                width={150}
                height={38}
                priority
                className="h-7 sm:h-8 w-auto object-contain"
              />
            </Link>
          </div>

          <Link
            href="/"
            className="inline-flex items-center gap-1 text-xs font-semibold text-[#3E484D] hover:text-[#007D9C] transition-colors focus:outline-none focus:ring-2 focus:ring-[#007D9C] rounded-lg px-2 py-1"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to Store</span>
          </Link>
        </div>
      </header>

      <main className="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-8 py-8 flex flex-col items-center justify-center">
        {children}
      </main>

      <footer className="w-full bg-[#F2F3FF] border-t border-[#E2E8F0] py-4">
        <div className="max-w-7xl mx-auto px-4 sm:px-8 flex flex-col sm:flex-row items-center justify-between gap-4 text-center sm:text-left text-xs text-[#3E484D]">
          <p>
            &copy; 2026 GOshop Inc. Crafted for tech collectors and developers.
          </p>
          <nav className="flex items-center gap-4">
            <Link
              href="/terms"
              className="hover:text-[#131B2A] transition-colors"
            >
              Terms of Service
            </Link>
            <Link
              href="/privacy-policy"
              className="hover:text-[#131B2A] transition-colors"
            >
              Privacy Policy
            </Link>
            <Link
              href="/help-center"
              className="hover:text-[#131B2A] transition-colors"
            >
              Help Center
            </Link>
          </nav>
        </div>
      </footer>
    </div>
  );
}
