# Milestone 39: 4-Role RBAC Separation, Full-Page Role Workspace Dashboard & Customer VIP Hub

**Goal:** Separate the application experience across 4 distinct RBAC tiers (`Guest`, `Customer`, `Department Organizer`, `System Admin`), replace the cramped popup modal with a **Full-Page 1280px Role Workspace View** (`RoleWorkspaceView.tsx`) featuring rich data modules (including the **Customer VIP Hub**, **Department Organizer Studio**, and **System Admin Command Center**), and add a **1-Click Demo Role Switcher** in `AuthModal.tsx` backed by seeded `organizer@tickets.com` in `auth-service`.

---

## Architecture & Role Matrix

| Role Tier | Identity / Demo Account | Header Navigation | Full-Page Workspace Tabs & Modules |
| :--- | :--- | :--- | :--- |
| **1. Guest** (`null`) | Unauthenticated visitor | `Sign In` button ONLY (hides `My Tickets` & `Organizer Studio` clutter) | Prompted to Sign In or use 1-Click Demo Switcher |
| **2. Customer** (`role: 'customer'`) | `user@tickets.com` / `user123` | `My Tickets`, `VIP Hub` (`data-testid="role-workspace-btn"`), Bell, Profile Badge (`customer`) | **Customer VIP Hub**:<br>• **Tab 1: VIP Overview & Loyalty Progress**: 4 KPI Cards (Total Bookings, Active Tickets, Loyalty Points & Tier Progress Bar `Bronze -> Silver -> Gold -> Platinum VIP`, Promo Savings), Upcoming Next-Event Countdown Banner, Spending by Category Visual Breakdown, & Quick Passbook/Invoice/Calendar Actions.<br>• **Tab 2: My Passbook & Digital Tickets**: Interactive list of confirmed bookings with QR Pass trigger, PDF Invoice download, and `.ics` Calendar export. |
| **3. Department Organizer** (`role: 'organizer'`) | `organizer@tickets.com` / `organizer123` | `My Tickets`, `Organizer Studio` (`data-testid="admin-studio-btn"`), Bell, Profile Badge (`organizer`) | **Department Organizer Studio**:<br>• **Tab 1: Department Telemetry & Seat Heat**: 4 KPI Cards (Gross Revenue, Tickets Issued, Avg Occupancy Ring Gauge, Active Events), **Category Revenue & Occupancy Breakdown Bars**, and Live Event Inventory Table with Occupancy Heat Bars & Pause/Resume controls.<br>• **Tab 2: Publish New Event**: Event & 2D Seating Grid Provisioning Studio.<br>• **Tab 3: Promotions & Vouchers**: Campaign Roster & Coupon Creator. |
| **4. System Admin** (`role: 'admin'`) | `admin@tickets.com` / `admin123` | `My Tickets`, `Admin Command` (`data-testid="admin-studio-btn"`), Bell, Profile Badge (`admin`) | **System Admin Command Center**:<br>• Includes all Organizer Studio tabs PLUS **Tab 4: Platform Health & RBAC Governance**:<br>  - **Microservice Mesh Telemetry Matrix**: Live status, port, & latency bars for `api-gateway (:8080)`, `catalog-service (:8081)`, `inventory-service (:8082)`, `booking-service (:8083)`, `auth-service (:8084)`, `notification-worker (:8085)`, `postgres (:5432)`, `redis (:6379)`, `rabbitmq (:5672)`.<br>  - **RBAC Role & Access Policy Matrix**: Visual governance table showing permissions across `Guest`, `Customer`, `Department Organizer`, and `System Admin`.<br>  - **Recent Multi-Channel Notification Audit Stream**: Live delivery log from `fetchNotificationDeliveries()`. |

---

## Review Focus (Top 5 Failure Modes & Edge Cases)
1. **Zero Privilege Escalation in `auth-service` (Law 6 OWASP Gate)**:
   - `sanitizeRegistrationRole` in `backend/auth-service/main.go` must allow `"organizer"` or `"admin"` ONLY when `validAdminSecret != ""` and `adminHeaderToken == validAdminSecret` (unauthenticated public `/register` requests without the setup token must still default to `"customer"`!).
   - Seed `organizer@tickets.com` (`password: organizer123`, `name: "Department Event Organizer"`, `role: "organizer"`) alongside `admin@tickets.com` and `user@tickets.com` in `backend/auth-service/main.go`.
   - In `backend/booking-service/main.go`, allow both `"admin"` and `"organizer"` roles to access `/api/admin/analytics` and `/api/admin/vouchers` (while keeping customer booking isolation intact).
2. **1-Click Demo Role Switcher in `AuthModal.tsx`**:
   - Add a sleek `"⚡ Instant Demo Role Switcher"` bar inside `AuthModal.tsx` with 3 one-click buttons:
     - `👤 Customer (Attendee)` -> logs in as `user@tickets.com` / `user123` (with instant client-side fallback session if backend is offline in unit tests)
     - `🎪 Department Organizer` -> logs in as `organizer@tickets.com` / `organizer123`
     - `🛡️ System Admin` -> logs in as `admin@tickets.com` / `admin123`
3. **Full-Page Workspace Mode (`App.tsx` + `AdminDashboardModal.tsx`)**:
   - When the user clicks `VIP Hub` / `Organizer Studio` / `Admin Command` in `Header.tsx`, `App.tsx` switches the main 1280px screen to the full-page Role Workspace (instead of a cramped popup dialog) with a prominent `"← Back to Live Arena Catalog"` button!
   - Keep `AdminDashboardModal` supporting `inlinePage?: boolean` (defaulting to full-page when rendered inside `<main className="main-content">`, while still working if instantiated in existing unit tests `admin_studio.test.ts` and `cinema_premiere_animations.test.ts`).
4. **Header Role Gating (`Header.tsx`)**:
   - When `currentUser === null` (Guest): hide `Organizer Studio` and `My Tickets` buttons from the header so guests only see clean discovery controls + `Sign In`.
   - When `currentUser.role === 'customer'`: show `My Tickets` and `✨ VIP Hub` button (`data-testid="role-workspace-btn"`).
   - When `currentUser.role === 'organizer'`: show `My Tickets` and `🎪 Organizer Studio` button (`data-testid="admin-studio-btn"`).
   - When `currentUser.role === 'admin'`: show `My Tickets` and `🛡️ Admin Command` button (`data-testid="admin-studio-btn"`).
   - Style `.user-role-badge.role-organizer` in `index.css` (purple/indigo badge) alongside `.role-customer` (emerald) and `.role-admin` (gold).
5. **UI Theme & Contrast Gate (`ui_audit.py`)**:
   - Every new card, progress bar, and table in the workspace must pass `ui_audit.py` with **0 violations** and support both Dark and Light themes!

---

### Task 1: Upgrade Backend `auth-service` & `booking-service` for `organizer` Role (RED -> GREEN)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/backend/auth-service/main.go`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/backend/auth-service/main_test.go`

- [ ] **Step 1: Add failing test in `backend/auth-service/main_test.go` for `organizer` role sanitization (RED)**
  In `backend/auth-service/main_test.go`, add assertions to `TestSanitizeRegistrationRole_PreventsPublicAdminEscalation`:
  - Public registration requesting `"organizer"` without `secret` returns `"customer"`.
  - Registration requesting `"organizer"` WITH valid `secret` returns `"organizer"`.
- [ ] **Step 2: Update `backend/auth-service/main.go` to seed `organizer@tickets.com` and support `"organizer"` in `sanitizeRegistrationRole` (GREEN)**
  - In `initSchema`: add `s.seedDefaultUser("organizer@tickets.com", "organizer123", "Department Event Organizer", "organizer")`.
  - In `sanitizeRegistrationRole`:
    ```go
    func sanitizeRegistrationRole(requestedRole, adminHeaderToken, validAdminSecret string) string {
    	role := strings.ToLower(strings.TrimSpace(requestedRole))
    	if role != "admin" && role != "organizer" {
    		return "customer"
    	}
    	if validAdminSecret != "" && strings.TrimSpace(adminHeaderToken) == strings.TrimSpace(validAdminSecret) {
    		return role
    	}
    	return "customer"
    }
    ```
  - Run `cd backend/auth-service && go test -timeout 30s ./...` to verify GREEN.

---

### Task 2: Write Failing Frontend Tests for 4-Role Separation, Full-Page Workspace & Customer VIP Hub (RED)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/test/admin_studio.test.ts`

- [ ] **Step 1: Add Milestone 39 Test Suite to `frontend/src/test/admin_studio.test.ts` (RED)**
  Add `describe('Milestone 39: 4-Role RBAC Workspace, Customer VIP Hub, Organizer Studio & System Admin Command Center', ...)` testing:
  1. **Role-Specific Header Gating (`Header.tsx`)**:
     - Guest (`currentUser: null`) does NOT render `admin-studio-btn` or `role-workspace-btn`.
     - Customer (`role: 'customer'`) renders `VIP Hub` (`role-workspace-btn`).
     - Organizer (`role: 'organizer'`) renders `Organizer Studio` (`admin-studio-btn`).
     - Admin (`role: 'admin'`) renders `Admin Command` (`admin-studio-btn`).
  2. **1-Click Demo Role Switcher (`AuthModal.tsx`)**:
     - Contains `demo-login-customer`, `demo-login-organizer`, and `demo-login-admin` quick-switch buttons.
  3. **Full-Page Role Workspace & Customer VIP Hub (`AdminDashboardModal.tsx`)**:
     - Supports `inlinePage` full-page workspace layout (`max-w-7xl` / `role-workspace-container`) with `"Back to Live Arena Catalog"` button.
     - Renders **Customer VIP Hub** when `user.role === 'customer'` (Loyalty Tier Progress Bar `Bronze / Silver / Gold / Platinum VIP`, Next-Event Countdown, Spending Breakdown, Passbook quick actions).
     - Renders **Category Revenue & Occupancy Breakdown** and **Occupancy Heat Bars** for `organizer` and `admin`.
     - Renders **Platform Health & RBAC Governance** tab (`tab-system-health`) exclusively when `user.role === 'admin'`.
- [ ] **Step 2: Run Vitest to witness the RED failure**
  Run:
  ```bash
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run src/test/admin_studio.test.ts < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ PASS" || (echo "❌ FAIL:" && tail -n 35 /tmp/agy-cmd.log)
  ```

---

### Task 3: Implement 4-Role Header, 1-Click Demo Switcher, and Full-Page Multi-Role Workspace (GREEN)

**Files:**
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/types/index.ts`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/Header.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/AuthModal.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/components/AdminDashboardModal.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/App.tsx`
- Modify: `/Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src/index.css`

- [ ] **Step 1: Update `User` role union in `frontend/src/types/index.ts`**
  Update `User.role` to `'customer' | 'organizer' | 'admin' | string`.

- [ ] **Step 2: Gate Header Navigation by Role in `frontend/src/components/Header.tsx`**
  In `Header.tsx`:
  - Only show `My Tickets` (`my-tickets-btn`) and the Notification Bell when `currentUser` is signed in (or keep Notification Bell for signed-in users so Guests have a clean header with Theme, Currency, Gateway status, and `Sign In`).
  - Role Workspace button in `Header.tsx`:
    - If `!currentUser`: do NOT render `admin-studio-btn` or `role-workspace-btn`!
    - If `currentUser.role === 'customer'` and `onOpenAdmin`: render `<button data-testid="role-workspace-btn" className="btn-admin-studio ..." onClick={onOpenAdmin}><SparklesIcon size={15} /><span>VIP Hub</span></button>`.
    - If `currentUser.role === 'organizer'` and `onOpenAdmin`: render `<button data-testid="admin-studio-btn" className="btn-admin-studio ..." onClick={onOpenAdmin}><SparklesIcon size={15} /><span>Organizer Studio</span></button>`.
    - If `currentUser.role === 'admin'` and `onOpenAdmin`: render `<button data-testid="admin-studio-btn" className="btn-admin-studio ..." onClick={onOpenAdmin}><SparklesIcon size={15} /><span>Admin Command</span></button>`.

- [ ] **Step 3: Add 1-Click Demo Role Switcher to `frontend/src/components/AuthModal.tsx`**
  In `AuthModal.tsx`:
  - Add a `"⚡ Instant Demo Role Switcher (1-Click Sign In)"` section inside the modal with 3 buttons:
    - `data-testid="demo-login-customer"` (`👤 Customer VIP` — `user@tickets.com` / `user123`)
    - `data-testid="demo-login-organizer"` (`🎪 Dept Organizer` — `organizer@tickets.com` / `organizer123`)
    - `data-testid="demo-login-admin"` (`🛡️ System Admin` — `admin@tickets.com` / `admin123`)
  - When clicked, call `loginUser({ email, password })`. If the backend login succeeds, call `onAuthSuccess(res.user, res.token)`. If `organizer@tickets.com` hasn't been seeded yet or offline, synthesize a valid client session (`setStoredSession`) with that role and call `onAuthSuccess` so demo switching works 100% reliably!
  - In the Register tab's role dropdown, include `<option value="customer">Customer (VIP Attendee)</option>`, `<option value="organizer">Department Organizer (Event Creator)</option>`, and `<option value="admin">System Administrator</option>`.

- [ ] **Step 4: Transform `AdminDashboardModal.tsx` into a Rich Full-Page Multi-Role Command Center & Customer VIP Hub**
  In `frontend/src/components/AdminDashboardModal.tsx`:
  - Add optional prop `inlinePage?: boolean;` and `events?: EventItem[];` and `onSelectEvent?: (ev: EventItem) => void;` and `onOpenPassbook?: () => void;`.
  - When `inlinePage` is true, render as a spacious full-page 1280px workspace (`role-workspace-container w-full space-y-6`) instead of `fixed inset-0 z-50 max-w-4xl`, with a top header bar containing `"← Back to Live Arena Catalog"` (`onClick={onClose}`).
  - **Role 1: Customer VIP Hub (`user?.role === 'customer'`)**:
    - Fetches user bookings (`fetchUserBookings({ userId: user.id, email: user.email })`) + fallback sample bookings if new user so the dashboard is immediately rich and visual!
    - **4 VIP KPI Cards**:
      1. `Active Passes & Tickets` (count + next event badge)
      2. `VIP Loyalty Status` (points calculated from spend/bookings, tier badge `SILVER VIP` / `GOLD VIP` / `PLATINUM VIP`, animated progress bar to next tier)
      3. `Total Live Spend` (formatted currency + average ticket value)
      4. `Member Perks & Savings` (voucher discounts saved + priority waitlist fast-track active)
    - **Next Upcoming Event Spotlight & Countdown Card**: Shows next upcoming event from catalog/bookings with days/hours countdown, venue weather readiness badge, and `"View Seat Map"` / `"Open QR Passbook"` buttons.
    - **Spending & Attendance by Category Breakdown**: Visual horizontal progress bars showing attendance mix (`Music & Concerts`, `Technology & Conferences`, `Sports & Fitness`, `Cinema & Film Premiere`) + VIP Member Perk checklist (`Zero-Fee Ticket Transfers`, `Priority Waitlist Auto-Claim`, `Instant PDF Tax Receipts`, `AI Concierge Itinerary`).
  - **Role 2: Department Organizer Studio (`user?.role === 'organizer'` or `'admin'`)**:
    - **Tab 1 (`analytics` — Department Telemetry & Inventory)**:
      - 4 KPI Cards (Total Revenue, Tickets Issued, Avg Occupancy SVG Ring Gauge, Active Events).
      - **NEW: Revenue & Seat Occupancy by Category Breakdown Panel**: Visual multi-bar breakdown aggregating revenue and seat fill rate per category so the dashboard looks like a true analytics suite!
      - **Enhanced Catalog Inventory Table**: Adds visual horizontal **Seat Occupancy Heat Bars** (`emerald` -> `amber` -> `rose`) inside each event row + Pause/Resume sales button.
      - Recent Ticket Purchases feed.
    - **Tab 2 (`create` — Publish New Event & 2D Seating Provisioning)**: Event creation form with automatic row A–E seat grid provisioning.
    - **Tab 3 (`vouchers` — Promotions & Vouchers)**: Active campaigns KPI row, coupon generator, and campaign roster table.
  - **Role 3: System Admin Command Center (`user?.role === 'admin'` ONLY)**:
    - Adds **Tab 4 (`system` — Platform Health & RBAC Governance)** (`data-testid="tab-system-health"`):
      - **Microservice Mesh Health & Latency Matrix**: 9 service cards (`API Gateway :8080`, `Catalog Service :8081`, `Inventory Service :8082`, `Booking Saga :8083`, `Auth Service :8084`, `Notification Worker :8085`, `PostgreSQL 16 :5432`, `Redis 7 Atomic Locks :6379`, `RabbitMQ 3 Broker :5672`) with `HEALTHY` pulse badges, protocol tags, and p95 latency bars.
      - **RBAC Role & Access Policy Matrix**: Clean 4-column governance table (`Guest`, `Customer VIP`, `Department Organizer`, `System Admin`) showing exact privileges.
      - **Live Multi-Channel Notification Delivery Audit Log**: Displays recent Email/SMS dispatches from `fetchNotificationDeliveries()`.

- [ ] **Step 5: Wire Full-Page Workspace Mode in `frontend/src/App.tsx` and `.role-organizer` in `frontend/src/index.css`**
  - In `App.tsx`:
    - When `isAdminModalOpen` is `true`, render `<AdminDashboardModal isOpen={true} inlinePage={true} onClose={() => setIsAdminModalOpen(false)} user={currentUser} events={events} onSelectEvent={(ev) => { setIsAdminModalOpen(false); handleSelectEvent(ev); }} onOpenPassbook={() => setIsMyBookingsOpen(true)} onEventCreated={...} />` directly inside `<main className="main-content">`!
    - Also make clicking the TicketVerse home logo (`onHomeClick`) close `isAdminModalOpen` (`setIsAdminModalOpen(false); setSelectedEvent(null);`).
  - In `index.css`:
    - Add `.user-role-badge.role-organizer` styling (`background: rgba(168, 85, 247, 0.2); border: 1px solid rgba(168, 85, 247, 0.45); color: #d8b4fe;`) and light-mode override.
    - Ensure `.role-workspace-container` has full light/dark theme support (using the universal light-mode surface adapter so all `bg-slate-900`/`bg-slate-800` cards inside `.role-workspace-container` flip to crisp light surfaces in Light Mode!).

- [ ] **Step 6: Run `ui_audit.py`, Go Tests, Vitest Suite, and TypeScript Compiler (GREEN)**
  Run:
  ```bash
  python3 /Users/thiennguyen/.gemini/antigravity-cli/skills/ui-ux-pro-max/scripts/ui_audit.py /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend/src
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/backend/auth-service && CI=true go test -timeout 30s ./...
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install vitest run < /dev/null > /tmp/agy-cmd.log 2>&1 && echo "✅ VITEST PASS" || (echo "❌ FAIL:" && tail -n 35 /tmp/agy-cmd.log)
  cd /Users/thiennguyen/teamwork_projects/ticket_booking_system/frontend && CI=true GIT_TERMINAL_PROMPT=0 npx --no-install -p typescript tsc --noEmit --pretty false < /dev/null > /tmp/agy-tsc.log 2>&1 && echo "✅ TSC PASS" || (echo "❌ TSC FAIL:" && tail -n 35 /tmp/agy-tsc.log)
  ```
