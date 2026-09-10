'use client'

import React from 'react'
import { usePathname } from 'next/navigation'

export function MainContainer({ children }: { children: React.ReactNode }) {
  const pathname = usePathname()
  const isHome = pathname === '/'

  return (
    <main className={`flex-1 flex flex-col ${isHome ? '' : 'pt-20 sm:pt-24'}`}>
      {children}
    </main>
  )
}
