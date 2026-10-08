import React from "react";
import { StorefrontLayout } from "@/components/layout/storefront-layout";

export default function CheckoutLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <StorefrontLayout>{children}</StorefrontLayout>;
}
