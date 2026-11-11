import React from "react";
import { cookies } from "next/headers";
import { CartDrawer } from "@/components/cart/cart-drawer";
import { ChatWidget } from "@/components/chat/unified-chat-widget";
import { Footer } from "@/components/layout/footer";
import { MainContainer } from "@/components/layout/main-container";
import { Navbar } from "@/components/layout/navbar";
import { ScrollToTop } from "@/components/layout/scroll-to-top";
import { ANON_COOKIE_NAME, fetchSearchHistory } from "@/lib/search-history";

export async function StorefrontLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // Read search history on the server so the dropdown has data on first paint.
  // The anon cookie belongs to this very request, so forwarding it is the one
  // identity the visitor already proved by arriving here.
  const cookieStore = await cookies();
  const anonId = cookieStore.get(ANON_COOKIE_NAME)?.value;
  const recentSearches = anonId ? await fetchSearchHistory(`goshop_anon_id=${anonId}`) : [];

  return (
    <>
      <Navbar recentSearches={recentSearches} />
      <CartDrawer />
      <MainContainer>{children}</MainContainer>
      <Footer />
      <ScrollToTop />
      {/* One launcher for both the AI assistant and the human support team. */}
      <ChatWidget />
    </>
  );
}
