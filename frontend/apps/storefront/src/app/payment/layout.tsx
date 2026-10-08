import React from "react";
import { StorefrontLayout } from "@/components/layout/storefront-layout";

export default function PaymentLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <StorefrontLayout>{children}</StorefrontLayout>;
}
