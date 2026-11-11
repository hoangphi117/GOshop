import React from "react";
import { CartDrawer } from "@/components/cart/cart-drawer";
import { ChatWidget } from "@/components/chat/unified-chat-widget";
import { Footer } from "@/components/layout/footer";
import { MainContainer } from "@/components/layout/main-container";
import { Navbar } from "@/components/layout/navbar";
import { ScrollToTop } from "@/components/layout/scroll-to-top";

export function StorefrontLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Navbar />
      <CartDrawer />
      <MainContainer>{children}</MainContainer>
      <Footer />
      <ScrollToTop />
      {/* One launcher for both the AI assistant and the human support team. */}
      <ChatWidget />
    </>
  );
}
