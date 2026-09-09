---
name: GOshop Storefront
description: Modern B2C e-commerce with Material Blue palette inspired by pikamc.vn floating glass aesthetic
colors:
  lightest: "#E3F2FD"
  light: "#90CAF9"
  primary: "#2196F3"
  dark: "#0D47A1"
  background: "#FAFBFC"
  surface: "#FFFFFF"
  text-primary: "#0D1B2A"
  text-secondary: "#64748B"
  success: "#10B981"
  warning: "#F59E0B"
  error: "#EF4444"
  on-primary: "#FFFFFF"
typography:
  display:
    fontFamily: Inter
    fontSize: 3.5rem
    fontWeight: 800
  h1:
    fontFamily: Inter
    fontSize: 2.25rem
    fontWeight: 700
  h2:
    fontFamily: Inter
    fontSize: 1.5rem
    fontWeight: 600
  body:
    fontFamily: Inter
    fontSize: 1rem
    fontWeight: 400
    lineHeight: 1.6
  caption:
    fontFamily: Inter
    fontSize: 0.75rem
    fontWeight: 500
rounded:
  sm: 6px
  md: 12px
  lg: 16px
  full: 9999px
spacing:
  xs: 4px
  sm: 8px
  md: 16px
  lg: 24px
  xl: 48px
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    rounded: "{rounded.md}"
    padding: 12px 24px
  button-primary-hover:
    backgroundColor: "{colors.dark}"
  card-product:
    backgroundColor: "{colors.surface}"
    rounded: "{rounded.lg}"
    padding: 16px
  navbar:
    backgroundColor: "rgba(255, 255, 255, 0.85)"
    rounded: "{rounded.full}"
---

# Design System: GOshop Storefront

## 1. Visual Theme & Atmosphere
Modern Material Blue e-commerce storefront taking inspiration from the hybrid SaaS floating aesthetic of pikamc.vn:
Floating Glass Navbar, deep gradient hero section, and a deliberate rhythm of dark and light sections. The interface exudes trust, speed, precision, and a clean buying experience.

## 2. Color Palette & Roles
- **Primary (#2196F3)**: Vibrant Material Blue — Main CTA buttons (Add to Cart, Buy Now, Checkout), active tab highlights, link accents.
- **Dark (#0D47A1)**: Deep Royal Indigo — Hero background sections, footer canvas, high-emphasis headlines.
- **Light (#90CAF9)**: Soft Sky Blue — Micro-interactions, hover outlines, subtle badge borders. (Never used for body text on light backgrounds to preserve WCAG AA contrast).
- **Lightest (#E3F2FD)**: Ice Blue — Background canvas, section alternate fills, feature highlight card surfaces.
- **Background (#FAFBFC)**: Crisp Off-White — Page body background.
- **Surface (#FFFFFF)**: Pure White — Product cards, modal dialogs, drawer panels.
- **Text Primary (#0D1B2A)**: Deep Navy Slate — Maximum readability for headers and body copy.
- **Text Secondary (#64748B)**: Cool Slate — Meta information, subheadings, SKU and timestamps.
- **Success (#10B981)**: Emerald Green — In-stock tags, applied coupon badge, verified purchase badge.
- **Error (#EF4444)**: Crimson Red — Form validation errors, strikethrough original prices, out-of-stock badges.

## 3. Typography Rules
- **Font Family:** `Inter`, system-ui, sans-serif.
- **Display Headings:** Bold (800), tight letter-spacing (`tracking-tight`), used in hero title and key value props.
- **Content Headings:** Semi-bold to Bold (600 - 700), providing clear content hierarchy.
- **Body & Microcopy:** Regular (400) to Medium (500) with line-height 1.6 for readability.

## 4. Component Stylings
- **Buttons:**
  - Primary CTA: Pill or rounded-lg, Material Blue background (`#2196F3`), crisp white typography, subtle hover scale & transition.
  - Secondary / Outline: Border `#2196F3` or neutral slate with backdrop blur.
- **Product Cards:**
  - Clean card surface (`#FFFFFF`), subtle border, gentle rounded corners (`16px`), soft shadow elevation on hover (`hover:-translate-y-1 hover:shadow-lg`).
  - Image container with aspect ratio `aspect-square`, object-cover image, quick add-to-cart button.
- **Navbar:**
  - Floating pill-shape (`rounded-full`), `backdrop-blur-md`, fixed at top with safe container margins, containing Logo, Categories menu, Search trigger, Cart drawer button with item badge, and Auth button.
- **Footer:**
  - Deep Indigo canvas (`#0D47A1`), high contrast text in `#E3F2FD`, comprehensive links, trust certificates, and copyright notice.

## 5. Layout Principles & Rhythm
- **Rhythm:** Dark Hero Section $\to$ Light Product Showcase $\to$ Soft Ice Blue Benefits & Categories $\to$ Pure White Flash Sale Grid $\to$ Deep Indigo Footer.
- **Grid:** 4 columns on desktop (`grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4`), 2 columns on tablet, 1 column on mobile.
- **Language Convention:** 100% English for all UI, buttons, alerts, headings, placeholders, and notification toasts per repository standard.
