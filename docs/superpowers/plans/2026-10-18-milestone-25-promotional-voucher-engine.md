# Milestone 25: Promotional Voucher & Coupon Engine Implementation Plan

## 1. Overview & Objectives
Milestone 25 introduces a Promotional Voucher & Coupon Engine for the Distributed Event Ticket Booking System. While users can currently browse events, reserve seats, and make payments, organizers previously lacked the ability to run targeted discount campaigns (percentage-off or fixed deductions), set minimum spend thresholds, cap maximum redemptions, or restrict promotional validity windows. Customers also had no mechanism during checkout to enter coupon codes and view real-time discounted subtotal breakdowns.

Milestone 25 delivers:
1. **Promotional Voucher Core (`backend/booking-service`)**:
   - `Voucher` database schema with support for `PERCENTAGE` and `FIXED` discount types, minimum spend thresholds, maximum usage caps, and expiration timestamps.
   - Core validation engine `ValidateVoucher` enforcing active status, expiration limits, usage caps, minimum spend rules, and anti-stacking guarantees.
   - Integration into the checkout and Saga booking lifecycle (`createBooking`), saving `voucher_code` and `discount_amount` on `bookings` and atomically incrementing usage counts upon confirmation.
2. **REST Endpoints & Gateway Routing (`backend/booking-service` & `backend/api-gateway`)**:
   - `POST /api/vouchers/validate`: Validates voucher code against order subtotal and returns discounted totals.
   - `GET /api/admin/vouchers` & `POST /api/admin/vouchers`: Allows organizers to inspect redemption velocity and launch new promo codes.
   - Gateway routing in `nginx.conf` forwarding `/api/vouchers` to `booking_service`.
3. **Frontend Real-Time Checkout Coupon Experience (`frontend/src`)**:
   - `CheckoutForm.tsx`: Collapsible/interactive voucher input section with tag icon, live "Apply" validation, clear error feedback (e.g., minimum spend not met, expired code), discount summary breakdown, and remove option.
   - Anti-stacking protection (strictly one promo code per reservation).
   - `BookingModal.tsx` & `App.tsx`: Passes applied voucher code to `createBooking()`.
4. **Organizer Studio Promotions Tab (`frontend/src/components/AdminDashboardModal.tsx`)**:
   - New "Promotions & Vouchers" management tab in the Admin/Organizer Studio.
   - KPI metrics on total vouchers, total redemptions, and active campaigns.
   - Voucher creation wizard (code, type, value, minimum spend, max uses).
5. **Automated Verification**:
   - Go unit test suites in `backend/booking-service/voucher_test.go` covering discount math, minimum spend constraints, usage limits, expiration dates, and anti-stacking logic.
   - Vitest frontend test suite in `frontend/src/test/voucher_checkout.test.ts` verifying coupon application, discount calculations, error states, and submission payloads.

---

## 2. Review Focus (Top Edge Cases & Failure Modes)
1. **Minimum Spend Boundary**: If an order subtotal is $45 and a voucher requires minimum spend $50, the engine must reject with HTTP 400 and clear message `"Minimum spend of $50.00 required for this voucher"`.
2. **Discount Clamping & Non-Negative Totals**: A fixed discount of $100 applied to an $80 order must clamp the discount to $80 and never produce a negative total amount.
3. **Usage Limits & Expiry Enforcement**: Expired vouchers or vouchers where `current_uses >= max_uses` must be rejected immediately.
4. **Case-Insensitive & Whitespace Trimming**: Promo codes must normalize automatically (e.g., ` earlybird20 ` -> `EARLYBIRD20`).
5. **Anti-Stacking Guarantee**: Exactly one voucher can be applied per checkout transaction.

---

## 3. Implementation Steps (RED-GREEN TDD)

### Step 1 (RED): Backend Tests for Voucher Engine
- Create `backend/booking-service/voucher_test.go` with unit tests for:
  - Percentage discount calculation (e.g., 20% off $150 = $30 discount, final $120).
  - Fixed discount calculation (e.g., $15 off $60 = $15 discount, final $45).
  - Fixed discount clamped to subtotal (e.g., $100 off $40 = $40 discount, final $0).
  - Minimum spend rejection.
  - Expiration check.
  - Max usage cap check.
  - HTTP handlers (`/api/vouchers/validate`, `/api/admin/vouchers`).
- Verify tests fail (RED) or compile failure.

### Step 2 (GREEN): Backend Implementation
- Implement `backend/booking-service/voucher.go`:
  - Types: `Voucher`, `ValidateVoucherRequest`, `ValidateVoucherResponse`, `CreateVoucherRequest`.
  - Schema init: `initVoucherSchema` creating `vouchers` table and altering `bookings` table.
  - Methods: `ValidateVoucher`, `ApplyVoucherToBooking`, `ListVouchers`, `CreateVoucher`.
  - HTTP handlers: `handleValidateVoucher`, `handleAdminVouchers`.
- Wire routes and schema initialization in `backend/booking-service/main.go`.
- Update `CreateBookingRequest` and `createBooking` in `backend/booking-service/main.go` to support `voucherCode` / `voucher_code`, compute discounted `total_amount`, and persist `voucher_code` & `discount_amount`.
- Update `backend/api-gateway/nginx.conf` to route `/api/vouchers` and `/vouchers` to `booking_service`.
- Run Go test suite: `go test -timeout 30s ./...` in `backend/booking-service`.

### Step 3 (RED): Frontend Unit Tests
- Create `frontend/src/test/voucher_checkout.test.ts` testing:
  - `validateVoucher()` API service helper.
  - Voucher validation calculation helpers.
  - Coupon input and discount display in checkout.
- Verify tests fail (RED).

### Step 4 (GREEN): Frontend Implementation
- Update `frontend/src/types/index.ts` with voucher types and updated `BookingRequest` / `BookingResponse`.
- Update `frontend/src/services/api.ts` with `validateVoucher`, `fetchAdminVouchers`, `createAdminVoucher`, and forward `voucherCode` in `createBooking`.
- Update `frontend/src/components/CheckoutForm.tsx` to include promo code input, validation feedback, and discount line item in summary.
- Update `frontend/src/components/BookingModal.tsx` and `frontend/src/App.tsx` to handle voucher application.
- Update `frontend/src/components/AdminDashboardModal.tsx` to include the "Promotions & Vouchers" studio tab.
- Run frontend tests: `npx --no-install vitest run`.
- Verify TypeScript types: `npx --no-install -p typescript tsc --noEmit`.

### Step 5: Verification & DoD Audit
- Run all backend test suites (`booking-service`, `catalog-service`, `inventory-service`).
- Run Vitest test suite and production build (`npm run build`).
- Update `docs/ARCHITECTURE.md` with M25 feature documentation and interface contracts.
- Perform clean commit, git push, and generate GitHub Pull Request link.
