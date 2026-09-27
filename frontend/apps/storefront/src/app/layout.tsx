import type { Metadata } from 'next'
import { Inter } from 'next/font/google'
import './globals.css'
import { QueryProvider } from '@/providers/query-provider'
import { ThemeProvider } from '@/providers/theme-provider'
import { Navbar } from '@/components/layout/navbar'
import { Footer } from '@/components/layout/footer'
import { CartDrawer } from '@/components/cart/cart-drawer'
import { ScrollToTop } from '@/components/layout/scroll-to-top'
import { ChatWidget } from '@/components/chat/chat-widget'
import { MainContainer } from '@/components/layout/main-container'
import { Toaster } from 'sonner'

const inter = Inter({
  variable: '--font-inter',
  subsets: ['latin'],
  display: 'swap',
})

export const metadata: Metadata = {
  title: 'GOshop — Gaming Merch, Plushies, Tees & Collectibles',
  description:
    'Discover authentic game boss plushies, graphic printed gamer tees, and collectible toys with fast shipping and secure payments.',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en" className={`${inter.variable} h-full antialiased`}>
      <body className="min-h-full flex flex-col bg-[#FAFBFC] text-[#0D1B2A]">
        <ThemeProvider>
          <QueryProvider>
            <Navbar />
            <CartDrawer />
            <MainContainer>{children}</MainContainer>
            <Footer />
            <ScrollToTop />
            <ChatWidget />
            <Toaster richColors position="top-right" />
          </QueryProvider>
        </ThemeProvider>
      </body>
    </html>
  )
}
