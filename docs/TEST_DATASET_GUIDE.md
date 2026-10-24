# Ticket Booking System — Master Test Dataset & Verification Runbook

This comprehensive test dataset is designed to exercise, verify, and demonstrate **100% of the features** across all microservices and frontend components of the Event Ticket Booking System.

---

## ⚡ Quick Start: 1-Click Seeding

To apply or reset this master test dataset at any time, execute the automated runner script:

```bash
./scripts/seed_dataset.sh
```

**What this does automatically:**
1. Resets and re-seeds all PostgreSQL tables (`users`, `events`, `ticket_tiers`, `event_inventory`, `seat_items`, `reservations`, `bookings`, `tickets`, `waitlist`, `vouchers`).
2. Flushes Redis caches (`FLUSHALL`) so catalog listings, seat allocations, and rate limiters immediately reflect the clean state.
3. Performs automated health checks against the API Gateway (`:8080`) and prints record counts.

---

## 👥 1. User Personas & Credentials

| Persona | Email | Password | Role | Features Tested |
| :--- | :--- | :--- | :--- | :--- |
| **System Administrator** | `admin@tickets.com` | `admin123` | `admin` | **Admin Dashboard & Organizer Studio**, Live Telemetry, Voucher Management, Event Lifecycle Toggles (`PUT /api/events/:id/status`). |
| **Event Producer** | `organizer@tickets.com` | `organizer123` | `admin` | Event provisioning, seating grid management, attendance analytics. |
| **Standard Demo Customer** | `user@tickets.com` | `user123` | `customer` | **Digital Passbook ("My Bookings")**, QR Ticket display, Multi-Currency booking history (USD & GBP), PDF invoice downloads, booking cancellation. |
| **VIP Tech Executive** | `sarah.connor@cyberdyne.io` | `password123` | `customer` | **Corporate B2B Invoicing** (Company Name, Tax ID, Corporate Address), Waitlist Queue (Position 1). |
| **Software Engineer** | `alex.chen@technova.com` | `password123` | `customer` | **Promotional Voucher Checkout** (`EARLYBIRD20` applied), Waitlist Queue (Position 2). |
| **European Sports Fan** | `maria.garcia@madrid.es` | `password123` | `customer` | **Euro (€) Currency Checkout** (20% EU VAT calculation, corporate sports billing), **Promoted Waitlist Hold**. |
| **Synthwave Concertgoer** | `kenji.sato@tokyo-synth.jp` | `password123` | `customer` | **Japanese Yen (¥) Checkout** (Zero decimal precision, 10% JP consumption tax). |
| **Corporate Procurement** | `procurement@acme-corp.com` | `password123` | `customer` | High-value corporate bulk orders, Fixed voucher discount (`CORP100`), international B2B billing. |

---

## 🎭 2. Event Catalog Matrix

The dataset provides **9 diverse events** spanning all **6 categories** and all **4 lifecycle statuses**:

| ID | Title | Category | Venue & City | Status | Tiers & Prices | Available / Total | Feature Showcase |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `evt-101` | **Neon Symphony 2026** | Music & Concerts | Grand Arena, Berlin | `ACTIVE` | VIP ($150), General ($60), Balcony ($45) | 61 / 70 | Interactive 2D Seat Map, Hold countdowns, E2E test harness target. |
| `evt-102` | **Global Tech Summit 2026** | Technology & Conferences | Convention Center, San Francisco | `ACTIVE` | Keynote ($299), Standard ($99) | 42 / 50 | Corporate B2B Checkout, Tax ID invoicing. |
| `evt-103` | **UEFA Champions Gala: Berlin vs Madrid** | Sports & Fitness | Olympiastadion, Berlin | `ACTIVE` | Skybox ($350), Grandstand ($95) | 34 / 40 | EUR (€) Currency conversions & 20% European VAT. |
| `evt-104` | **Hamilton: The Musical** | Theater & Arts | Victoria Palace, London | `ACTIVE` | Orchestra ($180), Dress Circle ($85) | 44 / 50 | GBP (£) Currency checkout & Royal theater seating. |
| `evt-105` | **Tokyo Cyberpunk Synth & Anime** | Music & Concerts | Budokan, Tokyo | `ACTIVE` | Cyber VIP ($120), Mezzanine ($50) | 47 / 50 | JPY (¥) Currency checkout (0 decimal rounding). |
| `evt-106` | **Stand-Up Comedy All-Stars** | Comedy & Entertainment | Comedy Cellar, NYC | `ACTIVE` | Front Row ($75) | **0 / 12 (SOLD OUT)** | **Waitlist Queue System**, automatic sold-out detection. |
| `evt-107` | **Distributed Systems Masterclass** | Workshops & Education | Tech Hub, Toronto | `PAUSED` | Lab Pass ($240) | 20 / 20 | **Event Status PAUSED**: Organizer staging toggle. |
| `evt-108` | **Rock Fest Summer Retrospective** | Music & Concerts | Parc del Fòrum, Barcelona | `COMPLETED` | 3-Day Pass ($195) | 0 / 30 | **Event Status COMPLETED**: Historical telemetry & past order audit. |
| `evt-109` | **Midnight Carnival (Rainout)** | Music & Concerts | Amphitheater, Sydney | `CANCELLED` | General ($80) | 20 / 20 | **Event Status CANCELLED**: Full refund saga & ticket invalidation. |

---

## 🪑 3. Interactive Seating Layout & Seat Status Matrix

For the flagship event `evt-101` (Neon Symphony), the venue seating grid illustrates every possible tactile visual state in the UI:

| Seat ID | Row & Number | Tier | Status | Visual Appearance | How to Test |
| :--- | :--- | :--- | :--- | :--- | :--- |
| `A1`, `A2` | Row A, #1, #2 | VIP Front Row | `AVAILABLE` | Luminous Gold | Click to book; used by automated test suite. |
| `A3`, `A4` | Row A, #3, #4 | VIP Front Row | `AVAILABLE` | Luminous Gold | Click to select in UI checkout. |
| `A5` | Row A, #5 | VIP Front Row | `HOLD` | Amber Pulsing (`res-active-hold-01`) | Tests active reservation lock countdown; clicking returns conflict. |
| `A10` | Row A, #10 | VIP Front Row | `BLOCKED` | Dark / Disabled | Maintenance / Sound console seat (disabled). |
| `B1` | Row B, #1 | VIP Front Row | `BOOKED` | Slate / Occupied | Pre-booked by Demo Customer; ticket code `TKT-VALID-VIP01`. |
| `B2` | Row B, #2 | VIP Front Row | `BOOKED` | Slate / Occupied | Pre-booked (Cancelled order); ticket code `TKT-CANC-NEON03`. |
| `B3` | Row B, #3 | VIP Front Row | `BOOKED` | Slate / Occupied | Occupied seat. |
| `C1`, `C2` | Row C, #1, #2 | General Floor | `BOOKED` | Slate / Occupied | Pre-booked by Alex Chen with `EARLYBIRD20` voucher. |
| `C3` | Row C, #3 | General Floor | `AVAILABLE` | Emerald Green | Open general seat. |
| `C4` | Row C, #4 | General Floor | `BOOKED` | Slate / Occupied | Pending reservation demonstration. |
| `D1`–`D15` | Row D | General Floor | `AVAILABLE` | Emerald Green | Open for multi-seat selection. |
| `E1` | Row E, #1 | Balcony Suite | `BOOKED` | Slate / Occupied | Occupied balcony seat. |
| `E2`–`F10` | Rows E & F | Balcony Suite | `AVAILABLE` | Emerald Green | Open balcony seats. |

---

## 🎟️ 4. Promotional Voucher & Coupon Engine Matrix

Test code validation in the Checkout Modal or via `POST /api/vouchers/validate`:

| Voucher Code | Type | Value | Min Spend | Max Uses | Used | Status | Expected Behavior / Test Case |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `EARLYBIRD20` | Percentage | 20% | $50.00 | 200 | 15 | **Active** | ✅ **Applies 20% off** on carts ≥ $50. |
| `VIP50` | Percentage | 50% | $100.00 | 50 | 10 | **Active** | ✅ **Applies 50% off** on high-tier VIP carts ≥ $100. |
| `SUMMER15` | Percentage | 15% | $30.00 | 300 | 8 | **Active** | ✅ **Applies 15% off** on carts ≥ $30. |
| `SAVE10` | Fixed | $10.00 | $30.00 | 500 | 45 | **Active** | ✅ **Applies $10 flat discount** on carts ≥ $30. |
| `WELCOME5` | Fixed | $5.00 | $0.00 | 1000 | 120 | **Active** | ✅ **Applies $5 off any order** (no minimum spend). |
| `FESTIVAL25` | Fixed | $25.00 | $75.00 | 100 | 22 | **Active** | ✅ **Applies $25 off** festival passes ≥ $75. |
| `CORP100` | Fixed | $100.00 | $300.00 | 50 | 5 | **Active** | ✅ **Applies $100 off** corporate orders ≥ $300. |
| `EXHAUSTED` | Percentage | 30% | $0.00 | 10 | 10 | **Exhausted** | ❌ **Error**: `VOUCHER_EXHAUSTED` (Usage limit 10/10 reached). |
| `EXPIRED2025`| Percentage | 40% | $20.00 | 100 | 12 | **Expired** | ❌ **Error**: `VOUCHER_EXPIRED` (Expired Jan 1, 2025). |
| `INACTIVE50` | Percentage | 50% | $0.00 | 100 | 0 | **Disabled** | ❌ **Error**: `VOUCHER_INACTIVE` (Campaign deactivated). |
| `HIGHMIN500` | Fixed | $150.00| $500.00 | 20 | 2 | **Active** | ❌ **Error**: `MIN_SPEND_NOT_MET` (Requires $500 cart). |

### Example CLI Test Commands:
```bash
# Test Valid Voucher
curl -s -X POST http://localhost:8080/api/vouchers/validate \
  -H "Content-Type: application/json" \
  -d '{"code":"EARLYBIRD20","subtotal":100}' | jq .

# Test Quota Exhausted Voucher
curl -s -X POST http://localhost:8080/api/vouchers/validate \
  -H "Content-Type: application/json" \
  -d '{"code":"EXHAUSTED","subtotal":100}' | jq .

# Test Minimum Spend Failure
curl -s -X POST http://localhost:8080/api/vouchers/validate \
  -H "Content-Type: application/json" \
  -d '{"code":"EARLYBIRD20","subtotal":30}' | jq .
```

---

## 💱 5. Multi-Currency, Regional Tax & PDF Invoicing

The system supports real-time multi-currency checkout, automated regional tax calculations, and corporate tax billing.

| Currency | Rate (USD) | Symbol | Tax Rate | Decimals | Pre-Seeded Test Booking | Corporate Billing Metadata |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| **USD** | 1.00 | `$` | 8.5% | 2 | `bkg-demo-001` ($162.75) | Individual Demo Customer booking. |
| **USD** | 1.00 | `$` | 8.5% | 2 | `bkg-sarah-004` ($648.83) | **Cyberdyne Systems LLC**, Tax ID: `US-EIN-94-3829102`, Sunnyvale, CA. |
| **EUR** | 0.92 | `€` | 20.0% | 2 | `bkg-maria-005` (€420.00) | **Iberia Sports Club**, Tax ID: `ES-B84920182`, Madrid, Spain. |
| **GBP** | 0.79 | `£` | 20.0% | 2 | `bkg-demo-002` (£102.00) | Hamilton London booking in Demo Customer passbook. |
| **JPY** | 155.00 | `¥` | 10.0% | **0** | `bkg-kenji-006` (¥40,920) | Japanese consumption tax, zero decimal formatting. |
| **USD** | 1.00 | `$` | 8.5% | 2 | `bkg-acme-008` ($865.75) | **Acme Corporation**, Tax ID: `US-EIN-13-5928100`, New York, NY. |

### PDF Tax Invoice Downloads:
Download verified PDF 1.4 tax receipts with vector layouts:
```bash
# 1. Download Demo Customer USD Tax Invoice
curl -s http://localhost:8080/api/bookings/bkg-demo-001/invoice/pdf -o invoice-demo.pdf

# 2. Download Sarah Connor Corporate B2B Tax Invoice (contains US-EIN & address)
curl -s http://localhost:8080/api/bookings/bkg-sarah-004/invoice/pdf -o invoice-corporate.pdf

# 3. View JSON Invoice Metadata
curl -s http://localhost:8080/api/bookings/bkg-sarah-004/invoice | jq .
```

---

## 📱 6. Digital Passbook & QR Ticket Verification

Digital admission passes with unique QR ticket codes (`ticket_code`) to test gate check-in:

| Ticket Code | Seat | Booking Ref | Status | Test Case & Gate Check-In Expectation |
| :--- | :--- | :--- | :--- | :--- |
| `TKT-VALID-VIP01` | Row B, #1 (VIP) | `REF-NEON-2026-001` | `VALID` | ✅ **Gate Admission Allowed** (Returns HTTP 200 `VALID`). |
| `TKT-VALID-HAM02` | Row C, #1 (Circle) | `REF-HAM-2026-002` | `VALID` | ✅ **Gate Admission Allowed** (Returns HTTP 200 `VALID`). |
| `TKT-USED-GATE99` | Row K, #1 (Keynote)| `REF-TECH-2026-004` | `USED` | ⚠️ **Double Entry Rejection** (Returns `valid: false`, status `USED`). |
| `TKT-CANC-NEON03` | Row B, #2 (VIP) | `REF-CANC-2026-003` | `CANCELLED`| ❌ **Gate Rejection** (Returns `valid: false`, status `CANCELLED`). |

### Example Gate Scanner & Ticket Transfer Commands:
```bash
# 1. Verify Valid Admission Ticket
curl -s -X POST http://localhost:8080/api/bookings/verify-ticket \
  -H "Content-Type: application/json" \
  -d '{"ticketCode":"TKT-VALID-VIP01"}' | jq .

# 2. Transfer Ticket to Friend
curl -s -X POST http://localhost:8080/api/tickets/transfer \
  -H "Content-Type: application/json" \
  -d '{"ticketCode":"TKT-VALID-VIP01","recipientEmail":"friend@example.com","recipientName":"Lucky Friend"}' | jq .

# 3. Verify Transferred Ticket (Old Code is permanently deactivated)
curl -s -X POST http://localhost:8080/api/bookings/verify-ticket \
  -H "Content-Type: application/json" \
  -d '{"ticketCode":"TKT-VALID-VIP01"}' | jq .

# 4. Verify Already Used Ticket (Anti-Passback check)
curl -s -X POST http://localhost:8080/api/bookings/verify-ticket \
  -H "Content-Type: application/json" \
  -d '{"ticketCode":"TKT-USED-GATE99"}' | jq .
```

---

## ⏳ 7. Sold-Out Events & Waitlist Queue

Event `evt-106` (**Stand-Up Comedy All-Stars Showcase**) is **100% sold out** (12/12 seats booked).

When opening `evt-106` in the frontend or calling the API, the system displays the **Join Waitlist** interface.

### Pre-Seeded Queue for `evt-106`:
1. `wl-001`: **Sarah Connor** (`sarah.connor@cyberdyne.io`) — **Queue Position 1** (`PENDING`, requested 2 seats).
2. `wl-002`: **Alex Chen** (`alex.chen@technova.com`) — **Queue Position 2** (`PENDING`, requested 1 seat).
3. `wl-003`: **Maria Garcia** (`maria.garcia@madrid.es`) — **PROMOTED** (Has allocated reservation hold `res-wl-promoted-003` with active 15-minute countdown to purchase).
4. `wl-004`: **Alex Morgan** (`user@tickets.com`) — **EXPIRED** (Promoted 24h ago but did not complete checkout).

### Inspect Waitlist via Gateway:
```bash
curl -s "http://localhost:8080/api/waitlist?eventId=evt-106" | jq .
```

---

## 📊 8. Admin & Organizer Studio Telemetry

Login as `admin@tickets.com` (`admin123`) and click the **Organizer Studio** button in the header (or query `/api/admin/analytics`):

### Pre-Seeded KPIs:
- **Gross Revenue**: **$43,223.49**
- **Total Bookings**: **10** (7 Confirmed, 1 Cancelled, 1 Pending, 1 Expired)
- **Total Tickets Sold**: **12**
- **Active Events**: **6**
- **Average Occupancy**: **29.25%**
- **Live Order Feed**: Real-time stream of recent orders from NYC, Berlin, London, Tokyo, and Madrid with currencies, totals, and timestamps.
- **Occupancy Breakdown**:
  - `evt-106`: **100% Occupancy** (Waitlist active)
  - `evt-108`: **100% Occupancy** (Completed event)
  - `evt-102`: **16% Occupancy**
  - `evt-103`: **15% Occupancy**
  - `evt-101`: **14.3% Occupancy**
  - `evt-104`: **12% Occupancy**
  - `evt-105`: **6% Occupancy**
  - `evt-107`: **0% Occupancy** (Paused)
  - `evt-109`: **0% Occupancy** (Cancelled)

### Inspect Live Analytics:
```bash
curl -s http://localhost:8080/api/admin/analytics | jq .
```

---

## 🧪 9. Automated Regression & Concurrency Verification

Run the full end-to-end test suite against the running environment at any time:

```bash
python3 scripts/verify_e2e.py
```

All 11 test assertions (API health, frontend web delivery, catalog retrieval, booking creation, exact inventory decrement, duplicate seat conflict 409 rejection, capacity limit 400 rejection, missing event 404, malformed payload 400, and 10-thread single-seat race contention with zero overselling) will execute and verify **100% PASS**.
