# Milestone 14: Standardized OpenAPI 3.0 / Swagger Documentation (R14.1) Implementation Plan

> **For agentic workers:** Execute this plan task-by-task. Follow strict RED-GREEN TDD and karpathy-guidelines.

**Goal:** Implement standardized OpenAPI 3.0 specification and interactive Swagger UI (R14.1) served directly through the API Gateway, documenting all microservices (Auth, Catalog, Inventory, Booking, Waitlist, and DLQ) with parameter schemas, models, idempotency headers, correlation IDs, and Bearer JWT authentication schemes.

**Architecture:** A comprehensive OpenAPI 3.0 specification (`openapi.yaml` and `openapi.json`) is maintained and served by NGINX API Gateway at `/docs/` and `/api/docs/` alongside a high-performance Swagger UI distribution. Go automated E2E tests validate spec integrity, schema completeness across all services, route coverage, and Gateway delivery.

**Tech Stack:** OpenAPI 3.0.3, Swagger UI, NGINX 1.25, Go 1.22+ (`gopkg.in/yaml.v3` / standard library JSON test validation), Docker Compose.

**Spec:** `REQUIREMENTS.md` (Requirement R14.1: Standardized API Documentation).

---

## Global Constraints
- Target Requirement: R14.1 ("All microservice APIs must be documented using the OpenAPI Specification (Swagger) to facilitate contract-driven development and frontend integration.")
- Delivery Entrypoints: Must be accessible via API Gateway at `http://localhost:8080/docs/`, `http://localhost:8080/api/docs/`, `http://localhost:8080/docs/openapi.yaml`, and `http://localhost:8080/docs/openapi.json`.
- Complete Schema Coverage: Must document 100% of production endpoints across Auth, Catalog, Inventory, Booking, Waitlist, and DLQ services.
- Traceability & Idempotency: All mutating endpoints document `Idempotency-Key` and `X-Correlation-ID` headers.
- Security Schemas: Must declare `BearerAuth` (HTTP Bearer JWT) with RBAC scopes/roles (`admin`, `customer`).

---

## Review Focus (Top 5 Edge Cases & Verification Criteria)
1. **Schema Completeness & Route Coverage**: All 6 API subsystems (Auth, Events, Inventory, Bookings, Waitlist, DLQ) must have full method and path representation in `openapi.yaml`.
2. **Dual Routing & Trailing Slash Redirection**: API Gateway must cleanly serve requests to `/docs`, `/docs/`, `/api/docs`, and `/api/docs/` without 404 or routing loops.
3. **Format Parity (YAML and JSON)**: Both `openapi.yaml` and `openapi.json` must be served with correct content-types (`application/yaml` / `text/yaml`, `application/json`) and identical structure.
4. **Idempotency & Header Declarations**: Mutating endpoints (`/api/bookings`, `/api/inventory/reserve`, `/api/waitlist`) must explicitly document `Idempotency-Key` and `X-Correlation-ID`.
5. **Swagger UI Standalone Rendering**: The HTML entrypoint at `/docs/` must render Swagger UI with spec URL pointing to `/docs/openapi.yaml` without script errors.

---

### Task 1: Comprehensive OpenAPI 3.0 Specification (`openapi.yaml` & `openapi.json`)

**Files:**
- Create: `backend/api-gateway/swagger/openapi.yaml`
- Create: `backend/api-gateway/swagger/openapi.json`

**Details:**
- Write an OpenAPI 3.0.3 specification documenting:
  - Info: Title "Distributed Ticket Booking System API", Version "1.0.0", Description.
  - Servers: `url: http://localhost:8080` (API Gateway).
  - Security Schemes: `BearerAuth` (type: http, scheme: bearer, bearerFormat: JWT).
  - Components Schemas:
    - User, RegisterRequest, LoginRequest, AuthResponse, UserClaims
    - Event, EventTier, EventListResponse
    - SeatItem, InventoryResponse, ReserveSeatsRequest, ReservationResponse
    - Booking, CreateBookingRequest, BookingTicket, CancelBookingResponse
    - WaitlistEntry, JoinWaitlistRequest, WaitlistStatusResponse, ClaimWaitlistResponse
    - DLQMessage, DLQStatsResponse, DLQRetryResponse
    - ErrorResponse
  - Paths:
    - `/api/auth/register`, `/api/auth/login`, `/api/auth/verify`, `/api/auth/me`, `/api/auth/users`
    - `/api/events`, `/api/events/{id}`
    - `/api/inventory/{eventId}`, `/api/inventory/{eventId}/seats`, `/api/inventory/reserve`, `/api/inventory/commit`, `/api/inventory/release`, `/api/inventory/cancel`, `/api/inventory/reservations/{id}`, `/api/inventory/janitor/sweep`
    - `/api/bookings`, `/api/bookings/{id}`, `/api/bookings/{id}/cancel`
    - `/api/waitlist`, `/api/waitlist/{id}`, `/api/waitlist/{id}/claim`, `/api/waitlist/{id}/cancel`
    - `/api/dlq/stats`, `/api/dlq/messages`, `/api/dlq/retry`, `/api/dlq/simulate`
    - `/health`

---

### Task 2: Interactive Swagger UI Web Page

**Files:**
- Create: `backend/api-gateway/swagger/index.html`

**Details:**
- Self-contained HTML bundling Swagger UI from unpkg/cdnjs CDN (`swagger-ui-dist@5`), pre-configured to load `./openapi.yaml`.
- Includes custom styling matching the ticket booking system palette.

---

### Task 3: API Gateway Nginx Configuration & Dockerfile Updates

**Files:**
- Modify: `backend/api-gateway/nginx.conf`
- Modify: `backend/api-gateway/Dockerfile`
- Modify: `docker-compose.yml`

**Details:**
- In `backend/api-gateway/Dockerfile`:
  - `COPY swagger/ /usr/share/nginx/html/docs/`
- In `backend/api-gateway/nginx.conf`:
  - Configure `location /docs/` and `location /api/docs/` serving `/usr/share/nginx/html/docs/`.
  - Add rewrites:
    `rewrite ^/docs$ /docs/ permanent;`
    `rewrite ^/api/docs$ /api/docs/ permanent;`
    `rewrite ^/api/docs/(.*)$ /docs/$1 last;`
  - Set MIME types for `.yaml` / `.yml` to `text/yaml; charset=utf-8` and `.json` to `application/json`.
- In `docker-compose.yml`:
  - Ensure volume mount `./backend/api-gateway/swagger:/usr/share/nginx/html/docs:ro` so updates reflect immediately during local dev.

---

### Task 4: Automated E2E Swagger Verification Test Suite (RED -> GREEN)

**Files:**
- Create: `e2e/swagger_test.go`

**Details:**
- Write Go E2E tests:
  - `TestSwaggerUIDelivery`: Asserts `GET http://localhost:8080/docs/` and `http://localhost:8080/api/docs/` return HTTP 200 with HTML containing `swagger-ui`.
  - `TestOpenAPISpecDelivery`: Asserts `GET http://localhost:8080/docs/openapi.yaml` and `http://localhost:8080/docs/openapi.json` return HTTP 200 with valid content.
  - `TestOpenAPISchemaIntegrityAndCoverage`: Parses the delivered spec and validates that all 6 services and all critical endpoints exist.
  - `TestOpenAPISecurityAndIdempotencyHeaders`: Validates security scheme declaration and `Idempotency-Key` / `X-Correlation-ID` header docs on mutating routes.
- Run tests: `cd e2e && go test -v -run TestSwagger ./...`

---

### Task 5: Documentation & Milestone Sign-Off

**Files:**
- Modify: `REQUIREMENTS.md` (Check off R14.1 with ✅)
- Modify: `explain.md` (Add detailed Milestone 14 architectural guide)
