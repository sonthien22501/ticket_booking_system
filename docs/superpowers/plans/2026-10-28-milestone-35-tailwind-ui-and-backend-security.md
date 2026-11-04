# Milestone 35: Tailwind CSS Engine, Decluttered UI Layout & Backend OWASP Security Hardening

**Goal**:
1. **Part A — Frontend Tailwind CSS Integration, Decluttered Toolbars & Seat Map Layout (`frontend/`)**:
   - Install and wire `@tailwindcss/vite` + `tailwindcss` in `frontend/package.json`, `frontend/vite.config.ts`, and `frontend/src/index.css` (`@import "tailwindcss";` at top) so all 1,000+ utility classes (`w-5`, `h-5`, `flex`, `items-center`, `gap-2`, `sr-only`, `px-3`, `rounded-xl`, modals, etc.) across all 23 components compile into real CSS rules.
   - **Fix `VenueSeatMap.tsx` & `index.css` Layout Bugs**:
     - Constrain the iTunes album artwork image in `VenueSeatMap.tsx` (`className="mashup-artwork-thumb w-5 h-5 rounded object-cover shadow-sm"`) with explicit `width: 22px; height: 22px; max-width: 22px; max-height: 22px; object-fit: cover; border-radius: 4px; flex-shrink: 0;` in `index.css` so it NEVER explodes to 300x300px even if an external stylesheet overrides img rules.
     - Format the Mash-Up Intelligence Bar (`.mashup-intel-bar`), Weather Badge (`.mashup-weather-pill`), Audio Preview Pill (`.mashup-audio-pill`), and AI Concierge Toggle (`.btn-ai-concierge`) into a single, sleek, compact horizontal bar with proper flex spacing between temperature (`13°C / 55°F`), condition (`Partly Cloudy`), and rain probability (`(0% rain)`).
     - Format the Section Switcher bar (`.seatmap-section-toolbar`) and section filter buttons (`.section-filter-pill`, `.section-filter-pill.active`, `.btn-floorplan-toggle`) so `Section: All Sections | VIP Front (Rows A-B) | General Admission (Rows C-F)` and `Floorplan Schematics` sit in a clean horizontal segmented bar with crisp spacing and light/dark mode contrast.
   - **Declutter `EventCatalog.tsx` Search & Filter Bar (Subtraction by Default)**:
     - Keep all existing `data-testid` attributes (`catalog-search-input`, `catalog-sort-select`, `reset-filters-btn`, `category-pill-*`, `city-facets-bar`, `city-pill-*`, `price-pill-*`) 100% intact, while styling `.search-and-filter-bar` into a clean, compact glassmorphic control panel where the search input and sort dropdown sit side-by-side on 1 row, category pills sit cleanly below, and Location/Budget facets sit in a single unified footer row inside the panel.

2. **Part B — Backend OWASP & Zero-Trust Security Hardening (`backend/`)**:
   - **Fix 1: Prevent Public Role Escalation in `backend/auth-service/main.go`**:
     - Extract a pure helper `sanitizeRegistrationRole(requestedRole string, adminHeaderToken string, validAdminSecret string) string` in `backend/auth-service/main.go`.
     - By default, public registration (`POST /api/auth/register`) MUST always assign `"customer"`, ignoring `"role": "admin"` in untrusted JSON bodies unless the request includes a valid `X-Admin-Setup-Token` matching `ADMIN_SETUP_SECRET` (or a valid Admin Bearer JWT).
     - Add unit test `TestSanitizeRegistrationRole_PreventsPublicAdminEscalation` in `backend/auth-service/main_test.go`.
   - **Fix 2: Prevent Unauthenticated Booking Enumeration / Header Spoofing in `backend/booking-service/main.go`**:
     - Extract a helper `resolveAuthorizedBookingFilter(queryUserID, queryEmail, headerUserID string, jwtClaims *Claims) (userID string, email string, isAdmin bool)` in `backend/booking-service/main.go`.
     - When a valid JWT is present (`jwtClaims != nil`) and `jwtClaims.Role != "admin"`, enforce that non-admin authenticated users can only query their own `jwtClaims.UserID` / `jwtClaims.Email` (preventing them from passing `?userId=someone_else` or spoofing `X-User-ID`).
     - When no JWT is present, strip untrusted `X-User-ID` header spoofing if it conflicts with explicit query params, and sanitize inputs.
     - Add unit test `TestResolveAuthorizedBookingFilter_PreventsCrossUserEnumeration` in `backend/booking-service/main_test.go`.
   - **Fix 3: Harden Rate-Limit Bypass Key in `backend/api-gateway/nginx.conf`**:
     - In `backend/api-gateway/nginx.conf`, restrict `$http_x_rate_limit_bypass` so arbitrary `"true"` strings from public bots do not bypass rate limiting unless it matches internal test key `"internal-e2e-harness"` (or loopback), and update `scripts/verify_e2e.py` default headers if needed.

---

## Step-by-Step TDD Execution Instructions for Workers

### Worker 1 (Frontend Tailwind + Decluttered UI & Seat Map Polish)
1. **Step 1 (RED)**: Add regression tests in `frontend/src/test/cinema_premiere_animations.test.ts` under `describe('6. Milestone 35: Seat Map Mashup Bar, Bounded Artwork Thumbnail & Section Switcher Layout', ...)` asserting:
   - `.mashup-intel-bar`, `.mashup-weather-pill`, `.mashup-audio-pill`, `.mashup-artwork-thumb` (`max-width: 22px; max-height: 22px`), `.seatmap-section-toolbar`, and `.section-filter-pill` are defined in `frontend/src/index.css`.
   - Run `cd frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run src/test/cinema_premiere_animations.test.ts` to witness RED failure.
2. **Step 2 (GREEN)**:
   - Install `@tailwindcss/vite` and `tailwindcss` in `frontend/` (`npm install -D tailwindcss @tailwindcss/vite`), add `tailwindcss()` plugin to `frontend/vite.config.ts`, and add `@import "tailwindcss";` at the top of `frontend/src/index.css` (ensuring existing custom CSS rules follow it so they take precedence).
   - Update `frontend/src/components/VenueSeatMap.tsx` to add semantic classes:
     - `mashup-intel-bar` on the Mash-Up bar container
     - `mashup-weather-pill` on the weather badge (with explicit spacing between temperature, condition, and rain %)
     - `mashup-audio-pill` on the iTunes preview container and `mashup-artwork-thumb` on the `<img>` tag (`width={22} height={22}`)
     - `btn-ai-concierge` on the AI Event Concierge button
     - `seatmap-section-toolbar` on the Section Switcher bar, `section-filter-pill ${selectedSection === sec.id ? 'active' : ''}` on each section button, and `btn-floorplan-toggle` on the Floorplan Schematics button.
   - Add explicit, rock-solid CSS rules in `frontend/src/index.css` (including Light Mode overrides) for `.mashup-intel-bar`, `.mashup-weather-pill`, `.mashup-audio-pill`, `.mashup-artwork-thumb`, `.btn-ai-concierge`, `.seatmap-section-toolbar`, `.section-filter-pill`, and `.btn-floorplan-toggle`, plus compact single-row styling for `.catalog-search-row` in `EventCatalog.tsx`.
3. **Step 3 (Verify)**:
   - Run `cd frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run`
   - Run `cd frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install -p typescript tsc --noEmit --pretty false`
   - Run `cd frontend && CI=true GIT_TERMINAL_PROMPT=0 npm run build` to confirm Vite + Tailwind production bundle compiles cleanly!

### Worker 2 (Backend OWASP Security Hardening)
1. **Step 1 (RED)**:
   - Add `TestSanitizeRegistrationRole_PreventsPublicAdminEscalation` in `backend/auth-service/main_test.go` testing that `sanitizeRegistrationRole("admin", "", "secret123")` returns `"customer"`, while `sanitizeRegistrationRole("admin", "secret123", "secret123")` returns `"admin"`.
   - Add `TestResolveAuthorizedBookingFilter_PreventsCrossUserEnumeration` in `backend/booking-service/main_test.go` testing that a non-admin JWT claim (`UserID: "usr-alice", Role: "customer"`) cannot query `queryUserID: "usr-bob"` or `queryEmail: "bob@test.com"`.
   - Run `go test -timeout 30s ./...` in `backend/auth-service` and `backend/booking-service` to witness RED failure.
2. **Step 2 (GREEN)**:
   - Implement `sanitizeRegistrationRole` in `backend/auth-service/main.go` and wire it into `handleRegister` (checking `r.Header.Get("X-Admin-Setup-Token")` against `getEnv("ADMIN_SETUP_SECRET", "ticketverse-internal-admin-secret")`).
   - Implement `resolveAuthorizedBookingFilter` in `backend/booking-service/main.go` and wire it into `listBookings`.
3. **Step 3 (Verify)**:
   - Run `go test -timeout 30s ./...` in `backend/auth-service` and `backend/booking-service` and format with `gofmt -s -w`.
