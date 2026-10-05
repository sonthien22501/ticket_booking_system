# Event Ticket Booking System (Microservices)

A high-performance, containerized event ticket booking platform built with a microservices architecture. Features real-time seat reservation with atomic concurrency control, distributed saga orchestration, an API Gateway reverse proxy, and an interactive React + TypeScript single-page application.

---

## 1. System Architecture

```
                    [ Web Browser / E2E Client ]
                                │
                    ┌───────────┴───────────┐
                    │                       │
                    ▼ :3000                 ▼ :8080
            ┌───────────────┐       ┌────────────────┐
            │ React + Vite  │       │  API Gateway   │
            │ SPA (Nginx)   │       │    (Nginx)     │
            └───────────────┘       └───────┬────────┘
                                            │ Reverse Proxy Routes
                     ┌──────────────────────┼──────────────────────┐
                     ▼ :8081                ▼ :8082                ▼ :8083
            ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
            │  Event Catalog  │    │ Seat Inventory  │    │ Booking Service │
            │     Service     │    │     Service     │    │ (Saga Manager)  │
            └────────┬────────┘    └────────┬────────┘    └────────┬────────┘
                     │                      │   ▲                  │
                     │                      │   └─── Reserve/Commit┘
                     ▼                      ▼                      ▼
            ┌───────────────────────────────────────────────────────────────┐
            │                 PostgreSQL 16 Relational DB                   │
            │                  Port 5432 (ticket_db)                        │
            └───────────────────────────────────────────────────────────────┘
```

### Core Services

| Service | Runtime / Stack | Port | Description |
|---|---|---|---|
| **`postgres`** | PostgreSQL 16 Alpine | `5432` | Relational storage for events, ticket tiers, seat items, bookings, and digital tickets. Seeded via `infra/postgres/init.sql`. |
| **`catalog-service`** | Go 1.22 / Alpine | `8081` | Manages event information, ticket tiers, search queries, and capacity metadata. |
| **`inventory-service`** | Go 1.22 / Alpine | `8082` | Atomic seat reservation, row-level locks, per-event mutex synchronization, hold expiration janitor, and zero-oversell guarantees. |
| **`booking-service`** | Go 1.22 / Alpine | `8083` | Saga orchestrator managing booking creation, inventory reservation/commit, payment simulation, and ticket generation. |
| **`api-gateway`** | Nginx 1.25 Alpine | `8080` | Unified public ingress point routing `/health`, `/api/events`, `/api/inventory`, and `/api/bookings` with full CORS support. |
| **`frontend`** | React 18 / TypeScript / Vite / Nginx | `3000` | Interactive web frontend featuring event catalog, interactive venue seat map, sticky checkout drawer, and real-time 409 conflict handling. |

---

## 2. Quick Start

### Prerequisites
- Docker Engine 24+ and Docker Compose v2+
- Python 3 (for running automated E2E verification tests)

### Launch the Entire Cluster
```bash
# Clone or navigate to the repository root
cd ~/teamwork_projects/ticket_booking_system

# Build and start all 6 services with health checks
docker compose up -d --build
```

### Check Service Health
```bash
docker compose ps
```
All containers (`ticket_postgres`, `ticket_catalog`, `ticket_inventory`, `ticket_booking`, `ticket_gateway`, `ticket_frontend`) will report `healthy`.

### Stop and Wipe Cluster
```bash
docker compose down -v
```

---

## 3. Endpoints & API Contracts

All microservice requests pass through the **API Gateway** on port `8080`:

| Method | Endpoint | Description | Status Code |
|---|---|---|---|
| `GET` | `/health` | API Gateway health check | `200 OK` |
| `GET` | `/api/events` | List all events (optional `?search=` and `?category=`) | `200 OK` |
| `GET` | `/api/events/{id}` | Get specific event details | `200 OK` / `404` |
| `GET` | `/api/inventory/{eventId}` | Get real-time seat availability & tier summary | `200 OK` / `404` |
| `GET` | `/api/inventory/{eventId}/seats` | Get individual seat states (`AVAILABLE`, `HOLD`, `BOOKED`) | `200 OK` |
| `POST` | `/api/bookings` | Create booking (atomic hold -> payment -> commit) | `201 CREATED` / `409 CONFLICT` / `400 BAD REQUEST` |
| `GET` | `/api/bookings/{id}` | Retrieve booking status and tickets | `200 OK` / `404` |

### Sample Booking Request
```bash
curl -X POST http://localhost:8080/api/bookings \
  -H "Content-Type: application/json" \
  -d '{
    "eventId": "evt-101",
    "seats": ["A1", "A2"],
    "ticketCount": 2,
    "customerName": "Alice Developer",
    "customerEmail": "alice@example.com"
  }'
```

---

## 4. Web Frontend (`http://localhost:3000`)

The frontend is an interactive Single Page Application built with React 18, TypeScript, and Vite, served via an optimized Nginx multi-stage build:
- **Event Discovery**: Search and category filtering across live events.
- **Interactive Venue Seat Map**: Visual curved stage representation, VIP tier differentiation (Row A/B) vs General Admission (Row C-F), seat tooltips, and real-time status indication.
- **Atomic Reservation & Conflict Handling**: If another user books a selected seat concurrently, the UI receives an HTTP 409 Conflict, displays a clear notification banner, automatically reloads live seat availability, and disables conflicting seats.

---

## 5. Automated E2E Verification

The project includes an opaque-box verification test harness in `scripts/verify_e2e.py` executed via `scripts/run_e2e_tests.sh`.

### Run Verification Suite
```bash
# Execute via shell runner
bash scripts/run_e2e_tests.sh

# Or directly via Python 3
python3 scripts/verify_e2e.py
```

### Verified Acceptance Tiers
1. **Tier 1 (Smoke & Liveness)**: API Gateway `/health` (HTTP 200) and Frontend `/` (HTTP 200 HTML DOM).
2. **Tier 2 (Acceptance Criteria)**: Catalog fetch, initial seat count $S_0$, booking creation (`HTTP 201 CONFIRMED`), and verification that seat inventory accurately decreased to $S_0 - Q$.
3. **Tier 3 (Boundary & Corner Cases)**: Duplicate booking rejection (`HTTP 409 Conflict`), overbooking rejection (`HTTP 400/409`), nonexistent event (`HTTP 404`), and malformed payload (`HTTP 400`).
4. **Tier 4 (High-Concurrency Contention)**: 10 concurrent threads simultaneously racing for a single remaining seat; exactly 1 winner receives `HTTP 201 CONFIRMED`, 9 receive `HTTP 409 Conflict`, with zero overselling.

---

## 6. Directory Layout

```
ticket_booking_system/
├── docker-compose.yml          # Container orchestration with healthcheck graph
├── README.md                   # System documentation
├── infra/
│   └── postgres/
│       └── init.sql            # PostgreSQL DDL and deterministic seed data
├── backend/
│   ├── catalog-service/        # Go catalog microservice (port 8081)
│   ├── inventory-service/      # Go seat inventory microservice (port 8082)
│   ├── booking-service/        # Go saga booking microservice (port 8083)
│   └── api-gateway/            # Nginx reverse proxy gateway (port 8080)
├── frontend/                   # React + TypeScript + Vite SPA (port 3000)
│   ├── Dockerfile              # Multi-stage Node builder + Nginx runner
│   ├── nginx.conf              # SPA routing & API reverse proxy
│   └── src/                    # Components, seat map, and API services
└── scripts/
    ├── verify_e2e.py           # Multi-tier E2E opaque-box test harness
    └── run_e2e_tests.sh        # Automated execution runner
```
