# Milestone 34.1: Header Pill Formatting, Icon Baseline Alignment, Light Theme Contrast & Fixed-Height Premiere Showcase

**Goal**: Resolve all 4 visual & responsive polish bugs identified during live browser inspection:
1. **USD Currency Select & Header Action Buttons (`Header.tsx` & `index.css`)**:
   - Hide the `<label htmlFor="currency-select" className="sr-only">` off-screen properly by defining `.sr-only` in `index.css` (`position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0,0,0,0)`).
   - Format `.currency-selector-pill`, `.currency-select-dropdown`, `.btn-my-tickets`, `.btn-notifications`, `.btn-admin-studio`, and `.notification-badge-counter` with explicit CSS rules in `index.css` so they never render as unstyled native OS controls.
2. **Icon-to-Text Vertical Baseline Alignment & Filter Pill Spacing (`EventCatalog.tsx`, `EventCard.tsx`, `Header.tsx`, `index.css`)**:
   - Remove `.inline-icon { vertical-align: -2px; }` or ensure all flex/inline-flex containers use `display: inline-flex; align-items: center; gap: 0.4rem; line-height: 1` and `svg { flex-shrink: 0; display: block; }` so icons sit dead-center with their text labels.
   - Replace Tailwind-only layout classes in `EventCatalog.tsx` (search + sort toolbar, city facet pills, budget filter pills, and sort `<select>`) with explicit semantic CSS classes in `index.css` (`.catalog-search-row`, `.catalog-sort-select`, `.city-facets-bar`, `.city-pill`, `.catalog-summary-bar`, `.budget-pills-group`, `.budget-pill`) so `Cities:` and `Budget:` pills never mash together without spacing (`All LocationsBerlinSan Francisco...`).
3. **Light Theme Contrast & Visibility (`index.css`)**:
   - Override `.site-header` in `html[data-theme='light'], body.light, .light` to use a crisp frosted light surface (`rgba(255, 255, 255, 0.92)`) with dark slate text (`#0f172a`) and visible borders (`#cbd5e1`).
   - Override `.brand-title` and `.hero-title` in light mode to use a high-contrast deep indigo-to-amber gradient (`linear-gradient(135deg, #1e1b4b 20%, #b45309 85%)`) instead of `#ffffff` on white!
   - Override `.premiere-showcase-card`, `.premiere-live-ribbon`, `.event-scarcity-widget`, and `.event-card-footer` in light mode so every text element meets WCAG AA contrast ($\ge 4.5:1$).
4. **Fixed-Height Premiere Showcase Box (`EventCatalog.tsx` & `index.css`)**:
   - Lock `.premiere-showcase-card` to a consistent height (`min-height: 168px; height: 168px` on desktop) and clamp `.premiere-showcase-title` (`white-space: nowrap; overflow: hidden; text-overflow: ellipsis`) and `.premiere-showcase-desc` (`display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; height: 2.6rem`) so cycling between short and long event titles/descriptions NEVER changes the height or dimensions of the wrapper card.

---

## Step-by-Step TDD Plan for Flash Worker

### Step 1 (RED): Add Regression Tests in `frontend/src/test/cinema_premiere_animations.test.ts`
Add a new `describe('5. Milestone 34.1 UI Polish: Alignment, Light Theme Contrast & Fixed Showcase Box', ...)` block in `frontend/src/test/cinema_premiere_animations.test.ts` that reads `frontend/src/index.css` (via Node `fs.readFileSync`) and verifies:
1. `.sr-only` is defined with `position: absolute` and `width: 1px` so `Select Currency` label is hidden visually while accessible to screen readers.
2. `.currency-selector-pill`, `.btn-my-tickets`, `.btn-notifications`, and `.btn-admin-studio` are defined in `index.css` with `display: inline-flex` and `align-items: center`.
3. `.city-facets-bar`, `.city-pill`, `.budget-pill`, and `.catalog-sort-select` are defined in `index.css` with explicit gap/padding so filter pills have proper spacing.
4. Light mode overrides (`html[data-theme='light'] .hero-title` and `html[data-theme='light'] .site-header`) exist in `index.css` for high contrast.
5. `.premiere-showcase-card` defines fixed height/min-height stability and `.premiere-showcase-desc` clamps to 2 lines (`-webkit-line-clamp: 2`) so the showcase box never jumps in size.

Run `cd frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run src/test/cinema_premiere_animations.test.ts` and confirm it FAILS (RED).

### Step 2 (GREEN): Update `Header.tsx`, `EventCatalog.tsx`, and `index.css`
1. In `frontend/src/components/Header.tsx`:
   - Add semantic classes `currency-select-dropdown` on the `<select id="currency-select">`, and `notification-badge-counter` on the unread badge `<span>`.
2. In `frontend/src/components/EventCatalog.tsx`:
   - Add semantic classes alongside existing classes and `data-testid` attributes:
     - `catalog-search-row` on the search + sort wrapper `<div>`.
     - `catalog-sort-select` on the sort `<select>`.
     - `btn-reset-filters` on the Clear button.
     - `city-facets-bar` on the Cities container, `city-facets-label` on the `Cities:` span, and `city-pill ${selectedCity === city ? 'active' : ''}` on each city button.
     - `catalog-summary-bar` on the `Showing X of Y live events` row, `budget-pills-group` on the budget wrapper, and `budget-pill ${selectedPriceRange === b.id ? 'active' : ''}` on each budget button.
3. In `frontend/src/index.css`:
   - Add `.sr-only` utility class.
   - Add explicit styles for `.currency-selector-pill`, `.currency-select-dropdown`, `.btn-my-tickets`, `.btn-notifications`, `.notification-badge-counter`, and `.btn-admin-studio` with `display: inline-flex; align-items: center; justify-content: center; gap: 0.45rem; line-height: 1;`.
   - Fix `.inline-icon` and SVG alignment across badges/buttons (`.premiere-live-ribbon`, `.ribbon-metric`, `.premiere-spotlight-pill`, `.premiere-venue-tag`, `.premiere-showcase-meta`, `.event-category-badge`, `.event-meta-item`, `.btn-select-seats`) so every icon + text pair uses `display: inline-flex; align-items: center; vertical-align: middle; line-height: 1;`.
   - Add explicit layout & pill styles for `.catalog-search-row`, `.catalog-sort-select`, `.btn-reset-filters`, `.city-facets-bar`, `.city-facets-label`, `.city-pill`, `.catalog-summary-bar`, `.budget-pills-group`, and `.budget-pill`.
   - Lock `.premiere-showcase-card` height (`min-height: 172px; height: 172px;` on desktop `@media (min-width: 768px)`), set `.premiere-showcase-title` to single-line truncation (`white-space: nowrap; overflow: hidden; text-overflow: ellipsis;`), and lock `.premiere-showcase-desc` to exact 2-line height (`display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; min-height: 2.5rem; max-height: 2.5rem; line-height: 1.25rem;`).
   - Add complete Light Mode overrides (`html[data-theme='light']`, `body.light`, `.light`) for `.site-header`, `.brand-title`, `.hero-title`, `.premiere-live-ribbon`, `.premiere-showcase-card`, `.premiere-showcase-title`, `.premiere-showcase-desc`, `.event-card-footer`, `.event-scarcity-widget`, `.city-pill`, `.budget-pill`, and `.catalog-sort-select`.

### Step 3: Verify Tests, TypeScript & Commit
1. Run `cd frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run` (must pass 100%).
2. Run `cd frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install -p typescript tsc --noEmit --pretty false` (must pass 0 errors).
3. Stage and commit cleanly (`git commit -m "fix(ui): polish header pills, icon alignment, light theme contrast & fixed showcase height"`).
