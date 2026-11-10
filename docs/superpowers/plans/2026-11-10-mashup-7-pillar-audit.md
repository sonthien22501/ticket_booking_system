# Mash-Up Engineering Compliance Upgrade Plan (7-Pillar Audit & Remediation)

**Baseline Commit (`BASE_SHA`)**: `e21f4bf`
**Target Branch**: `feature/mashup-architecture-compliance`

## Overview & Gap Analysis Against the 7 Course Pillars

1. **Pillar 1 — Avoiding the "Distributed Monolith" & Database-per-Service**:
   - *Current State*: `docker-compose.yml` configures `DB_SCHEMA` per service (`catalog_schema`, `inventory_schema`, `booking_schema`, `auth_schema`, `notification_schema`) and `POST /inventory/provision` is wired.
   - *Gaps*:
     1. In `backend/booking-service/`, `calendar.go` (`SELECT ... FROM events`), `main.go` (`getEventMetadata` and `handleAdminAnalytics`), `waitlist.go` (`tryAutoPromoteWaitlist` querying `reservations`, `seat_items`, `events`, `ticket_tiers`), and `saga_orchestrator.go` (`SELECT seat_id FROM seat_items`) still issue fallback/direct SQL queries against `events`, `ticket_tiers`, `event_inventory`, `reservations`, and `seat_items`.
     2. In `backend/inventory-service/main.go`, `seedDefaultInventoryIfEmpty` queries `events` and `ticket_tiers` directly.
     3. In `backend/catalog-service/`, `main.go` (`fetchTiers` lines 540, 557) and `pricing.go` (`line 152`) query `event_inventory` and `seat_items`.
   - *Remediation*:
     - Route cross-service lookups in `catalog-service`, `inventory-service`, and `booking-service` through REST HTTP helpers (`INVENTORY_SERVICE_URL`, `CATALOG_SERVICE_URL`) with strict schema boundaries.
     - Update `infra/postgres/init.sql` so each bounded-context schema owns its exclusive domain tables/views cleanly without cross-service table leakage.

2. **Pillar 2 — API Gateway Pattern (Centralized Cross-Cutting Concerns)**:
   - *Current State*: `backend/api-gateway/nginx.conf` defines `limit_req_zone` rate limiters and `location = /_auth_verify` (`GET /auth/verify`) and `location = /_auth_verify_admin` (`GET /auth/verify-admin`).
   - *Gaps*:
     - Several locations in `backend/api-gateway/nginx.conf` (`/api/waitlist`, `/api/vouchers`, `/api/currencies`, `/api/concierge`, `/api/tickets`, `/api/notifications`, `/api/addons`, `/api/reviews`, `/api/dlq`) were missing `auth_request /_auth_verify;` and/or `limit_req` directives.
   - *Remediation*:
     - Wire `auth_request /_auth_verify;`, `auth_request_set` identity propagation headers (`X-Verified-User-Id`, `X-Verified-User-Email`, `X-Verified-User-Role`), and `limit_req` across all remaining API locations in `backend/api-gateway/nginx.conf`.

3. **Pillar 3 — OpenAPI 3.0 Specifications & Agent-Ready Contracts**:
   - *Current State*: `docs/openapi.yaml`, `backend/api-gateway/swagger/openapi.json`, and `backend/*/openapi.json` exist and `/api/docs` serves Swagger UI.
   - *Gaps*:
     1. Per-service `backend/*/openapi.json` files (`catalog-service`, `inventory-service`, `booking-service`, `auth-service`, `notification-service`) have bare properties (missing `description` and concrete `example` on every DTO property) and do not declare `$ref` on `requestBody` and `responses` per status code (`200`, `201`, `204`, `400`, `401`, `403`, `404`, `409`, `500`) or path/query `parameters`.
     2. `docs/openapi.yaml` and `backend/api-gateway/swagger/openapi.json` have a few schema properties without `description` or `example` and missing `PUT /api/events/{id}/status`, `PATCH /api/inventory/seats/{seatId}`, `DELETE /api/bookings/{id}`, `DELETE /api/waitlist/{id}`.
     3. `scripts/validate_openapi.py` does not strictly fail when a schema property lacks `description` or `example`, or when `POST`/`PUT`/`PATCH` operations lack `$ref` in `requestBody`.
   - *Remediation*:
     - Upgrade `docs/openapi.yaml`, `backend/api-gateway/swagger/openapi.yaml`, `backend/api-gateway/swagger/openapi.json`, and all 5 `backend/*/openapi.json` specifications so 100% of paths declare parameters, `$ref` request bodies, `$ref` response schemas per status code, and every DTO property in `components.schemas` has both a detailed `description` and concrete `example`.
     - Enhance `scripts/validate_openapi.py` to enforce that every property in `components.schemas` includes `description` and `example`, and every `requestBody` uses `$ref`.

4. **Pillar 4 — Strict HTTP Methods & Status Code Predictability**:
   - *Current State*: `GET` returns `200`/`404`, `POST /bookings` and `POST /events` return `201 Created`, state conflicts return `409 Conflict`, auth failures return `401`/`403`.
   - *Gaps*:
     1. `DELETE /waitlist/:id` (`backend/booking-service/waitlist.go:661`), `DELETE /bookings/:id` (`backend/booking-service/main.go:1084,1255`), and `DELETE /webhooks/:id` (`backend/booking-service/webhook.go:406`) currently return `200 OK` with a JSON body instead of `204 No Content` with an empty body (`POST /bookings/:id/cancel` can remain `200 OK` for backward compatibility with existing E2E scripts, while `DELETE /bookings/:id` and `DELETE /waitlist/:id` and `DELETE /webhooks/:id` return `204 No Content` with empty body!).
     2. `PUT /PATCH` support: Add `PATCH` method support alongside `PUT` on `/events/:id/status` (`catalog-service`) and `/inventory/reserve` or `/inventory/seats/:id` (`inventory-service`) returning `200 OK` with the updated resource, and enforce `405 Method Not Allowed` on invalid HTTP methods in `inventory-service` (`/inventory/reserve`, `/inventory/commit`, `/inventory/release`, `/inventory/cancel`).

5. **Pillar 5 — Runtime Schema Validation & The 3 Contract Violation Tests**:
   - *Current State*: Missing required fields (Violation C) and wrong types (Violation B) return `400 Bad Request` via Go `json.Decoder` + field validation, but `DisallowUnknownFields()` is NOT enabled on `json.NewDecoder(r.Body)`! Thus Violation A (sending an unknown/wrong field identifier like `"user"` instead of `"userId"` or `"price"` instead of `"amount"`) is silently ignored instead of rejected with `400 Bad Request`.
   - *Remediation*:
     1. Create a strict JSON body decoder helper (`decodeStrictJSON(r.Body, &req)`) using `dec := json.NewDecoder(r.Body); dec.DisallowUnknownFields()` across `catalog-service`, `inventory-service`, `booking-service`, `auth-service`, and `notification-service` so unknown/wrong field identifiers immediately return `400 Bad Request`.
     2. Write automated integration tests in `backend/catalog-service/contract_validation_test.go`, `backend/inventory-service/contract_validation_test.go`, `backend/booking-service/contract_validation_test.go`, and `backend/auth-service/contract_validation_test.go` that explicitly test POST/PUT/PATCH endpoints against all 3 contract violations:
        - **Violation A (Wrong field identifier)**: e.g., sending `"user"` instead of `"userId"` or `"price"` instead of `"amount"` -> asserts `400 Bad Request`.
        - **Violation B (Wrong field type)**: e.g., sending string `"20"` where number `20` is required -> asserts `400 Bad Request`.
        - **Violation C (Missing required field)**: omitting a mandatory property -> asserts `400 Bad Request`.

6. **Pillar 6 — Mash-Up Integration & Runtime LLM Composition ("LLMs as APIs")**:
   - *Current State*: Already wired in `backend/booking-service/concierge.go` (`POST /api/concierge/advise` combines Catalog event metadata + Open-Meteo weather API + Frankfurter FX API + runtime Gemini/OpenAI HTTP LLM call with 3.5s circuit breaker and structured JSON validation) and tested in `backend/booking-service/concierge_test.go`. Ensure strict JSON validation (Violation A/B/C) also covers `POST /api/concierge/advise`.

7. **Pillar 7 — Independent CI/CD Pipelines per Microservice**:
   - *Current State*: `.github/workflows/ci.yml` uses `dorny/paths-filter@v3` with per-service jobs, plus we should make sure each microservice job runs its linter (`go vet ./...`), unit + contract violation tests (`go test -timeout 30s ./...`), OpenAPI contract validation (`python3 scripts/validate_openapi.py`), and Docker build independently.

---

## Review Focus (Top 5 Failure Modes)
1. **`DisallowUnknownFields()` vs Existing Internal/Frontend Payloads**: Before enabling `DisallowUnknownFields()`, ensure every struct (`BookingRequest`, `CreateEventRequest`, `ReserveRequest`, `CommitRequest`, `ReleaseRequest`, `StatusUpdateRequest`, `ConciergeAdviceRequest`, `RegisterRequest`, `LoginRequest`, `WaitlistRequest`, `ConvertCurrencyRequest`, `ReviewRequest`) includes all legitimate optional fields sent by the frontend or `scripts/verify_e2e.py` so legitimate requests never fail with 400.
2. **`DELETE` Returning `204 No Content`**: Update `backend/booking-service/waitlist_test.go` (line 277) to expect `204 No Content` (`http.StatusNoContent`) on `DELETE /waitlist/:id`, while keeping `POST /bookings/:id/cancel` returning `200 OK` (and `DELETE /bookings/:id` returning `204 No Content`) so `scripts/verify_e2e.py` and `frontend/src/services/api.ts` pass without regression.
3. **OpenAPI Validator Strictness**: `scripts/validate_openapi.py` must verify `description` and `example` on all schema properties and exit `0` across all 8 OpenAPI files in the repo.
4. **Nginx Gateway Config Syntax**: Every location block in `backend/api-gateway/nginx.conf` must use valid Nginx directives (`auth_request /_auth_verify;`, `limit_req`) without duplicate headers or invalid context.
5. **All 5 Backend Test Suites + Frontend Tests Green**: Every `go test -timeout 30s ./...` across all 5 Go microservices and `python3 scripts/validate_openapi.py` must pass 100%.
