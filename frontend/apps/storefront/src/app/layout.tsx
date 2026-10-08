import type { Metadata } from "next";
import { Rubik, Nunito_Sans } from "next/font/google";
import "./globals.css";
import NextTopLoader from "nextjs-toploader";
import { QueryProvider } from "@/providers/query-provider";
import { ThemeProvider } from "@/providers/theme-provider";
import { Toaster } from "sonner";

const rubik = Rubik({
  variable: "--font-heading",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700", "800"],
  display: "swap",
});

const nunitoSans = Nunito_Sans({
  variable: "--font-body",
  subsets: ["latin"],
  weight: ["300", "400", "500", "600", "700", "800"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "GOshop — Official Gaming Merch, Boss Plushies, Tees & Collectibles",
  description:
    "Discover authentic game boss plushies, graphic printed gamer tees, and collectible toys with express shipping and 100% genuine guarantee.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      className={`${rubik.variable} ${nunitoSans.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col bg-[#F8FAFC] text-[#0F172A] font-body selection:bg-emerald-100 selection:text-emerald-900">
        <NextTopLoader
          color="#007D9C"
          initialPosition={0.08}
          crawlSpeed={200}
          height={3}
          crawl={true}
          showSpinner={false}
          easing="ease"
          speed={200}
          shadow="0 0 10px #007D9C,0 0 5px #00ADD8"
        />
        <ThemeProvider>
          <QueryProvider>
            {children}
            <Toaster richColors position="top-center" />
          </QueryProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
