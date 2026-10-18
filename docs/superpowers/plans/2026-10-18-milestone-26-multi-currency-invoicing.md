# Milestone 26: Multi-Currency Checkout & PDF Invoicing Implementation Plan

## 1. Overview & Objectives
Milestone 26 delivers **Multi-Currency Checkout & PDF Invoicing** for the Distributed Event Ticket Booking System. As our ticketing platform expands to international attendees and corporate buyers, users need the ability to:
1. View event prices, ticket tiers, and seat costs in their preferred currency (USD, EUR, GBP, JPY, CAD, AUD) with real-time exchange rates.
2. Calculate localized sales tax / VAT automatically during checkout with transparent breakdowns.
3. Provide corporate billing details (Company Name, Tax/VAT Registration Number, Billing Address) during reservation.
4. Download standardized, audit-compliant PDF ticket receipts and invoices for corporate expense reports and tax deductions.

### Delivered Capabilities:
1. **Multi-Currency & Conversion Engine (`backend/booking-service`)**:
   - Supported currencies: `USD` (Base $), `EUR` (€), `GBP` (£), `JPY` (¥), `CAD` (C$), `AUD` (A$).
   - Standard exchange rates relative to USD with precision formatting (0 decimals for JPY, 2 decimals for fiat currencies).
   - `GET /api/currencies/rates`: Returns real-time exchange rates and localized currency symbols.
   - `POST /api/currencies/convert`: Converts arbitrary amounts between currencies with precise rounding.
2. **Automated Tax & VAT Engine (`backend/booking-service`)**:
   - Regional tax/VAT calculation based on chosen currency (e.g., USD: 8.5%, EUR: 20%, GBP: 20%, JPY: 10%, CAD: 13%, AUD: 10%).
   - Calculates taxable subtotal (strictly after applying promotional voucher discounts, never before).
   - Extends database `bookings` schema with `currency`, `exchange_rate`, `tax_rate`, `tax_amount`, `company_name`, `tax_id`, and `billing_address`.
3. **Audit-Ready PDF Ticket Invoice Generator (`backend/booking-service`)**:
   - `GET /api/bookings/:id/invoice`: Structured invoice metadata with itemized breakdown, tax details, and issuer registration.
   - `GET /api/bookings/:id/invoice/pdf`: Real, spec-compliant binary PDF generation (`%PDF-1.4`) with corporate header, line items, VAT registration, payment confirmation stamp, and clean byte streams downloadable directly by attendees and accounting teams.
4. **API Gateway Routing (`backend/api-gateway/nginx.conf`)**:
   - Routing for `/api/currencies` and `/currencies` with rate limiting and distributed tracing headers.
5. **Frontend Multi-Currency & Invoicing Experience (`frontend/src`)**:
   - Global currency selector in Header allowing instant toggling between USD, EUR, GBP, JPY, CAD, AUD.
   - Dynamic price conversion across event catalog cards, ticket tiers, and interactive seat map.
   - Checkout corporate invoice accordion (Company Name, Tax/VAT ID, Billing Address) and order summary with itemized Tax/VAT line.
   - Download PDF Invoice button in `MyBookingsModal` and booking confirmation dialogs.
6. **Automated Verification**:
   - Go unit and HTTP tests in `backend/booking-service/currency_invoice_test.go`.
   - Vitest unit tests in `frontend/src/test/currency_invoicing.test.ts`.

---

## 2. Review Focus (Top 5 Edge Cases & Failure Modes)
1. **Zero-Decimal Currency Precision (JPY)**: JPY amounts must always round to integers (e.g. `155.0` JPY, not `155.43` JPY), preventing fractional cents in Japanese Yen transactions.
2. **Tax on Discounted Subtotal**: When an attendee applies a voucher (e.g., $20 off $100), tax must calculate on the net taxable amount ($80), never the pre-discount amount ($100).
3. **Zero-Dependency Spec-Compliant PDF Generation**: The PDF generator must produce valid `%PDF-1.4` binary streams with correct cross-reference tables (`xref`) and trailer dictionaries so that PDF viewers (Chrome, Adobe Acrobat, macOS Preview) render and parse without corruption.
4. **Default Currency Resilience**: Requests without explicit currency or exchange parameters must seamlessly default to `USD` (rate `1.0`) with zero runtime errors or schema validation crashes.
5. **Corporate Billing Non-Destructive Storage**: If a user does not provide corporate billing details, the booking record must remain fully valid with NULL/empty corporate fields.

---

## 3. Implementation Steps (Strict RED-GREEN TDD)

### Step 1 (RED): Backend Tests for Multi-Currency & Invoice Generator
- Create `backend/booking-service/currency_invoice_test.go`:
  - Test exchange rates retrieval and currency conversion math (including JPY zero-decimal rounding).
  - Test tax calculation logic on both standard and voucher-discounted subtotals.
  - Test `GET /api/currencies/rates` and `POST /api/currencies/convert`.
  - Test `GET /api/bookings/:id/invoice` structured metadata.
  - Test `GET /api/bookings/:id/invoice/pdf` binary generation, verifying `%PDF-1.4` magic bytes, `Content-Type: application/pdf`, and `Content-Disposition`.
- Run tests and witness failure (RED).

### Step 2 (GREEN): Backend Implementation
- Create `backend/booking-service/currency.go`:
  - Define currencies, rates, symbols, and tax rate presets.
  - Implement `ConvertCurrency(amount float64, from, to string) (float64, float64, error)`.
  - Implement `CalculateTax(subtotal, discount float64, currency string) (taxRate float64, taxAmount float64, taxableAmount float64)`.
  - Implement HTTP handlers: `handleCurrencyRates`, `handleCurrencyConvert`.
- Create `backend/booking-service/invoice.go`:
  - Define `Invoice` struct, `InvoiceItem`, `TaxBreakdown`.
  - Implement `BuildInvoice(booking *Booking, tickets []Ticket) *Invoice`.
  - Implement `GenerateInvoicePDF(inv *Invoice) ([]byte, error)` using pure, self-contained standard PDF 1.4 vector generation.
  - Implement HTTP handlers: `handleBookingInvoice(w, r, bookingID)`.
- Update `backend/booking-service/main.go`:
  - Add schema migration in `initDB()` or `initInvoiceSchema()` to ensure `bookings` table has `currency`, `exchange_rate`, `tax_rate`, `tax_amount`, `company_name`, `tax_id`, `billing_address`.
  - Update `CreateBookingRequest` and `createBooking()` to process currency, tax calculation, and corporate billing info.
  - Wire `/currencies/rates`, `/currencies/convert`, and handle `/bookings/:id/invoice` & `/bookings/:id/invoice/pdf`.
- Update `backend/api-gateway/nginx.conf`:
  - Route `/api/currencies` and `/currencies` to `booking_service`.
- Run `go test -timeout 30s ./...` in `backend/booking-service` and verify GREEN.

### Step 3 (RED-GREEN): Frontend Multi-Currency & Invoicing
- Create `frontend/src/services/currency.ts`:
  - Define supported currencies, rates, symbols, formatting utilities (`formatCurrency(amount, currency)`).
  - Export functions to fetch live rates or use cached presets.
- Update `frontend/src/types/index.ts`:
  - Add `Currency`, `CorporateBillingDetails`, `InvoiceData` interfaces.
  - Extend `CreateBookingPayload` and `Booking` interfaces with currency, tax, and billing details.
- Update `frontend/src/services/api.ts`:
  - Add `fetchCurrencyRates()`, `getInvoice(bookingId)`, `downloadInvoicePDF(bookingId)`.
- Update `frontend/src/components/Header.tsx`:
  - Add currency dropdown selector with sleek badge styling and flag/symbol indicators.
- Update `frontend/src/components/CheckoutForm.tsx`:
  - Add corporate billing disclosure toggle ("Add Business / Expense Details").
  - Render itemized tax/VAT line in Order Summary.
  - Pass corporate billing fields and active currency in checkout submission.
- Update `frontend/src/components/MyBookingsModal.tsx` & passbook views:
  - Add "Download PDF Invoice" button for confirmed tickets, triggering direct PDF download.
- Create Vitest suite `frontend/src/test/currency_invoicing.test.ts`:
  - Test currency conversion formatting (USD, EUR, GBP, JPY).
  - Test tax calculation logic on discounted and regular subtotals.
  - Test corporate billing payload propagation.
- Run `npm test -- --run` and `tsc --noEmit` and verify GREEN.

### Step 4: Verification & Anti-Cheat QA
- Verify all 5 Review Focus edge cases.
- Perform mutation testing on tax calculation and JPY rounding, ensuring test suite fails when logic is modified, then revert immediately.
- Run all backend service tests (`booking-service`, `catalog-service`, `inventory-service`, `auth-service`).
- Verify production build `npm run build`.

### Step 5: DoD Sign-off & Clean Local Commit
- Audit diff with `git diff <BASE_SHA>..HEAD --stat`.
- Update `docs/ARCHITECTURE.md` milestone progress checklist.
- Commit cleanly: `git commit -m "feat(currency): implement multi-currency checkout, automated tax & PDF invoicing (M26)"`.
- Output handoff message for `push-pr`.
