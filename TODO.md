# Project Roadmap & TODOs

## Phase 1: MVP Core (COMPLETED)
- [x] Basic Go Microservices (Catalog, Inventory, Booking)
- [x] NGINX API Gateway
- [x] PostgreSQL database and schemas
- [x] Atomic Seat Reservation (Row-level locking)
- [x] React + TypeScript Frontend with Interactive Seat Map
- [x] Docker Compose orchestration
- [x] End-to-End Test Suite (`verify_e2e.py`)

## Phase 2: User Authentication & Departments (COMPLETED)
- [x] Create `Auth Service` (Go) to issue JWT tokens.
- [x] Update API Gateway to validate JWT signatures.
- [x] Implement Role-Based Access Control (RBAC) (e.g., standard user vs Department Admin).
- [x] Update Frontend to include Login/Registration pages.

## Phase 3: Domain Expansion (Movies & Music)
- [x] Update PostgreSQL `events` schema to support `event_type` (Movie, Music, Sports).
- [x] Update Event Catalog Service to filter by `event_type`.
- [x] Add specific metadata for Movies (e.g., Director, Runtime) and Music (e.g., Artist, Genre).
- [x] Update React Frontend to display different UI layouts depending on the `event_type`.

## Phase 4: Production Readiness (COMPLETED)
- [x] Implement Rate Limiting at the API Gateway to prevent bot scraping.
- [x] Set up GitHub Actions CI pipeline for automated testing.
- [x] Implement OpenTelemetry tracing across all microservices.

