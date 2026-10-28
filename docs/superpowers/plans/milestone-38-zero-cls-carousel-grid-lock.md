# Milestone 38: Zero-CLS Carousel & Grid Card Alignment + Rule 7 Layout Shift Gate

**Goal:** Eliminate the 6-second Cumulative Layout Shift (CLS) in the Featured Premiere Spotlight Carousel AND align all catalog grid cards + filter summary bar, while upgrading `ui_audit.py` with **Rule 7 (Zero-CLS Carousel & Card Badge Row Lock)** so dynamic badges/titles can never cause layout jumping again.

**Root-Cause Diagnosis of the 2 Screenshots:**
1. **Why the Featured Premiere Carousel Box Changes Height Every 6 Seconds (Pushing the Whole Page Up and Down)**:
   - In `EventCatalog.tsx` line 181: `<div className="flex items-center gap-2 flex-wrap mb-2">` contains `.premiere-spotlight-pill`, `.premiere-category-tag`, and `.premiere-venue-tag`.
   - On Slide #1 (`Neon Symphony 2026`, category `MUSIC & CONCERTS`), all 3 tags fit on **1 horizontal row** (`height: 24px`).
   - On Slide #2 (`Global Tech Summit 2026`, category `TECHNOLOGY & CONFERENCES` + `Metropolitan Convention Center (San Francisco, USA)`), `flex-wrap` wraps the venue tag onto a **2nd row** (`height: 54px`), growing `.premiere-showcase-card` by `+30px` and pushing the search bar, filter pills, and event grid down by 30px every 6 seconds!
   - **Fix**: Give the badge row a class `.premiere-badges-row` with `display: flex; align-items: center; gap: 0.5rem; flex-wrap: nowrap; overflow: hidden;` and make `.premiere-venue-tag` `min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;` (and `.premiere-spotlight-pill`, `.premiere-category-tag` `flex-shrink: 0; white-space: nowrap;`). Every slide's header row is now strictly 1 row (`24px`), descriptions are locked at `2.5rem`, titles are `nowrap` (`1.35rem`), and `.premiere-showcase-card` has a rock-solid constant height (`172px`) with **zero layout shift** and **zero clipping**!
2. **Bonus Visual Glitches in the Same Screenshots**:
   - **Catalog Summary Bar (`Showing 9 of 9 live eventsBudget: Any`)**: Because `.search-and-filter-bar` has `align-items: center;`, `.catalog-summary-bar` lacked `width: 100%;` and shrunk to fit its content, mashing `"Showing 9 of 9 live events"` directly into `"Budget:"` with 0px gap! Adding `width: 100%; gap: 1rem; flex-wrap: wrap;` fixes it immediately.
   - **Event Grid Cards (`Neon Symphony 2026` 1-line title vs `UEFA Champions Gala Match: Berlin vs Madrid` 2-line title)**: In `index.css` line 1236, `.event-title` had no `min-height` or `line-clamp`, so 1-line titles shifted the description and date rows higher than 2-line titles across the same row. Locking `.event-title` to `display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; min-height: 2.6em;` aligns every card in the grid horizontally!

---

## Review Focus (Top 5 Failure Modes)
1. **Zero-CLS Premiere Badge Row**: `.premiere-badges-row` must enforce `flex-wrap: nowrap; overflow: hidden;` with `flex-shrink: 0` on pills and `text-overflow: ellipsis; white-space: nowrap; min-width: 0;` on `.premiere-venue-tag`.
2. **Catalog Summary Bar Full Width**: `.catalog-summary-bar` must have `width: 100%; gap: 1rem;` so `"Showing X of Y live events"` and `"Budget:"` are separated across the full 720px bar.
3. **Uniform Grid Card Titles**: `.event-title` must have `-webkit-line-clamp: 2` and `min-height: 2.6em` so 1-line and 2-line event titles align across the grid.
4. **Upgrade `ui_audit.py` with Rule 7 (Carousel / Showcase CLS Trap Detector)**: Flag any `flex-wrap` inside a carousel/showcase card (`premiere-showcase`, `carousel`, `slide`) that isn't height-locked, and flag `.event-title` / `.premiere-showcase-title` if missing `line-clamp` or `nowrap`.
5. **100% Test & Build Verification**: `ui_audit.py`, `vitest run`, and `tsc --noEmit` must pass 100%.

---

### Task 1: Upgrade `ui_audit.py` with Rule 7 (Zero-CLS Carousel & Card Title Alignment Gate) (RED)

**Files:**
- Modify: `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py`
- Modify: `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/SKILL.md`

- [ ] **Step 1: Add Rule 7 to `ui_audit.py`**
  Update `/Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py` to check:
  - **Rule 7a (Carousel / Showcase Unbounded `flex-wrap` CLS Trap)**: Scan `.tsx`/`.jsx` for any element inside a showcase/carousel container (`premiere-showcase` or `carousel`) that uses `flex-wrap` without an explicit fixed `h-` / `min-h-` height lock (because wrapping badges between slides causes Cumulative Layout Shift every tick!).
  - **Rule 7b (Card Title Line-Height Stability)**: In CSS, verify `.event-title` defines `min-height` and `-webkit-line-clamp` (or `white-space: nowrap`) so adjacent grid cards never misalign when titles wrap to 2 lines.

- [ ] **Step 2: Run `ui_audit.py` against `frontend/src` (RED)**
  Run:
  ```bash
  python3 /Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src
  ```
  Expected: Fails (RED) flagging `flex-wrap` in `EventCatalog.tsx` premiere showcase and `.event-title` in `index.css`.

---

### Task 2: Add Milestone 38 Regression Tests to `cinema_premiere_animations.test.ts` (RED)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/test/cinema_premiere_animations.test.ts`

- [ ] **Step 1: Add Milestone 38 Zero-CLS & Alignment Assertions**
  Add a `describe('7. Milestone 38: Zero-CLS Carousel Badge Row, Full-Width Filter Summary & Grid Title Lock', ...)` block in `frontend/src/test/cinema_premiere_animations.test.ts` asserting:
  1. `.premiere-badges-row` enforces `flex-wrap: nowrap` and `overflow: hidden`, and `.premiere-venue-tag` enforces `white-space: nowrap`, `overflow: hidden`, and `text-overflow: ellipsis` so long category/venue names never wrap to a 2nd line or change `.premiere-showcase-card` height between slides.
  2. `EventCatalog.tsx` uses `premiere-badges-row` (without `flex-wrap`) in the Featured Premiere header.
  3. `.catalog-summary-bar` defines `width: 100%` and `gap:` so `"Showing X of Y live events"` and `"Budget:"` never collide.
  4. `.event-title` defines `-webkit-line-clamp: 2` and `min-height` so 1-line and 2-line event titles align across the catalog grid.

- [ ] **Step 2: Run Vitest to witness the failure (RED)**
  Run:
  ```bash
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run src/test/cinema_premiere_animations.test.ts < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ PASS" || (echo "❌ FAIL:" && tail -n 35 /tmp/agy-cmd.log)
  ```

---

### Task 3: Fix `EventCatalog.tsx` and `frontend/src/index.css` for Zero-CLS & Crisp Alignment (GREEN)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/EventCatalog.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/index.css`

- [ ] **Step 1: Update `EventCatalog.tsx` Premiere Badges Row**
  In `frontend/src/components/EventCatalog.tsx` (line 181):
  - Change `<div className="flex items-center gap-2 flex-wrap mb-2">` to `<div className="premiere-badges-row mb-2">`.
  - Wrap the venue text inside `.premiere-venue-tag` or keep `<span className="premiere-venue-tag" title={`${currentFeatured.venueName} (${currentFeatured.venueLocation})`}>` so hovering shows the full venue if truncated on narrow screens.

- [ ] **Step 2: Update `frontend/src/index.css` for `.premiere-badges-row`, `.premiere-venue-tag`, `.catalog-summary-bar`, and `.event-title`**
  In `frontend/src/index.css`:
  - Add `.premiere-badges-row`:
    ```css
    .premiere-badges-row {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      flex-wrap: nowrap;
      overflow: hidden;
      min-height: 1.65rem;
      max-height: 1.65rem;
    }
    ```
  - Update `.premiere-spotlight-pill` and `.premiere-category-tag` to include `flex-shrink: 0; white-space: nowrap;`.
  - Update `.premiere-venue-tag`:
    ```css
    .premiere-venue-tag {
      display: inline-flex;
      align-items: center;
      font-size: 0.75rem;
      color: #cbd5e1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    ```
  - Update `.catalog-summary-bar` (around line 846):
    Add `width: 100%; gap: 1rem; flex-wrap: wrap;` so `"Showing 9 of 9 live events"` and `"Budget:"` sit cleanly at opposite ends of the 720px bar.
  - Update `.city-facets-bar` (around line 800):
    Add `width: 100%; justify-content: center;` so the city pills bar aligns cleanly with the search bar.
  - Update `.event-title` (around line 1236):
    ```css
    .event-title {
      font-family: var(--font-heading);
      font-size: 1.35rem;
      font-weight: 700;
      margin-bottom: 0.5rem;
      line-height: 1.3;
      min-height: 2.6em;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
      color: var(--text-primary);
    }
    ```

- [ ] **Step 3: Run `ui_audit.py`, Vitest Suite, and TypeScript Compiler (GREEN)**
  Run:
  ```bash
  python3 /Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ VITEST PASS" || (echo "❌ FAIL:" && tail -n 35 /tmp/agy-cmd.log)
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install -p typescript tsc --noEmit --pretty false < /dev/null > /tmp/agy-tsc.log 2>&1 && echo "✅ TSC PASS" || (echo "❌ TSC FAIL:" && tail -n 35 /tmp/agy-tsc.log)
  ```
