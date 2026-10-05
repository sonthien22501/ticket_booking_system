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
- `POST /api/bookings` -> `http://booking-service:8083/bookings`
- `GET /api/bookings/:id` -> `http://booking-service:8083/bookings/:id`

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
