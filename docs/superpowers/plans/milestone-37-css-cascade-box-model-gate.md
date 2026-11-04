# Milestone 37: Eliminate CSS Cascade & Box-Model Traps (Tailwind v4 `@layer`, `@theme`, and Overflow Clipping Gate)

**Goal:** Fix all 3 structural CSS bugs shown in the user's screenshots AND upgrade `~/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py` with Rules 4, 5, and 6 so these CSS cascade / box-model traps can never happen again in any project.

**Root-Cause Diagnosis of the 3 Screenshots:**
1. **Screenshot 1 (Dark Navy Ticket Icon inside Dark Navy Header Badge)**:
   - `Header.tsx` line 57 uses `<TicketIcon size={24} className="text-brand-accent" />` inside `.brand-icon-badge` (`background: linear-gradient(135deg, #1e1b4b, #312e81)`).
   - Because Tailwind v4 (`@import "tailwindcss";`) has no `@theme` registration for `--color-brand-accent` and no `.text-brand-accent` class in `index.css`, `text-brand-accent` is a dead/undefined CSS class!
   - The SVG uses `stroke="currentColor"`, so in Light Mode it inherits `body { color: #0f172a }` (dark navy) inside a dark navy `#1e1b4b` box -> **dark-on-dark invisible logo icon**!
2. **Screenshot 2 (Zero Padding Inside VIP Add-on Cards — Text Touching Borders)**:
   - `frontend/src/index.css` has `@import "tailwindcss";` on line 2, followed by unlayered `* { box-sizing: border-box; margin: 0; padding: 0; }` on line 548!
   - In CSS Cascade Levels (Tailwind v4), **unlayered CSS (`* { padding: 0; margin: 0; }`) ALWAYS overrides `@layer utilities` (`.p-3`, `.p-4`, `.px-6`, `.py-4`, `.m-2`, `.mb-3`)** regardless of specificity!
   - That single `* { margin: 0; padding: 0; }` rule wiped out every Tailwind `p-*` and `m-*` utility across the entire app (`CheckoutForm.tsx`, `VenueSeatMap.tsx`, `AdminDashboardModal.tsx`, `MyBookingsModal.tsx`, etc.)!
3. **Screenshot 3 (Featured Premiere Card Bottom Cut Off When Tags Wrap)**:
   - `frontend/src/index.css` line 3068-3085 sets `.premiere-showcase-card { min-height: 172px; overflow: hidden; }` and `@media (min-width: 768px) { .premiere-showcase-card { height: 172px; } }`.
   - When a long category badge like `TECHNOLOGY & CONFERENCES` wraps the venue location onto a second line, the content height reaches ~200px, and `height: 172px` + `overflow: hidden` slices off the bottom date and `From $...` price pill!

---

## Review Focus (Top 5 Failure Modes)
1. **Unlayered `* { margin: 0; padding: 0; }` Clobbering `@layer utilities`**: Remove `margin: 0; padding: 0;` from unlayered `*` in `index.css` (Tailwind v4's `@layer base` preflight already resets margins/padding cleanly without overriding `@layer utilities`).
2. **Undefined Custom Utility Classes (`text-brand-accent`, `bg-brand-accent/*`, `border-brand-accent/*`)**: Register `--color-brand-accent: var(--brand-accent);` inside `@theme` in `index.css` AND define explicit `.text-brand-accent` and `.brand-icon-badge` color rules (`color: #fbbf24` on the dark indigo badge).
3. **Fixed `height: 172px` + `overflow: hidden` Clipping on Multi-Line Cards**: Remove fixed `height: 172px` from `.premiere-showcase-card` (keep `min-height: 172px; height: auto;` so it never jumps on 1-line cards AND never clips when tags wrap).
4. **Upgrade `ui_audit.py` with Rules 4, 5, and 6**:
   - **Rule 4**: Ban unlayered `* { ... padding: 0 / margin: 0 ... }` in any CSS file that imports Tailwind.
   - **Rule 5**: Ban fixed `height: <px>` on text/card containers that also have `overflow: hidden` (require `min-height` instead).
   - **Rule 6**: Flag undefined custom `text-brand-*` / `bg-brand-*` classes unless defined in CSS or `@theme`.
5. **100% Test & Build Verification**: Verify `ui_audit.py`, `vitest run`, `tsc --noEmit`, and `vite build` (confirming compiled CSS no longer has unlayered `* { margin:0; padding:0 }`).

---

### Task 1: Upgrade `ui_audit.py` with Rules 4, 5, and 6 (RED — Catch All 3 Bugs Automatically)

**Files:**
- Modify: `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py`
- Modify: `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/SKILL.md`

- [ ] **Step 1: Add Rules 4, 5, and 6 to `ui_audit.py`**
  Update `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py` to check:
  - **Rule 4 (Tailwind v4 Unlayered Wildcard Reset Trap)**: If a CSS file contains `@import "tailwindcss"` (or `@tailwind`), flag any unlayered `*` selector that sets `padding:\s*0` or `margin:\s*0` (because unlayered `*` overrides all `@layer utilities` `.p-*` and `.m-*` classes in CSS Cascade Layers!).
  - **Rule 5 (Fixed Height + Overflow Hidden Card Clipping Trap)**: Flag any CSS rule for a card/showcase/panel/modal/container (selector matching `card|showcase|panel|modal|banner|section|box`, excluding decorative `orb|beam|bar|track|fill|thumb|dot|laser|badge|pill|avatar|icon|skeleton|spinner|seat`) that combines `overflow:\s*hidden` (in the base rule or media query) with a fixed pixel `height:\s*\d+px` (must use `min-height` instead so wrapping text is never clipped).
  - **Rule 6 (Undefined Custom Token Utility Classes)**: Scan `.tsx`/`.jsx` for custom token classes like `text-brand-[a-z0-9-]+`, `bg-brand-[a-z0-9-]+`, `border-brand-[a-z0-9-]+` and verify that either `@theme` defines `--color-brand-...` or the CSS file explicitly defines `.text-brand-...`.

- [ ] **Step 2: Run `ui_audit.py` against `frontend/src` (RED)**
  Run:
  ```bash
  python3 /Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src
  ```
  Expected: Fails (RED) catching all 3 bugs (`* { margin: 0; padding: 0 }`, `.premiere-showcase-card { height: 172px }`, and undefined `text-brand-accent`).

---

### Task 2: Add Milestone 37 Regression Tests to `dark_mode_desktop.test.ts` (RED)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/test/dark_mode_desktop.test.ts`

- [ ] **Step 1: Add Milestone 37 test block in `frontend/src/test/dark_mode_desktop.test.ts`**
  Add `describe('Milestone 37: Tailwind v4 Cascade Layer Safety, Brand Icon Contrast & Card Overflow Protection', ...)` asserting:
  1. `index.css` does NOT contain an unlayered `* { ... padding: 0 ... }` reset that clobbers Tailwind v4 `@layer utilities` `.p-*` and `.m-*` classes.
  2. `index.css` defines `@theme` with `--color-brand-accent` AND explicit `.text-brand-accent` + `.brand-icon-badge` contrast (`color: #fbbf24`) so `<TicketIcon className="text-brand-accent" />` is bright gold in both Dark and Light mode.
  3. `.premiere-showcase-card` uses `min-height: 172px` without a fixed `height: 172px` media-query lock so multi-line wrapped category/venue headers never clip the bottom date and price pills.
  4. `.addon-upsell-card` has explicit `padding: 0.875rem 1rem` (`14px 16px`) so internal text and badges never touch the rounded card border.

- [ ] **Step 2: Run Vitest to witness the failure (RED)**
  Run:
  ```bash
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run src/test/dark_mode_desktop.test.ts < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ PASS" || (echo "❌ FAIL:" && tail -n 35 /tmp/agy-cmd.log)
  ```

---

### Task 3: Fix `frontend/src/index.css` Cascade Reset, Brand Accent Tokens, Add-on Card Padding & Showcase Card Height (GREEN)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/index.css`

- [ ] **Step 1: Register `@theme` for `--color-brand-accent` and define `.text-brand-accent` + `.brand-icon-badge` color**
  In `frontend/src/index.css`:
  - Right after `@import "tailwindcss";`, add:
    ```css
    @theme {
      --color-brand-accent: var(--brand-accent);
    }
    ```
  - In `.brand-icon-badge` (around line 720), add `color: #fbbf24;` so the ticket icon inside the dark indigo badge is always luminous gold in both Dark and Light themes.
  - Define `.text-brand-accent { color: var(--brand-accent); }` and `.brand-icon-badge .text-brand-accent { color: #fbbf24 !important; }`.

- [ ] **Step 2: Remove `margin: 0; padding: 0;` from unlayered `*` selector**
  In `frontend/src/index.css` (around line 548):
  - Change:
    ```css
    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
      scrollbar-width: thin;
      scrollbar-color: var(--border-light) transparent;
    }
    ```
    to:
    ```css
    * {
      box-sizing: border-box;
      scrollbar-width: thin;
      scrollbar-color: var(--border-light) transparent;
    }
    ```
    (Tailwind v4 `@layer base` already sets `margin: 0; padding: 0;` inside `@layer base`, which allows `@layer utilities` `.p-3`, `.p-4`, `.m-2`, etc. to work properly across every component!)
  - Also add explicit base styling for `.addon-upsell-card` in `index.css`:
    ```css
    .addon-upsell-card {
      padding: 0.875rem 1rem;
    }
    ```

- [ ] **Step 3: Fix `.premiere-showcase-card` fixed height clipping**
  In `frontend/src/index.css` (around line 3081):
  - Remove the `@media (min-width: 768px) { .premiere-showcase-card { height: 172px; } }` rule so `.premiere-showcase-card` uses `min-height: 172px; height: auto;` and never clips wrapped content at the bottom.

- [ ] **Step 4: Run `ui_audit.py`, Vitest Suite, TypeScript Compiler, and Vite Production Build Check (GREEN)**
  Run:
  ```bash
  python3 /Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ VITEST PASS" || (echo "❌ FAIL:" && tail -n 35 /tmp/agy-cmd.log)
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install -p typescript tsc --noEmit --pretty false < /dev/null > /tmp/agy-tsc.log 2>&1 && echo "✅ TSC PASS" || (echo "❌ TSC FAIL:" && tail -n 35 /tmp/agy-tsc.log)
  ```
