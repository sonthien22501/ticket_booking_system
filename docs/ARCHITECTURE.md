# Project: Event Ticket Booking System (Microservices)

## Architecture
Microservices architecture in `~/teamwork_projects/ticket_booking_system` coordinating 7 containerized services connected via Docker bridge network (`ticket-net`):
1. **API Gateway (`api-gateway`)** — Port 8080: Unified reverse proxy (Nginx / Go proxy) routing public traffic to backend services with CORS and health checks.
2. **Event Catalog Service (`catalog-service`)** — Port 8081: Go service managing events, venues, ticket tiers, metadata, and catalog seeding.
3. **Seat Inventory Service (`inventory-service`)** — Port 8082: Go service with high-concurrency atomic seat reservation, row-level locks, Redis atomic counters, and TTL lease janitor.
4. **Booking Service (`booking-service`)** — Port 8083: Go Saga orchestrator coordinating checkout, payment simulation, ticket emission, and booking lifecycle.
5. **PostgreSQL 16 Database (`postgres`)** — Port 5432: Relational storage for catalog, inventory, and booking data with initial seed data.
6. **Redis 7 Cache (`redis`)** — Port 6379: High-throughput atomic counter and distributed locks.
7. **Frontend (`frontend`)** — Port 3000: React + TypeScript (Vite) single-page app served via Nginx with interactive seat map and booking checkout.

```
[ Client / Web Browser / Test Harness ]
                   │
                   ▼ :8080
            [ API Gateway ]
          ┌────────┼────────┐
          │ :8081  │ :8082  │ :8083
          ▼        ▼        ▼
     [ Catalog ] [Inventory] [Booking]
          │        │        │
          └────────┼────────┘
                   ▼
         [ Postgres / Redis ]
```

---

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F1 | DB Schema & Seed Data | PostgreSQL schema (`events`, `ticket_tiers`, `event_inventory`, `seat_items`, `bookings`, `tickets`) and seed data | M1 | Survey |
| F2 | Event Catalog Service | CRUD, search, category filters, venue metadata, `/health`, `/api/events` | M2 | Survey / R1 |
| F3 | Seat Inventory Service | Real-time seat grid, aggregate counts, atomic hold/reserve with TTL, commit/release, zero-oversell concurrency | M3 | Survey / R1 |
| F4 | Booking Saga Service | Orchestrate booking checkout, call inventory hold, simulate payment, commit inventory, emit digital tickets | M4 | Survey / R1 |
| F5 | API Gateway | Single entrypoint port 8080, reverse proxy routes (`/api/events`, `/api/inventory`, `/api/bookings`), CORS headers, gateway `/health` | M5 | Survey / R1 |
| F6 | React + TypeScript Frontend | Vite + TS app on port 3000, event list, interactive venue seat map, booking checkout modal, real-time conflict handling (409) | M6 | Survey / R2 |
| F7 | Docker Compose Orchestration | Single `docker-compose.yml` with health checks, `condition: service_healthy` startup hierarchy across all 7 services | M7 | Survey / R3 |
| F8 | Automated E2E Verification Harness | Python & Bash test harness executing against Gateway port 8080 (catalog fetch, booking creation, seat decrement verification, boundary conflicts) and Frontend port 3000 | M8 | Acceptance Criteria |
| F26 | Multi-Currency Checkout & PDF Invoicing | Multi-currency conversions (USD, EUR, GBP, JPY, CAD, AUD), automated regional VAT/sales tax, corporate billing, and downloadable/printable PDF tax invoices | M26 | Survey / R4 |
| F27 | Waitlist Self-Service & Secure Ticket Transfer | Frontend sold-out detection, waitlist queue joining, live position tracker in Passbook, atomic ticket transfer with code rotation & anti-fraud gate admission | M27 | Acceptance Criteria |
| F28 | In-App Notification Center & Calendar Sync | Real-time in-app notification center (bell badge, order alerts, waitlist promotions, ticket transfers) & standard RFC 5545 iCalendar (.ics) pass export | M28 | Acceptance Criteria |
| F29 | VIP Add-ons, Merchandise & Concessions | Curated upsells (priority parking, event t-shirts, VIP lounge access), atomic inventory deduction, cart subtotal itemization & confirmation | M29 | Acceptance Criteria |
| F30 | Attendee Reviews & Verified Buyer Ratings | 1-5 star attendee reviews, automated verified buyer check against confirmed bookings, rating score aggregation, rating breakdown matrix & helpful voting | M30 | Acceptance Criteria |

---

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Database & Seed Infra | Database schema `infra/postgres/init.sql` and Docker setup | None | DONE |
| M2 | Event Catalog Service | Go implementation in `backend/catalog-service/` with Dockerfile | M1 | DONE |
| M3 | Seat Inventory Service | Go implementation in `backend/inventory-service/` with high-concurrency locks & Dockerfile | M1 | DONE |
| M4 | Booking Service | Go implementation in `backend/booking-service/` with Saga & Dockerfile | M2, M3 | DONE |
| M5 | API Gateway | Nginx gateway in `backend/api-gateway/` with routing & CORS | M2, M3, M4 | DONE |
| M6 | React/TypeScript Frontend | React + Vite + TS app in `frontend/` with seat map & Dockerfile | M5 | DONE |
| M7 | Docker Compose Containerization | Complete `docker-compose.yml` and integration scripts | M1, M2, M3, M4, M5, M6 | DONE |
| M8 | Final Acceptance & E2E Verification | Automated verification script `scripts/verify_e2e.py` validating 100% acceptance criteria under docker-compose up | M7 | DONE |
| M21 | Frontend Auth & User Profile Integration | React AuthModal, Header profile badge, token storage & checkout pre-fill (R3.1) | M5, M6 | DONE |
| M22 | Cinematic Frontend Redesign | Design system tokens, curved cinema screen, tactile luminous seats, glassmorphism modals | M6, M21 | DONE |
| M23 | User Bookings, Passbook & Gate QR Ticket Verification | MyBookingsModal history, TicketPassModal QR passes, gate check-in API, and Saga cancellation | M4, M6, M21, M22 | DONE |
| M24 | Admin & Organizer Studio & Live Telemetry | Admin analytics API, automated event & 2D seat provisioning, event status management, and Organizer Studio modal | M2, M4, M5, M6, M21, M23 | DONE |
| M25 | Promotional Voucher & Coupon Engine | Percentage/fixed discount codes, minimum spend thresholds, usage limits, and real-time checkout cart calculation with anti-stacking protection | M4, M5, M6, M21, M24 | DONE |
| M26 | Multi-Currency Checkout & PDF Invoicing | Currency conversion engine, regional tax calculation, corporate expense billing, pure Go vector PDF invoices (%PDF-1.4), frontend currency picker, and 1-click invoice downloads | M4, M5, M6, M21, M25 | DONE |
| M27 | Waitlist Self-Service & Secure Ticket Transfer | Sold-out event waitlist join modal, queue position tracking, promoted hold claim flow, atomic ticket transfer with QR code invalidation & gate check-in verification | M4, M5, M6, M21, M23, M26 | DONE |
| M28 | In-App Notification Center & Calendar Sync | Persistent notifications store, automated event triggers, unread counter badges, glassmorphism activity drawer, and RFC 5545 iCalendar (.ics) pass downloads | M4, M5, M6, M21, M23, M27 | DONE |
| M29 | VIP Add-ons, Merchandise & Concessions | Curated upsells, atomic inventory locking/deduction, order itemization, and checkout upsell engine | M4, M5, M6, M21, M25, M28 | DONE |
| M30 | Attendee Reviews & Verified Buyer Ratings | Verified buyer checks, star rating aggregations, rating breakdown distributions, and interactive social proof modal | M4, M5, M6, M21, M23, M29 | DONE |

---

## Interface Contracts

### 1. API Gateway ↔ Microservices
- `GET /health` -> Gateway local health response `{ "status": "UP", "gateway": "ok" }`
- `GET /api/events` -> `http://catalog-service:8081/events`
- `GET /api/events/:id` -> `http://catalog-service:8081/events/:id`
- `GET /api/inventory/:eventId` -> `http://inventory-service:8082/inventory/:eventId`
- `GET /api/inventory/:eventId/seats` -> `http://inventory-service:8082/inventory/:eventId/seats`
- `POST /api/inventory/reserve` -> `http://inventory-service:8082/inventory/reserve`
- `POST /api/inventory/commit` -> `http://inventory-service:8082/inventory/commit`
- `POST /api/inventory/release` -> `http://inventory-service:8082/inventory/release`
- `POST /api/bookings` -> `http://booking-service:8083/bookings` (supports optional `voucherCode` with discount deduction)
- `GET /api/bookings` -> `http://booking-service:8083/bookings` (filters by userId / email / JWT)
- `GET /api/bookings/:id` -> `http://booking-service:8083/bookings/:id`
- `POST /api/bookings/:id/cancel` -> `http://booking-service:8083/bookings/:id/cancel` (Saga inventory compensation)
- `POST /api/bookings/verify-ticket` -> `http://booking-service:8083/bookings/verify-ticket` (gate scanner check-in)
- `POST /api/vouchers/validate` -> `http://booking-service:8083/vouchers/validate` (validates code against subtotal, returns discount amount & final total)
- `GET /api/admin/vouchers` -> `http://booking-service:8083/admin/vouchers` (retrieves active promotional codes & redemption counts)
- `POST /api/admin/vouchers` -> `http://booking-service:8083/admin/vouchers` (provisions new campaign codes with usage limits & spend rules)
- `GET /api/admin/analytics` -> `http://booking-service:8083/admin/analytics` (gross revenue, occupancy %, active events, and recent orders)
- `PUT /api/events/:id/status` -> `http://catalog-service:8081/events/:id/status` (lifecycle toggle: ACTIVE, PAUSED, COMPLETED, CANCELLED)
- `GET /api/currencies/rates` -> `http://booking-service:8083/currencies/rates` (supported currency rates, tax rates, and decimal precision)
- `POST /api/currencies/convert` -> `http://booking-service:8083/currencies/convert` (real-time amount conversion)
- `GET /api/bookings/:id/invoice` -> `http://booking-service:8083/bookings/:id/invoice` (structured invoice metadata)
- `GET /api/bookings/:id/invoice/pdf` -> `http://booking-service:8083/bookings/:id/invoice/pdf` (binary %PDF-1.4 tax invoice download)
- `GET /api/bookings/:id/calendar.ics` -> `http://booking-service:8083/bookings/:id/calendar.ics` (standard RFC 5545 iCalendar pass export)
- `POST /api/tickets/transfer` -> `http://booking-service:8083/tickets/transfer` (atomic ticket transfer, old code invalidation, new code generation)
- `GET /api/tickets/transfers` -> `http://booking-service:8083/tickets/transfers` (audit transfer logs by user or booking)
- `POST /api/waitlist` -> `http://booking-service:8083/waitlist` (join priority waitlist for sold-out events)
- `GET /api/waitlist` -> `http://booking-service:8083/waitlist` (query waitlist entries and live queue positions)
- `DELETE /api/waitlist/:id` -> `http://booking-service:8083/waitlist/:id` (cancel waitlist entry)
- `POST /api/waitlist/:id/claim` -> `http://booking-service:8083/waitlist/:id/claim` (claim promoted hold and confirm booking)
- `GET /api/notifications` -> `http://booking-service:8083/notifications` (queries notifications and unread badge count)
- `POST /api/notifications/:id/read` -> `http://booking-service:8083/notifications/:id/read` (marks specific notification as read)
- `POST /api/notifications/read-all` -> `http://booking-service:8083/notifications/read-all` (marks all user notifications as read)
- `GET /api/events/:id/addons` -> `http://booking-service:8083/events/:id/addons` (lists available VIP add-ons and event merchandise with inventory)
- `GET /api/addons` -> `http://booking-service:8083/addons` (retrieves all system add-on packages)
- `GET /api/events/:id/reviews` -> `http://booking-service:8083/events/:id/reviews` (fetches attendee reviews, star distribution, and average rating)
- `POST /api/events/:id/reviews` -> `http://booking-service:8083/events/:id/reviews` (submits attendee review with verified buyer validation)
- `POST /api/reviews/:id/vote` -> `http://booking-service:8083/reviews/:id/vote` (increments review helpful vote count)

### 2. Booking Service ↔ Inventory Service
- **Reserve Hold**: `POST http://inventory-service:8082/inventory/reserve`
  - Body: `{ "eventId": string, "seats": string[], "ticketCount": number, "holdSeconds": 600 }`
  - Returns 200: `{ "reservationId": string, "expiresAt": string }`
  - Returns 409: `{ "error": "SEAT_UNAVAILABLE" }`
- **Commit**: `POST http://inventory-service:8082/inventory/commit`
  - Body: `{ "reservationId": string, "bookingId": string }`
  - Returns 200: `{ "status": "COMMITTED" }`
- **Release**: `POST http://inventory-service:8082/inventory/release`
  - Body: `{ "reservationId": string }`
  - Returns 200: `{ "status": "RELEASED" }`

---

## Code Layout
Project root: `~/teamwork_projects/ticket_booking_system`
```
~/teamwork_projects/ticket_booking_system/
├── docker-compose.yml
├── README.md
├── infra/
│   └── postgres/
│       └── init.sql
├── backend/
│   ├── catalog-service/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── go.sum
│   │   └── main.go
│   ├── inventory-service/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── go.sum
│   │   └── main.go
│   ├── booking-service/
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   ├── go.sum
│   │   └── main.go
│   └── api-gateway/
│       ├── Dockerfile
│       └── nginx.conf
├── frontend/
│   ├── Dockerfile
│   ├── nginx.conf
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── index.html
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── types/
│       ├── components/
│       └── services/
└── scripts/
    ├── verify_e2e.py
    └── run_e2e_tests.sh
```
