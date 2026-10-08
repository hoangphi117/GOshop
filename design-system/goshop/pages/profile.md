# GOshop Storefront — Customer Profile Page Specification

> **Screen Reference:** Stitch Project `1907536566999352297` &bull; Screen `94d8f422bbb440898e87ce5155ea5a82`
> **Design Theme:** GOshop GoLang Blue (`#007D9C`), Flat Solid Colors, Strictly `rounded-md` Max Radius, Zero Animation, Zero Gradient.

---

## 1. Visual Directives
- **Zero Gradient:** Strictly use solid color backgrounds (`#007D9C`, `#00ADD8`, `#F8FAFC`, `#FFFFFF`, `#0F172A`).
- **Zero Animation:** Instant state switches; no CSS keyframe spins, pulses, or bounce effects.
- **Max Radius (`rounded-md`):** All cards, inputs, buttons, badges, and avatar frames must not exceed `6px–8px` (`rounded-md` / `rounded-sm`). Never use `rounded-xl`, `rounded-2xl`, or `rounded-3xl`.
- **Typography:**
  - Headings & Action Labels: `Rubik` (`--font-heading`)
  - Form Fields & Body: `Nunito Sans` (`--font-body`)

---

## 2. Color Tokens Matrix

| Element Role | Color Token | Class / Value |
|---|---|---|
| Background Canvas | `#F8FAFC` | `bg-[#F8FAFC]` |
| Card & Form Surfaces | `#FFFFFF` | `bg-white border border-[#E2E8F0]` |
| Primary Actions (Save, Active Tab) | `#007D9C` | `bg-[#007D9C] text-white hover:bg-[#006782]` |
| Secondary Highlight / Active Tint | `#E6F7FB` / `#007D9C` | `bg-[#E6F7FB] text-[#007D9C]` |
| Heading & Primary Text | `#0F172A` | `text-[#0F172A]` |
| Field Labels & Muted Meta | `#64748B` | `text-slate-500` / `text-slate-600` |
| Verified Status Pill | `#10B981` / `#ECFDF5` | `bg-emerald-50 text-emerald-700 border border-emerald-200` |
| Danger / Sign Out Action | `#DC2626` / `#FEF2F2` | `text-rose-600 hover:bg-rose-50` |

---

## 3. Layout Structure (12-Column Grid)

### 3.1 Left Sidebar (3 Columns / `lg:col-span-3`)
1. **User Profile Summary Card (`rounded-md border border-[#E2E8F0] p-4 bg-white`):**
   - Square avatar (`w-12 h-12 rounded-md bg-[#007D9C] text-white font-bold`).
   - Full name, email address, and "Verified Member" pill (`rounded-md text-[10px]`).
2. **Account Navigation Menu (`rounded-md border border-[#E2E8F0] p-2 bg-white`):**
   - Personal Profile (Active: `bg-[#E6F7FB] text-[#007D9C] rounded-md font-bold`).
   - My Orders, Notifications (with counter badge), Returns & Refunds, My Reviews, Shipping Addresses, Password & Security.
   - Sign Out trigger (`text-rose-600 rounded-md`).
3. **Membership Loyalty Card (`rounded-md border border-[#E2E8F0] p-4 bg-white`):**
   - Tier status ("Gold Gopher Member"), Points balance ("2,450 Points"), Available Coupons count ("4 Coupons").

### 3.2 Main Profile Content (9 Columns / `lg:col-span-9`)
1. **Header:** Title "Personal Profile" (`text-xl font-bold text-[#0F172A]`) & description.
2. **Quick Metrics (3 Cards):**
   - Total Orders (`28 Orders`)
   - Saved Addresses (`3 Locations`)
   - Active Product Reviews (`12 Reviews`)
3. **Personal Info Form Card (`rounded-md border border-[#E2E8F0] p-6 bg-white`):**
   - Avatar management row with "Change Photo" (`border border-[#007D9C] text-[#007D9C] rounded-md`) and "Remove" text.
   - 2-Column form grid:
     - Full Name (`input rounded-md bg-slate-50 border border-[#E2E8F0]`)
     - Display Name / Nickname
     - Email Address (disabled, verified badge)
     - Phone Number
     - Date of Birth (Day, Month, Year select inputs)
     - Gender selection (Radio group: Male, Female, Other)
4. **Preferences & Security Card:**
   - Default Shipping Address preview with quick edit shortcut.
   - Communication checkboxes (SMS / Email updates).
   - Two-Factor Authentication (2FA) status badge.
5. **Footer Actions:**
   - Discard / Reset button (`rounded-md border border-[#E2E8F0] text-slate-700`).
   - Save Changes button (`rounded-md bg-[#007D9C] text-white font-bold hover:bg-[#006782]`).
