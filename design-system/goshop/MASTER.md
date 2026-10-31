# GOshop Design System — MASTER.md

> **Global Source of Truth:** Official GoLang Blue theme for **Storefront (Next.js App Router)** and **Admin Dashboard (Vite + React 19)**.
> When building or refactoring a specific page, check `design-system/goshop/pages/[page-name].md`. If that file exists, its rules **override** this Master file.

---

## 1. Brand Identity & Visual Vibe
- **Product Type:** High-Conversion E-commerce Platform & Precision Admin Dashboard.
- **Visual Style:** Modern Go-Themed Tech-Commerce (Vibrant GoLang Blue with High Contrast & Subtle Glassmorphism).
- **Core Philosophy:** High contrast, deliberate spatial rhythm, content-first layout, zero "AI-slop" (no generic purple gradients, no arbitrary rounded bubbles, zero layout shifts).

---

## 2. Color Palette & Design Tokens (GoLang Brand Edition)

Fully compatible with Tailwind CSS & CSS Variables (`globals.css` and `index.css`).

### 2.1 Color Tokens Table

| Token Role | Hex (Light) | Hex (Dark) | CSS Variable | Intended Usage |
|:---|:---|:---|:---|:---|
| **Primary (Go Blue)** | `#007D9C` | `#00ADD8` | `--color-primary` | Main brand identity, high-contrast buttons, active nav indicators |
| **Primary Light / Cyan** | `#00ADD8` | `#38BDF8` | `--color-primary-cyan` | Gopher Cyan highlights, badge backgrounds, glow rings |
| **Primary Foreground** | `#FFFFFF` | `#082F49` | `--color-on-primary` | Text and icons on primary background |
| **Secondary (Deep Teal)** | `#005F73` | `#0EA5E9` | `--color-secondary` | Secondary buttons, filter chips, secondary stats |
| **Accent / CTA (Flame)** | `#F26522` | `#FB923C` | `--color-accent` | "Buy Now", Flash Sale urgency, Cart checkout trigger |
| **Accent Foreground** | `#FFFFFF` | `#431407` | `--color-on-accent` | Text and icons on accent background |
| **Background** | `#F8FAFC` | `#070B12` | `--color-background` | Global app background |
| **Foreground** | `#0F172A` | `#F1F5F9` | `--color-foreground` | Primary text (contrast ratio >= 7:1) |
| **Card / Surface** | `#FFFFFF` | `#0E1726` | `--color-card` | Product cards, table panels, drawer content surfaces |
| **Card Foreground** | `#0F172A` | `#F1F5F9` | `--color-card-foreground` | Content text on cards |
| **Muted** | `#F1F5F9` | `#172338` | `--color-muted` | Disabled input backgrounds, scrollbar tracks |
| **Muted Foreground** | `#64748B` | `#94A3B8` | `--color-muted-foreground` | Secondary labels, timestamps, subtitles |
| **Border** | `#E2E8F0` | `#1E2E48` | `--color-border` | Subtle 1px dividers and card borders |
| **Destructive** | `#DC2626` | `#EF4444` | `--color-destructive` | Delete actions, payment errors, order cancellations |
| **Ring / Focus** | `#00ADD8` | `#38BDF8` | `--color-ring` | Keyboard focus indicator ring (A11y) |

---

## 3. Typography & Font Pairing
- **Heading Font:** `Rubik` (Solid geometric structure, builds trust and energy).
- **Body & Data Font:** `Nunito Sans` (Friendly, open counters, clear numerical legibility for prices).

```css
@import url('https://fonts.googleapis.com/css2?family=Nunito+Sans:ital,opsz,wght@0,6..12,300..800;1,6..12,300..800&family=Rubik:ital,wght@0,300..900;1,300..900&display=swap');

:root {
  --font-heading: 'Rubik', sans-serif;
  --font-body: 'Nunito Sans', sans-serif;
}
```

---

## 4. Spacing, Radius & Shadows (Spatial System)

### 4.1 Spacing Scale (8pt Grid)
- `4px` (`--space-xs`): Tight gaps between icons and inline badges.
- `8px` (`--space-sm`): Padding inside chips, inline button gaps.
- `16px` (`--space-md`): Standard input padding, card inner gutters.
- `24px` (`--space-lg`): Primary card padding and drawer spacing.
- `32px` (`--space-xl`): Spacing between section component blocks.
- `48px` (`--space-2xl`): Vertical rhythm between sections on Storefront.

### 4.2 Border Radius
- **Small (Inputs, Badges, Tabs):** `6px` (`rounded-md`)
- **Medium (Buttons, Dropdowns, Tooltips):** `8px` (`rounded-lg`)
- **Large (Cards, Dialogs, Drawers):** `12px` (`rounded-xl` / `rounded-2xl`)
- **Pill (Status Badges, Filter Chips):** `9999px` (`rounded-full`)

---

## 5. Standard Component Specs (shadcn/ui Primitives)
- **Button:** 5 variants (default, secondary, accent, outline, ghost) + 4 sizes (sm, md, lg, icon).
- **Input:** Clean 1px border `#E2E8F0` with focus ring `ring-2 ring-[#00ADD8]`.
- **Badge:** Solid and soft-tinted status pills.
- **Cart Drawer:** Slide-over sheet from right, includes free shipping progress bar, quantity stepper, coupon input, and sticky checkout footer.
