'use client'

import React, { useState, useEffect } from 'react'
import { ArrowUp } from 'lucide-react'

interface ScrollToTopProps {
  threshold?: number
}

export function ScrollToTop({ threshold = 400 }: ScrollToTopProps) {
  const [isVisible, setIsVisible] = useState(false)

  useEffect(() => {
    const handleScroll = () => {
      if (window.scrollY > threshold) {
        setIsVisible(true)
      } else {
        setIsVisible(false)
      }
    }

    // Check initial scroll position
    handleScroll()

    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => window.removeEventListener('scroll', handleScroll)
  }, [threshold])

  const scrollToTop = () => {
    window.scrollTo({
      top: 0,
      behavior: 'smooth',
    })
  }

  return (
    <button
      onClick={scrollToTop}
      aria-label="Scroll to top"
      title="Scroll to top"
      className={`fixed bottom-6 right-6 z-40 w-11 h-11 rounded-full bg-[#0D47A1] text-white shadow-xl hover:bg-[#2196F3] flex items-center justify-center transition-all duration-300 transform hover:scale-110 active:scale-95 border border-white/20 focus:outline-none focus:ring-2 focus:ring-[#2196F3]/50 ${
        isVisible
          ? 'opacity-100 translate-y-0 pointer-events-auto'
          : 'opacity-0 translate-y-4 pointer-events-none'
      }`}
    >
      <ArrowUp className="w-5 h-5 transition-transform group-hover:-translate-y-0.5" />
    </button>
  )
}
