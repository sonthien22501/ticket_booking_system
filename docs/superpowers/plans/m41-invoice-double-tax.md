# M41 — Invoice Double-Tax & Booking Total Consistency

BASE_SHA: b5b0a99 (run `git rev-parse HEAD` to confirm before starting)
Branch: current. Commit locally, do NOT push. Strict RED → GREEN TDD. Surgical diffs only.

## Convention (Ruling)
`bookings.total_amount` = **grand total actually paid, tax-inclusive, after voucher discount, in USD base**.
`bookings.tax_amount` = tax portion (same unit as CalculateTax returns — verify, see Step 0).
This matches the seed data (`bkg-demo-001`: total 162.75 = 150 + 12.75 tax) and the frontend checkout (`CheckoutForm.tsx` shows `calculateTax(...).finalTotal`, tax-inclusive).
Ruling: tax-inclusive total_amount — reason: matches seed + what customer sees at checkout — cost if wrong: invoices/analytics revenue shift by tax %.

## Observed bugs
1. **Invoice double tax** — `backend/booking-service/invoice.go` `BuildInvoice` (L77-91): `subtotalUSD := booking.TotalAmount (+discount)` then `totalCurr := taxableCurr + taxAmount`. Since total_amount already includes tax, tax is added twice.
   Live evidence: `bkg-demo-001` invoice shows Subtotal $162.75, Tax $12.75, Total $175.50 — should be Subtotal $150.00, Tax $12.75, Total $162.75.
2. **New bookings store pre-tax total** — `backend/booking-service/main.go` ≈L539: `taxRate, taxAmount, _ := CalculateTax(totalAmount, 0.0, currency)` but `totalAmount` is never increased by tax before the INSERT (≈L564-566). A $150 VIP seat booking stored `total_amount=150` while checkout displayed $162.75.

## Step 0 — Verify units (read only)
Read `CalculateTax` (grep `func CalculateTax` in backend/booking-service). Determine whether returned `taxAmount` is USD or converted currency. Read how `handleBookingInvoice` (invoice.go ≈L487-586) loads `taxAmount` / `rate` and calls `BuildInvoice` (L565). Read existing tests `backend/booking-service/currency_invoice_test.go` L150-260 to see what they assert.

## Step 1 — RED tests (`backend/booking-service/invoice_test.go`)
- `TestBuildInvoiceNoDoubleTax_USD`: Booking{TotalAmount:162.75, DiscountAmount:0}, currency USD, rate 1, taxRate 0.085, taxAmount 12.75 → expect Subtotal 150.00, TaxAmount 12.75, TotalAmount 162.75.
- `TestBuildInvoiceNoDoubleTax_WithVoucher`: Booking{TotalAmount:104.16, DiscountAmount:24.00} (seed `bkg-chen-007`, tax 8.16) → expect Subtotal 120.00, Discount 24.00, TaxAmount 8.16, TotalAmount 104.16.
- Invariant in both: `Subtotal - Discount + TaxAmount == TotalAmount` (±0.01).
- Booking creation: if there is an existing handler test harness for POST /bookings (grep `handleCreateBooking`/`httptest` in *_test.go), add a test asserting stored/returned total == pre-tax + tax. If no DB-free harness exists, extract a tiny pure helper (e.g. `applyTax(totalPreTax float64, currency string) (total, taxRate, taxAmount float64)`) in main.go and unit-test it — do not build a new harness.

Run `cd backend/booking-service && go test -timeout 30s ./...` → see the new tests FAIL.

## Step 2 — GREEN
- `BuildInvoice`: derive in USD: `taxUSD` = taxAmount converted to USD if Step 0 shows it is in local currency (else as-is); `preTaxUSD := booking.TotalAmount - taxUSD`; `subtotalUSD := preTaxUSD + booking.DiscountAmount`; keep `taxableCurr = (subtotal - discount) * rate`; `totalCurr` must equal `booking.TotalAmount * rate` (rounded per currency decimals). Line-item fallback unit prices (L112, L130, L145) should be based on the pre-tax subtotal — they already use subtotal vars, so they'll follow.
- If a booking has `taxAmount == 0` (legacy rows), keep old behavior: treat TotalAmount as pre-tax and compute tax from taxRate. Document with a one-line comment.
- Booking creation (main.go ≈L539): after computing taxAmount, set `totalAmount = round2(totalAmount + taxAmountUSD)` before INSERT. Make sure the response/`finalBooking` total reflects it, and that the payment simulation/notification uses the same value.
- Existing tests in `currency_invoice_test.go`: if any assertion encodes the double-tax behavior, update it to the new convention and note each change as `Ruling:` in your summary. Do NOT weaken unrelated assertions.

## Step 3 — Verify
1. `go test -timeout 30s ./...` all green; `gofmt -s -w` modified Go files.
2. `docker compose up -d --build booking-service < /dev/null > /tmp/agy-cmd.log 2>&1`
3. `curl -s -o /tmp/inv.pdf localhost:8080/api/bookings/bkg-demo-001/invoice/pdf && grep -a "Tj" /tmp/inv.pdf | grep '\$'` → expect `$150.00` subtotal, `$12.75` tax, `$162.75` total (and no `$175.50`).
4. Also check JSON: `curl -s localhost:8080/api/bookings/bkg-chen-007/invoice` → subtotal 120, discount 24, tax 8.16, total 104.16.
5. Frontend vitest still green: `cd frontend && CI=true npx --no-install vitest run < /dev/null > /tmp/agy-cmd.log 2>&1 && echo PASS || tail -n 30 /tmp/agy-cmd.log`.

## Finish
`GIT_EDITOR=true git add -A && git commit -m "fix(invoice,booking): tax-inclusive booking totals and remove invoice double tax (M41)"`
Reply <200 words: files changed, tests added, pass counts, curl outputs from Step 3.3/3.4, any Rulings, and any non-USD seed rows (GBP/JPY bookings) whose numbers look inconsistent — report them, don't rewrite seed data. If a test fails twice, STOP and report exact error.
