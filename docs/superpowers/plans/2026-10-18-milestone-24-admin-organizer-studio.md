# Milestone 24: Admin & Organizer Studio, Live Sales Telemetry & Event Management Implementation Plan

## 1. Overview & Objectives
Milestone 24 provides event organizers and venue operators with an administrative command center and live telemetry dashboard. While customers can browse, reserve, and view passes, organizers previously had no unified portal to monitor ticket sales velocity, track gross revenue, analyze seat occupancy, or publish new events into the catalog with automated seat provisioning.

Milestone 24 delivers:
1. **Admin Analytics API (`backend/booking-service`)**:
   - `GET /api/admin/analytics`: Aggregates gross revenue, confirmed vs cancelled orders, ticket scan status, active event counts, seat occupancy percentages, and recent booking transaction feeds.
   - Access control via JWT verification or administrative header fallback.
2. **Catalog Admin Enhancements (`backend/catalog-service`)**:
   - Enhanced `POST /api/events` event creation that automatically provisions `ticket_tiers`, `event_inventory`, and default 2D `seat_items` grid for immediate booking.
   - `PUT /api/events/{id}/status` or status update endpoint to pause, publish, or archive events.
3. **API Gateway Routing (`backend/api-gateway`)**:
   - Routes `/api/admin/*` and `/admin/*` to `booking_service`.
4. **Frontend Admin & Organizer Studio (`frontend/src`)**:
   - `AdminDashboardModal`: Glassmorphic modal with two tabs:
     - **Telemetry & Analytics**: Glowing KPI cards (Gross Revenue, Tickets Sold, Average Occupancy Rate, Active Events), real-time booking stream, and per-event occupancy bars.
     - **Event Studio**: Interactive event creation wizard with category presets, venue configurations, ticket tiers, base pricing, and automated seat row generation.
   - `Header`: "Organizer Studio" quick-access launcher with admin badge.
   - `api.ts`: Helper methods `fetchAdminAnalytics()`, `createAdminEvent()`, `updateEventStatus()`.
5. **Automated Verification**:
   - Go backend unit tests for admin analytics aggregation and event provisioning.
   - Vitest unit tests for Admin Studio rendering, KPI calculations, and event creation workflow.

---

## 2. Review Focus (Top Edge Cases & Failure Modes)
1. **Empty / Zero Data Resilience**: Analytics endpoint must gracefully handle empty database states (0 events, 0 bookings) without division-by-zero errors in average occupancy calculations.
2. **Seat Item Provisioning**: When creating a new event in the catalog, default seat rows (e.g. Rows A..N with 10 seats each) must be generated in `seat_items` so customers can immediately view and reserve seats on the 2D seat map.
3. **Consistent Revenue Aggregation**: Gross revenue must accurately sum only `CONFIRMED` bookings, excluding `CANCELLED` and `EXPIRED` orders.
4. **Instant Catalog Synchronization**: Creating a new event through the Admin Studio must automatically invalidate the catalog cache and trigger an event refresh on the main browsing screen.
5. **Role-Aware Navigation & Demo Accessibility**: The Admin Studio is accessible to logged-in users with `role: "admin"` or via an explicit Organizer mode toggle so reviewers can evaluate the feature without backend credential friction.

---

## 3. Implementation Steps (RED-GREEN TDD)

### Step 1 (RED): Backend Tests for Admin Analytics & Event Provisioning
- Create `backend/booking-service/admin_analytics_test.go` verifying analytics calculation.
- Create `backend/catalog-service/event_admin_test.go` verifying event creation with seat generation.

### Step 2 (GREEN): Backend Implementation
- Implement `handleAdminAnalytics` in `backend/booking-service/main.go`.
- Enhance `createEvent` in `backend/catalog-service/main.go` to generate `seat_items` and support status updates.
- Update `backend/api-gateway/nginx.conf` to route `/api/admin`.

### Step 3 (RED): Frontend Unit Tests
- Create `frontend/src/test/admin_studio.test.tsx` verifying KPI cards, booking stream, and event creation form.

### Step 4 (GREEN): Frontend Implementation
- Update `frontend/src/types/index.ts` with admin analytics types.
- Update `frontend/src/services/api.ts` with admin API methods.
- Create `frontend/src/components/AdminDashboardModal.tsx`.
- Update `frontend/src/components/Header.tsx` to add "Organizer Studio" launcher.
- Integrate into `frontend/src/App.tsx`.

### Step 5: Verification & DoD Audit
- Run all Go test suites (`catalog-service`, `booking-service`, `inventory-service`, `auth-service`).
- Run Vitest frontend test suite.
- Update `docs/ARCHITECTURE.md`.
- Clean commit and git push.
