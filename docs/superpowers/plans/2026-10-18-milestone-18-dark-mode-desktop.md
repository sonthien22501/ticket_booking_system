# Milestone 18: Dark Mode Interface Toggle & Modern Desktop Cross-Browser Usability (R28, R32) Implementation Plan

## Goal
Implement a responsive dark/light visual theme toggle (**Requirement R28**) with persistent preferences in local storage, automatic system OS preference detection (`prefers-color-scheme`), and full usability conformance across modern desktop browsers (**Requirement R32** - Chromium, Firefox, WebKit). Also formally mark **Requirement R1** (React-based frontend for users) as fully completed.

---

## Technical Design & Scope
1. **Theme Management Service (`frontend/src/services/theme.ts`)**:
   - `getInitialTheme()`: Inspects `localStorage['ticketverse_theme']`, falls back to `window.matchMedia('(prefers-color-scheme: dark)')`, defaults to `dark`.
   - `toggleThemeValue()`: Switches cleanly between `'dark'` and `'light'`.
   - `applyThemeToDocument()`: Injects `data-theme` attribute and `dark`/`light` CSS classes on `document.documentElement` and persists to `localStorage`.
2. **Accessible Theme Toggle Component (`frontend/src/components/ThemeToggle.tsx`)**:
   - Switch button with `role="switch"`, `aria-checked`, `aria-label`, and `tabIndex={0}`.
   - Smooth animated thumb transition with Sun and Moon SVG vector icons.
   - Keyboard accessibility supporting both `Enter` and `Space` triggers.
3. **Desktop Cross-Browser Polish (`frontend/src/index.css`)**:
   - Dual-mode CSS custom properties (`--bg-primary`, `--bg-surface`, `--text-primary`, `--border-color`, `--accent-primary`).
   - Cross-browser scrollbar standards: `scrollbar-width: thin` and `scrollbar-color` for Firefox; `::-webkit-scrollbar` for Chromium and WebKit browsers.
   - `:focus-visible` accessible outline rings across all interactive controls.
   - Fluid typography and viewport constraint preservation preventing horizontal scroll clipping.
4. **Header Integration (`frontend/src/components/Header.tsx`, `frontend/src/App.tsx`)**:
   - Integrated theme toggle into header status bar beside gateway connection indicator.

---

## Verification
- Unit test suite: `frontend/src/test/dark_mode_desktop.test.ts` (100% pass).
- TypeScript compile & Vite production build: `npm run build` (100% pass).
- Regression suite: All existing Vitest suites (`seatmap_dialog.test.ts`, `unit.test.ts`) pass.
