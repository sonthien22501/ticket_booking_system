# M40 — Seat Price, Currency Display, PDF Invoice & Passbook UI Fixes

BASE_SHA: 527d8afd3d5d5fa63e53b78aede190916125b4c2
Branch: current (`feature/milestone-34-cinema-premiere-animations`). Commit locally at the end, do NOT push.

Follow RED → GREEN TDD. Keep diffs surgical (karpathy-guidelines). Run tests silently:
- Frontend: `cd frontend && CI=true npx --no-install vitest run < /dev/null > /tmp/agy-cmd.log 2>&1 && echo PASS || tail -n 40 /tmp/agy-cmd.log`
- Frontend types: `cd frontend && npx --no-install tsc --noEmit --pretty false`
- Go: `cd backend/booking-service && go test -timeout 30s ./...`

---

## Bug 1 — Seat price is $0 at checkout (ROOT CAUSE CONFIRMED)

`GET /api/inventory/evt-101/seats` returns seats with `tierId` but **no `price` and no `tierName`**:
`{"id":"A1","tierId":"tier-vip-101","row":"A","number":1,"status":"AVAILABLE"}`

`fetchSeats()` in `frontend/src/services/api.ts` (≈L159-192) maps `price: s.price ? Number(s.price) : undefined` → undefined → subtotal 0 in `CheckoutForm.tsx` L113, `VenueSeatMap.tsx`, and the confirm dialog in `App.tsx` ≈L484-501.

**Fix:** in `fetchSeats`, when mapping server seats, enrich from `eventDetails?.tiers` by matching `tier.id === seat.tierId`:
- `price`: `s.price` if present, else matching tier's `price`, else first tier price.
- `tierName`: `s.tier_name || s.tierName` else matching tier's `name`.

**RED test** (`frontend/src/services/__tests__/` or existing api test file): mock `fetch` to return the payload above, call `fetchSeats('evt-101', eventWithTiers)` where tiers include `{id:'tier-vip-101', name:'VIP', price:150}` → expect `price === 150` and `tierName === 'VIP'`.

---

## Bug 2 — Currency selector doesn't change displayed prices

`currentCurrency` (App.tsx state) is only passed to Header and BookingModal. Everywhere else hardcodes `$`:
- `frontend/src/components/EventCard.tsx` (lowest price display, `lowestPrice` L43)
- `frontend/src/components/EventCatalog.tsx` (premiere showcase price pill / any `$` price text, budget labels may stay USD)
- `frontend/src/components/VenueSeatMap.tsx` L410 (`$${sec.basePrice}`), L445 (seat chip), L454 (subtotal), plus any tier legend prices
- `frontend/src/components/SeatGrid.tsx` / seat tooltips if they show `$`
- `frontend/src/App.tsx` L490 and L497 (seat confirm dialog chips + subtotal)

**Fix:** add optional `currentCurrency?: string` prop (default `'USD'`) to EventCatalog → EventCard, and VenueSeatMap; pass `currentCurrency` from App.tsx. Replace hardcoded `$x.toFixed(n)` with `formatCurrency(x, currentCurrency)` from `../services/currency`. Prices in data remain USD base; `formatCurrency` converts.

Use `grep -n '\$\${\|\$[0-9{]' ` on these files to find all occurrences. Do not touch budget filter logic values.

**RED test:** render `EventCard` with `currentCurrency="EUR"` and a tier price 100 → expect text `€92.00`. Render VenueSeatMap selection footer with a selected seat price 100 and `currentCurrency="GBP"` → expect `£79.00`.

---

## Bug 3 — PDF invoice shows no prices / seat / qty (ROOT CAUSE CONFIRMED)

`backend/booking-service/invoice.go` `GenerateInvoicePDF`: inside a single `BT…ET` block it emits multiple `x y Td` operators with **absolute** coordinates, but PDF `Td` is **relative** to the previous line start. So 2nd+ text runs land off-page. Affected:
- Line items loop (≈L350-363): seat/qty/unit price/total use `268 %d Td`, `340 %d Td`, `400 %d Td`, `470 %d Td`.
- Subtotal (≈L375-382), Voucher Discount (≈L387-394), Tax (≈L400-407), Total (≈L415-422): value uses `470 %d Td` / `460 %d Td`.
- Footer (≈L425-434): `45 78 Td`, `45 66 Td`.

**Fix:** replace each absolute `x y Td` that is NOT the first in its BT block with absolute positioning via text matrix: `fmt.Sprintf("1 0 0 1 %d %d Tm\n", x, y)` (Tm sets absolute position). Simplest uniform approach: use `Tm` for every positioned text run in those blocks. Keep layout coordinates identical.

**RED test** (`backend/booking-service/invoice_test.go`, add if missing): build an `Invoice` with one item (SeatID "A6", qty 1, unit 150, total 150, subtotal 150, tax 12.75, total 162.75, symbol "$"), call `GenerateInvoicePDF`, assert the content stream contains no Td sequence where a non-first run in a BT block uses absolute x ≥ 200 — practical assertion: `bytes.Contains(pdf, []byte("1 0 0 1 470"))` and `"1 0 0 1 268"` are present AND the string `"268 589 Td"`-style absolute Td for those columns is absent (regex `\n(268|340|400|470|460) -?\d+ Td`). Also assert `($150.00) Tj` and `($162.75) Tj` exist.
After GREEN, rebuild: `docker compose up -d --build booking-service`, then `curl -s -o /tmp/inv.pdf localhost:8080/api/bookings/bkg-demo-001/invoice/pdf && head -c 8 /tmp/inv.pdf` (expect `%PDF-1.4`).

---

## Bug 4 — Passbook / Ticket Pass UI broken in light theme + cramped layout

Screenshots: light theme on. Global overrides in `frontend/src/index.css` L491-504 turn `text-slate-100/200/300` dark, but modal surfaces use arbitrary hex backgrounds not covered by L478-489 (`bg-[#16162A]`, `bg-[#1B1B30]`, `bg-[#161628]`, `bg-[#141424]`) and headers use `bg-gradient-to-r from-indigo-950 via-slate-900 to-indigo-900`. Result: event titles ("Neon Symphony 2026", "My Bookings & Passbook") are dark-on-dark invisible; "Simulate Gate Check-In" button looks washed out.

**Fix A (CSS, index.css, next to L478 block):** in light theme:
- `[class*='bg-[#16162A]'], [class*='bg-[#1B1B30]'], [class*='bg-[#161628]'], [class*='bg-[#141424]']` → `background-color: var(--bg-surface) !important; border-color: var(--border-color) !important;`
- `[class*='from-indigo-950']` (gradient headers) → `background-image: linear-gradient(90deg, #eef2ff, #f8fafc, #ede9fe) !important;`
- Keep amber primary buttons readable: `.bg-amber-600` keep `color:#0f172a` (do not let overrides fade it). Check `opacity`/`disabled` isn't applied.
Use the same 3-selector pattern (`html[data-theme='light']`, `body.light`, `.light`) as the file.

**Fix B (layout):**
- `MyBookingsModal.tsx` ticket chips (≈L572-613): grid is `sm:grid-cols-2` inside narrow card causing "Transfer"/"View Pass" to wrap per word. Change chip to `flex-col sm:flex-row gap-2`, add `min-w-0` + `truncate` on seat label/code, `whitespace-nowrap shrink-0` on buttons. Use grid `grid-cols-1` (single column) to give chips full width.
- `TicketPassModal.tsx` action row (≈L233): add `flex-wrap`; buttons `whitespace-nowrap`; "Print Pass" currently cut off at the right edge.
- Reference cell in MyBookingsModal (≈L502-514): long refs overflow — add `break-all text-left`.

**Verification (Law 5):** add a vitest test that renders `TicketPassModal` and asserts the action container has class `flex-wrap`; and that MyBookingsModal ticket buttons have `whitespace-nowrap`. Then manually confirm via `npx --no-install tsc --noEmit`.

---

## Finish
1. All vitest + go tests GREEN, tsc clean.
2. Format modified files only: `gofmt -s -w backend/booking-service/invoice.go backend/booking-service/invoice_test.go`; `cd frontend && npx --no-install prettier --write <modified files>` (skip if prettier not installed).
3. Rebuild containers: `docker compose up -d --build booking-service frontend < /dev/null > /tmp/agy-cmd.log 2>&1`.
4. `GIT_EDITOR=true git add -A && git commit -m "fix(checkout,invoice,ui): seat tier pricing, currency-aware prices, PDF Tm positioning, light-theme passbook (M40)"`.
5. Return a <200 word summary: files changed, tests added, test command output (pass counts), and anything you could not fix. If any test fails twice, STOP and report exact error output.
