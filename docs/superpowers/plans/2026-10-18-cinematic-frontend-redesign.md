# Implementation Plan: TicketVerse Cinematic Frontend Redesign (M22)

Elevate the TicketVerse frontend (React, Vite, TypeScript) into a high-end, cinematic web application with deep OLED contrast, dramatic gold spotlights, curved amphitheater screen projection, tactile glassmorphism, and strict WCAG 4.5:1 accessibility.

---

## 1. Objectives & Requirements

- **Design System Tokens (`frontend/src/index.css`)**:
  - Integrate tokens generated from `design-system/ticketverse/MASTER.md` into CSS custom properties:
    - OLED Background: `--color-background: #0F0F23`, `--color-card: #1B1B30`, border `--color-border: #4338CA`
    - Spotlight Gold / CTA: `--color-accent: #CA8A04`, `--color-accent-hover: #EAB308`, `--color-on-accent: #0F172A`
    - Primary & Secondary: `--color-primary: #1E1B4B`, `--color-secondary: #312E81`
    - Foreground: `--color-foreground: #F8FAFC`, `--color-muted: #94A3B8`
    - Luminous Seat States:
      - Available: `#10B981` (emerald luminous glow)
      - Selected: `#F59E0B` (spotlight amber glow)
      - Booked/Reserved: `#475569` (tactile slate)
      - VIP: `#A855F7` (royal purple luminous glow)
      - Waitlist: `#06B6D4` (cyan neon glow)
    - Preserve existing tokens (`--bg-primary`, `--bg-surface`, `--accent-primary`) for backward compatibility with existing tests and services.
  - Import cinematic typography: Righteous (`font-family: 'Righteous', cursive, sans-serif`) for headers and Poppins/Plus Jakarta Sans for clean high-density typography.

- **Component Redesign & Elevation**:
  - **Header & Navigation (`frontend/src/components/Header.tsx`)**:
    - High-contrast Sign In / Profile buttons with tactile hover transforms (`translateY(-2px)`, subtle gold glow).
    - Sleek API Gateway status pill with pulsating animated dot, clear latency indicator text.
    - Polished branding badge with glowing ticket emblem, dual-tone metallic gradient typography.
  - **Event Catalog & Cards (`frontend/src/components/EventCatalog.tsx`, `EventCard.tsx`)**:
    - Cinematic movie/concert cards with poster aspect ratio headers, subtle backdrop blur, and glowing category badges.
    - Pricing chips with clear "Starting at" highlight.
    - Tactile "Select Seats" action button with smooth transitions and gold spotlight hover.
    - Refined search & category filtering bar with pill tabs and glassmorphic input.
  - **Venue & Interactive Seat Map (`frontend/src/components/VenueSeatMap.tsx`, `SeatGrid.tsx`, `SeatNode.tsx`)**:
    - Premium curved cinema screen canvas with glowing projection beam/arc light effect and "CINEMA SCREEN / STAGE" title.
    - Row and column indicators along both flanks and top column numbers for spatial orientation.
    - Tactile seat nodes with 5 distinct states: Available, Selected, Reserved/Booked, VIP, Waitlist.
    - Smooth hover tooltips showing row, number, tier, price, and status.
  - **Booking & Auth Modals (`frontend/src/components/BookingModal.tsx`, `CheckoutForm.tsx`, `AuthModal.tsx`)**:
    - Deep glassmorphic overlay (`backdrop-filter: blur(12px)`).
    - 3-step progression header (`1. Seat Selection -> 2. Review Details -> 3. Payment Simulation`).
    - High-contrast inputs with floating focus rings (`box-shadow: 0 0 0 3px rgba(202, 138, 4, 0.35)`).
    - Explicit accessible error messages and conflict handling banners.

- **Pre-Delivery Quality Checklist**:
  - All icons are Lucide-compatible SVGs (zero raw emojis).
  - Explicit `cursor-pointer` and 150-250ms transitions on all interactive elements.
  - WCAG 4.5:1 text/button contrast verified for dark and light modes.
  - Vitest test suite (`npm test`) and TypeScript check (`npm run lint` / `npx tsc --noEmit`) pass 100%.

---

## 2. Review Focus (Top 5 Edge Cases & Failure Modes)

1. **Light Mode Semantic Consistency**: Ensure updating `:root` tokens does not break the light mode overrides in `html[data-theme='light']` or `body.light`.
2. **Keyboard Accessibility & Focus Rings**: Confirm every button and seat node maintains `:focus-visible` outline without breaking layout or overlapping adjacent seats.
3. **Seat Map Responsiveness**: Verify that the curved screen canvas and seat grid maintain proper padding and horizontal overflow containment on 375px mobile up to 1440px desktop screens.
4. **Auth Modal & Booking Modal Layering**: Ensure modal z-indices and backdrop blurs layer cleanly over sticky headers and fixed selection footer without flickering or scroll bleed.
5. **Existing Vitest Test Contracts**: Maintain exact component prop interfaces for `Header`, `CheckoutForm`, `SeatNode`, `SeatGrid`, and `EventCard` to preserve 100% test pass rate.

---

## 3. Implementation Steps

### Step 1: Design Tokens & Typography in `frontend/src/index.css` and `index.html`
- Add Google Fonts link for Righteous & Poppins in `index.html`.
- Add `@import` and define semantic tokens (`--color-background`, `--color-card`, `--color-accent`, luminous seat variables, etc.) in `frontend/src/index.css`.
- Add cinematic curved screen styles, projection light effects, tactile button hover states, and glassmorphic modal styles.

### Step 2: Header & Navigation Elevation
- Update `frontend/src/components/Header.tsx` and styling.
- Polish branding, gateway status pill, profile badge, and tactile sign-in button.

### Step 3: Event Catalog & Cinematic Cards
- Update `frontend/src/components/EventCatalog.tsx` and `EventCard.tsx`.
- Enhance search bar with glassmorphic depth, glowing category pills, poster-style card layout, and pricing chips.

### Step 4: Venue Cinema Screen & Interactive Seat Map
- Update `frontend/src/types/index.ts` to include `'WAITLIST'` status.
- Update `frontend/src/components/VenueSeatMap.tsx`, `SeatGrid.tsx`, and `SeatNode.tsx`.
- Implement curved projection screen SVG, column indicators, and tactile luminous seat states with rich accessible tooltips.

### Step 5: Booking & Auth Glassmorphic Modals with Step Progression
- Update `frontend/src/components/BookingModal.tsx`, `CheckoutForm.tsx`, and `AuthModal.tsx`.
- Add visual 3-step progress bar, floating focus rings on inputs, and clear error banners.

### Step 6: Verification & QA
- Run `npm test` and `npm run lint`.
- Verify Pre-Delivery Checklist items.
