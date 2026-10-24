# Milestone 29: VIP Add-ons, Event Merchandise & Concessions Upsell Engine Implementation Plan

## 1. Overview & Objectives
Milestone 29 delivers the **VIP Add-ons, Event Merchandise & Concessions Upsell Engine** for the Distributed Event Ticket Booking System. As attendees complete seat selection, organizers need an integrated e-commerce mechanism to offer high-margin auxiliary items:
1. **VIP Priority Parking Passes**
2. **Commemorative Tour & Festival Merchandise (T-Shirts, Posters)**
3. **VIP Lounge & Hospitality Access**
4. **Pre-paid Concessions & Beverage Vouchers**

Purchased add-ons must be atomically checked against available stock, itemized during checkout, incorporated into regional tax calculations and PDF invoices, and visible in attendee passbooks.

### Delivered Capabilities:
1. **Add-on Catalog & Inventory Management (`backend/booking-service`)**:
   - `event_addons` table: tracks per-event add-on catalog, pricing, category, total inventory, and available inventory.
   - `booking_addons` table: links purchased add-ons to confirmed bookings with unit prices and quantities.
   - `GET /api/events/{id}/addons`: queries active add-ons for an event.
   - `POST /api/addons`: provisions or updates add-on items.
2. **Checkout Cart Integration & Atomic Deduction (`backend/booking-service`)**:
   - Updates `CreateBookingRequest` with `addons: [{ addonId: string, quantity: number }]`.
   - Validates add-on inventory and atomically decrements `available_inventory`.
   - Adds add-on subtotals into `total_amount`, currency conversions, and voucher discounts.
3. **Frontend Upsell UI (`frontend/src`)**:
   - Add-on selector in `BookingModal.tsx` allowing 1-click addition and quantity toggling (`+` / `-`).
   - Dynamic real-time cart subtotal calculation including add-ons.
   - Itemized add-on receipt displayed in `BookingConfirmation.tsx` and `MyBookingsModal.tsx`.
4. **Automated Verification**:
   - Go unit tests in `backend/booking-service/addon_test.go`.
   - Vitest unit tests in `frontend/src/test/addon_upsell.test.ts`.

---

## 2. Review Focus (Top 5 Edge Cases)
1. **Add-on Stock Exhaustion**: Ordering more add-ons than `available_inventory` must reject with HTTP 409 Conflict.
2. **Add-on Event Mismatch**: Adding an add-on that belongs to a different `eventId` must be rejected with HTTP 400 Bad Request.
3. **Add-on Tax & Invoice Line Items**: Add-on costs must be correctly aggregated into subtotal, regional tax calculation, and displayed in PDF invoices.
4. **Voucher Discount Application**: Subtotal including add-ons must correctly satisfy voucher `min_spend` and receive percentage or fixed deductions.
5. **Zero Add-on Backwards Compatibility**: Bookings created without add-ons (standard tickets only) must continue to function with zero regressions.

---

## 3. Implementation Steps (Strict RED-GREEN TDD)

### Step 1 (RED): Backend Tests
- Create `backend/booking-service/addon_test.go`:
  - Test `GET /api/events/{id}/addons`.
  - Test add-on inventory deduction during booking creation.
  - Test conflict when requested quantity exceeds stock.
- Run tests and witness failure (RED).

### Step 2 (GREEN): Backend Implementation
- Create `backend/booking-service/addon.go`:
  - `EventAddon`, `BookingAddon`, `AddonItemRequest`.
  - Addon endpoints and DB queries.
  - Hook into `createBooking` to deduct inventory and insert `booking_addons`.
- Update `infra/postgres/init.sql` and `scripts/seed_test_dataset.sql`.
- Update `backend/api-gateway/nginx.conf`.
- Run Go tests and verify passing (GREEN).

### Step 3 (RED & GREEN): Frontend Implementation
- Add add-on types to `frontend/src/types/index.ts`.
- Add `fetchEventAddons` in `frontend/src/services/api.ts`.
- Integrate add-on selection into `BookingModal.tsx`, `BookingConfirmation.tsx`, and `MyBookingsModal.tsx`.
- Write `frontend/src/test/addon_upsell.test.ts`.
- Run `vitest run` and verify GREEN.

### Step 4: Verification, Local Commit & Push PR
- Run full regression suite (`verify_e2e.py`).
- Update `docs/ARCHITECTURE.md`.
- Commit and push to GitHub, open PR with `push-pr`.
