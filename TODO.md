# Project Roadmap & TODOs

## Phase 1: MVP Core (COMPLETED)
- [x] Basic Go Microservices (Catalog, Inventory, Booking)
- [x] NGINX API Gateway
- [x] PostgreSQL database and schemas
- [x] Atomic Seat Reservation (Row-level locking)
- [x] React + TypeScript Frontend with Interactive Seat Map
- [x] Docker Compose orchestration
- [x] End-to-End Test Suite (`verify_e2e.py`)

## Phase 2: User Authentication & Departments
- [ ] Create `Auth Service` (Go) to issue JWT tokens.
- [ ] Update API Gateway to validate JWT signatures.
- [ ] Implement Role-Based Access Control (RBAC) (e.g., standard user vs Department Admin).
- [ ] Update Frontend to include Login/Registration pages.

## Phase 3: Domain Expansion (Movies & Music)
- [ ] Update PostgreSQL `events` schema to support `event_type` (Movie, Music, Sports).
- [ ] Update Event Catalog Service to filter by `event_type`.
- [ ] Add specific metadata for Movies (e.g., Director, Runtime) and Music (e.g., Artist, Genre).
- [ ] Update React Frontend to display different UI layouts depending on the `event_type`.

## Phase 4: Production Readiness
- [ ] Implement Rate Limiting at the API Gateway to prevent bot scraping.
- [ ] Set up GitHub Actions CI pipeline for automated testing.
- [ ] Implement OpenTelemetry tracing across all microservices.
