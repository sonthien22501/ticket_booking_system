# Milestone 36: Semantic Theme Architecture & Automated UI Contrast Gate (`ui_audit.py`)

**Goal:** Eliminate all Dark/Light "split-brain" theme collisions across TicketVerse (`frontend/src/index.css` and React components) AND upgrade `ui-ux-pro-max` + `auto-ship` with an automated static theme & contrast gate (`~/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py`) that fails in `<0.2s` if any rule mixes hardcoded dark backgrounds with semantic text variables or leaves white-on-white / dark-on-dark collisions.

**Architecture & Root-Cause Summary:**
1. **Why Screenshot 1–4 Happened:**
   - `.form-input`, `.payment-method-card`, `.order-summary-section` hardcoded `background: #0f0f23` while `.modal-container` flipped to `var(--bg-surface)` (`#ffffff`) and `color: var(--text-primary)` flipped to `#0f172a` (dark navy text inside pitch-black inputs/cards).
   - `.booking-ref-card`, `.confirmation-details-card`, `.digital-ticket-card` hardcoded `background: #0f172a` while `.detail-value` inherited dark text `#0f172a` -> black-on-black invisible confirmation details!
   - `.btn-print-ticket` used `background: var(--bg-surface-elevated)` (`#f1f5f9` in light mode) with hardcoded `color: #ffffff` -> white-on-white invisible action buttons!
   - `.premiere-showcase-card` was overridden to a light gradient in Light Mode, but child `.premiere-category-tag` (`color: #c7d2fe`), `.premiere-price-pill` (`color: #6ee7b7`), and the date text (`text-slate-300`) kept pastel dark-mode text colors -> invisible on white/lavender!
   - `.selection-summary-footer` (sticky bottom bar) hardcoded `background: rgba(30, 41, 59, 0.95)` and `.btn-checkout:disabled` hardcoded `#27273b`.
   - `VenueSeatMap.tsx` (`ai-concierge-panel` & `floorplan-overview-card`), `CheckoutForm.tsx` (VIP add-on cards & corporate billing box), `BookingConfirmation.tsx` (confirmed add-on rows & action buttons), and `Header.tsx` (`btn-admin-studio`) used hardcoded dark Tailwind classes (`bg-slate-950/70`, `bg-slate-900/60`, `text-slate-100`, `text-amber-300`) without light-theme support.

**Tech Stack:** Python 3 (for `ui_audit.py`), React 18, TypeScript, Tailwind CSS v4 + CSS Custom Properties (`frontend/src/index.css`), Vitest.

---

## Review Focus (Top 5 Failure Modes & Edge Cases)
1. **CSS Rule Split-Brain Collision**: A CSS selector sets a hardcoded dark background (`#0f0f23`, `#0f172a`, `#1e293b`) while inheriting or setting `color: var(--text-primary)` (or vice-versa: `background: var(--bg-surface-elevated)` with hardcoded `color: #ffffff`).
2. **Premiere Showcase Child Contrast in Light Mode**: `.premiere-category-tag`, `.premiere-price-pill`, and `.premiere-showcase-meta` date text must have WCAG AA contrast ($\ge 4.5:1$) in both Dark Mode and Light Mode.
3. **Checkout & Confirmation Modal Readability**: `.form-input`, `.payment-method-card`, `.order-summary-section`, `.booking-ref-card`, `.confirmation-details-card`, `.digital-ticket-card`, and `.btn-print-ticket` must use semantic surface/text tokens so inputs, order details, and buttons are crisp in Light and Dark themes.
4. **Component-Level Dark Tailwind Overrides**: VIP Add-on cards (`CheckoutForm.tsx`), AI Event Concierge panel & Floorplan Schematics (`VenueSeatMap.tsx`), Organizer Studio header button (`Header.tsx`), and Secondary Modals (`MyBookingsModal.tsx`, `NotificationCenter.tsx`, `ReviewModal.tsx`, `WaitlistModal.tsx`, `TicketTransferModal.tsx`, `AdminDashboardModal.tsx`) must adapt cleanly in `html[data-theme='light']` / `.light`.
5. **Backward Compatibility with Existing Tests**: All 17 existing Vitest suites (including `cinema_premiere_animations.test.ts` and `dark_mode_desktop.test.ts`) must continue to pass 100%.

---

### Task 1: Create Automated UI Theme & Contrast Linter (`ui_audit.py`) and Upgrade `ui-ux-pro-max` + `auto-ship`

**Files:**
- Create: `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py`
- Modify: `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/SKILL.md`
- Modify: `/Users/thiennguyen/.gemini/antigravity-cli/skills/auto-ship/SKILL.md`

- [ ] **Step 1: Write `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py`**
  Write a fast, zero-dependency Python 3 static analyzer (`ui_audit.py`) that accepts a frontend directory path (e.g., `python3 ~/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py frontend/src`) and checks:
  1. **Rule 1 (Dark-Background + Theme-Var Text Split-Brain)**: Any CSS rule (outside `:root`/`light` overrides) that sets `background` or `background-color` to a hardcoded dark hex (`#0f0f23`, `#0f172a`, `#1e293b`, `#1b1b30`, `#27273b`, `#020617`) WITHOUT either:
     - an explicit light `color` in the same rule AND no descendant inheriting `var(--text-primary)`, OR
     - a matching `.light <selector>` / `html[data-theme='light'] <selector>` override.
     Specifically flag any rule with hardcoded dark `background: #(0f|1e|1b|27|02)...` that uses `color: var(--text-...)` or lacks an explicit light `color`!
  2. **Rule 2 (Theme-Var Background + Hardcoded White Text Split-Brain)**: Any CSS rule that sets `background: var(--bg-...)` or `background: var(--color-card|muted|background)` alongside hardcoded light text (`color: #fff`, `#ffffff`, `#f8fafc`, `#f1f5f9`, `#e2e8f0`, `#cbd5e1`) without a `.light` override.
  3. **Rule 3 (Unbounded Image Thumbnails)**: Checks `.tsx`/`.jsx` for `<img>` tags without bounded CSS class or inline max-width/width.
  4. Exit code `1` with exact selector/line diagnostics if any violations are found; exit code `0` with `✅ UI Theme & Contrast Audit PASSED` when clean.

- [ ] **Step 2: Run `ui_audit.py` against `frontend/src` (RED — Verify it catches existing bugs)**
  Run:
  ```bash
  python3 /Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src
  ```
  Expected: Fails (RED) flagging `.form-input`, `.payment-method-card`, `.order-summary-section`, `.booking-ref-card`, `.confirmation-details-card`, `.digital-ticket-card`, `.btn-print-ticket`, `.confirmation-dialog-footer`, `.seat-confirm-chip`, `.payment-confirm-details`, etc.

- [ ] **Step 3: Document `ui_audit.py` in `ui-ux-pro-max/SKILL.md` and `auto-ship/SKILL.md`**
  Add a mandatory step in `ui-ux-pro-max/SKILL.md` (under `## Automated Theme & Contrast Gate`) and `auto-ship/SKILL.md` (Phase 3 Step 3) requiring `python3 ~/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py <frontend-src>` to pass with 0 errors before marking any UI task Done.

---

### Task 2: Write Failing Frontend Theme & Contrast Regression Tests (RED)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/test/dark_mode_desktop.test.ts`

- [ ] **Step 1: Add Milestone 36 Light/Dark Semantic Token & Contrast Assertions to `dark_mode_desktop.test.ts`**
  Add a `describe('Milestone 36: Semantic Theme Architecture & Zero Split-Brain Contrast Gate', ...)` block in `frontend/src/test/dark_mode_desktop.test.ts` that reads `frontend/src/index.css`, `VenueSeatMap.tsx`, `CheckoutForm.tsx`, `BookingConfirmation.tsx`, and `Header.tsx` and asserts:
  1. `.form-input`, `.payment-method-card`, `.order-summary-section`, `.booking-ref-card`, `.confirmation-details-card`, `.digital-ticket-card` use semantic theme variables (`var(--bg-surface-elevated)` or `var(--bg-surface)`) instead of hardcoded `#0f0f23` / `#0f172a`.
  2. `.btn-print-ticket`, `.card-section-title`, and `.total-row` use `var(--text-primary)` instead of hardcoded `#ffffff` so they never render white-on-white in Light Mode.
  3. Light mode overrides exist for `.premiere-category-tag`, `.premiere-price-pill`, `.btn-admin-studio`, `.ai-concierge-panel`, `.floorplan-overview-card`, and `.selection-summary-footer`.
  4. Zero CSS rules in `index.css` combine `background: #0f0f23` or `background: #0f172a` with `color: var(--text-primary)` or `background: var(--bg-surface-elevated)` with `color: #ffffff`.

- [ ] **Step 2: Run Vitest to witness the test failure (RED)**
  Run:
  ```bash
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run src/test/dark_mode_desktop.test.ts < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ PASS" || (echo "❌ FAIL:" && grep -iE "(fail|error|expect)" -C 3 /tmp/agy-cmd.log | tail -n 35)
  ```

---

### Task 3: Fix All Split-Brain CSS Rules & Component Classes in TicketVerse (GREEN)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/index.css`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/VenueSeatMap.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/CheckoutForm.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/BookingConfirmation.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/EventCatalog.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/Header.tsx`

- [ ] **Step 1: Convert hardcoded dark surfaces & white text in `frontend/src/index.css` to Semantic Tokens**
  In `frontend/src/index.css`:
  - `.form-input`: change `background: #0f0f23;` -> `background: var(--bg-surface-elevated);` and `box-shadow: var(--shadow-sm);`
  - `.payment-method-card`: change `background: #0f0f23;` -> `background: var(--bg-surface-elevated);`
  - `.order-summary-section`: change `background: #0f0f23;` -> `background: var(--bg-surface-elevated);`
  - `.total-row`: change `color: #ffffff;` -> `color: var(--text-primary);`
  - `.text-accent`: change `color: #fbbf24;` -> `color: var(--color-accent);`
  - `.booking-ref-card`: change `background: #0f172a;` -> `background: var(--bg-surface-elevated);`
  - `.ref-code`: change `color: #38bdf8;` -> `color: var(--color-waitlist);`
  - `.confirmation-details-card`: change `background: #0f172a;` -> `background: var(--bg-surface-elevated);`
  - `.card-section-title`: change `color: #ffffff;` -> `color: var(--text-primary);`
  - `.digital-ticket-card`: change `background: #0f172a;` -> `background: var(--bg-surface-elevated);`
  - `.btn-print-ticket`: change `color: #ffffff;` -> `color: var(--text-primary);`
  - `.confirmation-dialog-message`: change `color: #cbd5e1;` -> `color: var(--text-secondary);`
  - `.confirmation-dialog-footer`: change `background: #0f172a; border-top: 1px solid #334155;` -> `background: var(--bg-surface-elevated); border-top: 1px solid var(--border-color);`
  - `.btn-dialog-cancel`: change `background: #334155; color: #e2e8f0;` -> `background: var(--bg-surface); color: var(--text-primary); border: 1px solid var(--border-color);`
  - `.seat-confirm-chip`: change `background: #1e293b; border: 1px solid #334155;` -> `background: var(--bg-surface-elevated); border: 1px solid var(--border-color);`
  - `.chip-tier`: change `color: #94a3b8;` -> `color: var(--text-muted);`
  - `.seat-confirm-total-bar`: change `color: #e2e8f0; border-top: 1px solid #334155;` -> `color: var(--text-primary); border-top: 1px solid var(--border-color);`
  - `.payment-confirm-details`: change `background: #1e293b; border: 1px solid #334155;` -> `background: var(--bg-surface-elevated); border: 1px solid var(--border-color);`
  - `.selection-summary-footer`: change `background: rgba(30, 41, 59, 0.95);` -> `background: var(--bg-surface-card);`
  - `.selected-count-badge`: change `color: #f8fafc;` -> `color: var(--text-primary);`
  - `.total-amount`: change `color: #38bdf8;` -> `color: var(--color-waitlist);`
  - `.btn-checkout:disabled`: change `background: #27273b; color: #64748b;` -> `background: var(--bg-surface-elevated); color: var(--text-muted); border: 1px solid var(--border-color);`
  - `.seat-booked`: change `background-color: #27273b; color: #64748b; border: 1px solid #1b1b30;` -> `background-color: var(--bg-surface-elevated); color: var(--text-muted); border: 1px solid var(--border-color);`
  - `.btn-reset-filters`: change `background: #1e293b; color: #fda4af; border: 1px solid #334155;` -> `background: rgba(244, 63, 94, 0.12); color: #e11d48; border: 1px solid rgba(244, 63, 94, 0.3);`
  - `.city-facets-label` and `.catalog-summary-bar`: change `color: #94a3b8;` -> `color: var(--text-secondary);`

- [ ] **Step 2: Add Comprehensive Light Mode Overrides in `frontend/src/index.css` for Premiere Showcase, AI Concierge, Floorplan, Add-on Cards, and Modals**
  Add clean Light Mode rules in `frontend/src/index.css`:
  - `.light .premiere-category-tag`: `background: #e0e7ff; border-color: #818cf8; color: #312e81;` (high contrast deep indigo on soft lavender)
  - `.light .premiere-price-pill`: `background: #d1fae5; border-color: #34d399; color: #065f46;` (deep emerald on mint)
  - `.light .premiere-showcase-meta`: `color: #334155;`
  - `.light .btn-admin-studio`: `background: #f3e8ff !important; border-color: #c084fc !important; color: #6b21a8 !important;`
  - `.light .selection-summary-footer`: `background: rgba(255, 255, 255, 0.96); border-color: #cbd5e1; box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);`
  - `.light .ai-concierge-panel`: `background: linear-gradient(135deg, #ffffff 0%, #faf5ff 50%, #f3e8ff 100%) !important; border-color: #c084fc !important; color: #0f172a !important; box-shadow: 0 8px 20px rgba(107, 33, 168, 0.08) !important;`
  - `.light .ai-concierge-card`: `background: #ffffff !important; border-color: #e9d5ff !important; color: #1e293b !important; box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);`
  - `.light .floorplan-overview-card`: `background: #f8fafc !important; border-color: #cbd5e1 !important; color: #0f172a !important;`
  - `.light .floorplan-section-card`: `background: #ffffff !important; border-color: #cbd5e1 !important; color: #0f172a !important;`
  - `.light .addon-upsell-card`: `background: #f8fafc !important; border-color: #cbd5e1 !important; color: #0f172a !important;`
  - `.light .addon-upsell-card.is-selected`: `background: #fffbeb !important; border-color: #f59e0b !important;`
  - **Universal Light-Mode Tailwind Surface Adapter**: For secondary modals (`AdminDashboardModal`, `MyBookingsModal`, `NotificationCenter`, `ReviewModal`, `TicketPassModal`, `TicketTransferModal`, `WaitlistModal`) that use Tailwind dark utilities (`bg-slate-900`, `bg-slate-950`, `bg-slate-800`, `text-slate-100`, `text-slate-200`, `text-slate-300`, `border-slate-800`, `border-slate-700`), add scoped `.light` surface/text remappings in `index.css` so every modal, table, and drawer automatically renders in crisp Light Mode without breaking dark mode or poster banners!

- [ ] **Step 3: Polish Component Classes in `VenueSeatMap.tsx`, `CheckoutForm.tsx`, `BookingConfirmation.tsx`, `EventCatalog.tsx`**
  - In `VenueSeatMap.tsx`:
    - Add class `ai-concierge-panel` and generous padding (`p-5 mx-4 my-3`) to `[data-testid="ai-concierge-panel"]`, and `ai-concierge-card p-3.5` to the 3 inner recommendation cards + insider tips box so text never touches the rounded border (`Screenshot 1`).
    - Add class `floorplan-overview-card` to `[data-testid="floorplan-overview-card"]` and `floorplan-section-card` to its inner cards.
  - In `CheckoutForm.tsx`:
    - Add class `addon-upsell-card` (and `is-selected` when `qty > 0`) to `[data-testid^="addon-item-"]` and replace hardcoded dark text colors inside add-on cards and corporate billing box with semantic classes (`Screenshot 3`).
  - In `BookingConfirmation.tsx`:
    - Update the invoice/calendar `.btn-print-ticket` buttons so they don't hardcode `text-cyan-300` / `text-amber-300` (which are invisible on light buttons in Light Mode; use `.btn-invoice-action` / `.btn-calendar-action` with crisp dark/light contrast) (`Screenshot 4`).
  - In `EventCatalog.tsx`:
    - Change the featured premiere date span (line 201) from `text-slate-300` to `premiere-date-tag` with dark/light support (`Screenshot 2`).

- [ ] **Step 4: Run `ui_audit.py`, Vitest Suite, and TypeScript Compiler (GREEN)**
  Run:
  ```bash
  python3 /Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ PASS" || (echo "❌ FAIL:" && tail -n 35 /tmp/agy-cmd.log)
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install -p typescript tsc --noEmit --pretty false < /dev/null > /tmp/agy-tsc.log 2>&1 && echo "✅ TSC PASS" || (echo "❌ TSC FAIL:" && tail -n 35 /tmp/agy-tsc.log)
  ```
