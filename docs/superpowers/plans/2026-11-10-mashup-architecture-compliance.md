# Architectural Compliance Plan: Database-per-Service, Gateway Auth/Rate-Limit, Path-Filtered CI/CD + OpenAPI Validation, and Runtime LLM API

**Baseline Commit (`BASE_SHA`)**: `e156c13`
**Target Branch**: `feature/mashup-architecture-compliance`

## Overview
Address all 4 architectural pillars from Chapter 1 (Part 2) & Chapter 2 (Part 1) of the Mash-Up Engineering course while preserving 100% backward compatibility with existing E2E tests (`scripts/verify_e2e.py`) and service tests:

1. **Database-per-Service Isolation (Anti-Distributed Monolith)**:
   - Configure PostgreSQL initialization (`infra/postgres/init.sql`) to create isolated PostgreSQL schemas (`catalog_schema`, `inventory_schema`, `booking_schema`, `auth_schema`, `notification_schema`) and support `DB_SCHEMA` / `search_path` isolation in `docker-compose.yml` and each Go service's DSN connection string (`?sslmode=disable&search_path=<schema>,public`).
   - Eliminate cross-context direct table mutations where services bypass REST APIs:
     - `catalog-service`: When creating an event (`POST /events`), replace direct `INSERT INTO event_inventory` and `INSERT INTO seat_items` with an HTTP `POST` to `INVENTORY_SERVICE_URL/inventory/provision` (with fallback so tests without a live inventory service still pass).
     - `inventory-service`: Expose `POST /inventory/provision` so `inventory-service` exclusively owns `event_inventory` and `seat_items` creation.
     - `booking-service`: Add `fetchCatalogEventMetadata(eventId)` helper calling `CATALOG_SERVICE_URL/events/:id` for event/tier lookups before falling back to local cache/read-replica views.

2. **API Gateway Central Rate Limiting & Authentication (`auth_request`)**:
   - In `backend/auth-service/main.go`, expose a fast `GET /auth/verify` (and `/api/auth/verify`) endpoint for Nginx `auth_request`:
     - If `Authorization: Bearer <token>` is present and invalid/expired -> returns `401 Unauthorized`.
     - If valid -> returns `200 OK` with response headers `X-Auth-User-Id`, `X-Auth-User-Email`, `X-Auth-User-Role`.
     - For admin-only verification (`GET /auth/verify-admin`), returns `401` if missing token or `403` if role != `admin`.
   - In `backend/api-gateway/nginx.conf`, add internal location `= /_auth_verify` (`proxy_pass http://auth_service/auth/verify`) and apply `auth_request /_auth_verify` + `auth_request_set` to propagate verified identity headers (`X-Verified-User-Id`, `X-Verified-User-Role`) to downstream services, plus explicit gateway-level JWT bearer guard on `/api/admin/*`.

3. **Independent Path-Filtered CI/CD & Per-Service OpenAPI Schema Validation**:
   - Expose `GET /openapi.json` and `GET /api/openapi.json` on all 5 microservices (`catalog-service`, `inventory-service`, `booking-service`, `auth-service`, `notification-service`), and route `/api/<service>/openapi.json` in `backend/api-gateway/nginx.conf`.
   - Create `scripts/validate_openapi.py` to validate `docs/openapi.yaml`, `backend/api-gateway/swagger/openapi.yaml`, and all 5 service OpenAPI specifications for valid OpenAPI 3.0.3 structure, required paths, schemas, and operations.
   - Refactor `.github/workflows/ci.yml` to include:
     - `changes` path-filtering detection (`dorny/paths-filter` or git diff matrix)
     - Independent unit-test & build jobs per microservice (`catalog-service`, `inventory-service`, `booking-service`, `auth-service`, `notification-service`, `frontend`)
     - Dedicated `openapi-schema-validation` job running `python3 scripts/validate_openapi.py`
     - `e2e-integration` job.

4. **"LLMs as APIs Themselves" (Runtime Non-Deterministic Logic)**:
   - Upgrade `POST /api/concierge/advise` in `backend/booking-service/concierge.go`:
     - At runtime, fetch live event metadata from `CATALOG_SERVICE_URL/events/:id` and live weather from Open-Meteo (`https://api.open-meteo.com/v1/forecast` or `CATALOG_SERVICE_URL/events/:id/mashup`) when not already supplied by the caller.
     - Perform a real runtime HTTP call to an LLM API (`https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent` when `GEMINI_API_KEY` is set, or `https://api.openai.com/v1/chat/completions` when `OPENAI_API_KEY` is set, with configurable `LLM_API_URL` for testing) requesting structured JSON output (`response_mime_type: "application/json"`).
     - Enforce a 3.5-second circuit-breaker timeout: if the API key is absent, the network times out, or the LLM returns malformed JSON, gracefully degrade to the deterministic synthesizer with `fallbackUsed: true` (when LLM call failed) or `engine: "runtime-heuristic-fallback"`.

---

## Review Focus (Top 5 Edge Cases / Failure Modes)
1. **PostgreSQL `search_path` Backward Compatibility**: Creating isolated schemas (`catalog_schema`, `inventory_schema`, `booking_schema`, `auth_schema`, `notification_schema`) must not break `scripts/verify_e2e.py` or existing seed data. Use `search_path=<service_schema>,public` and create schema-scoped views/tables in `infra/postgres/init.sql` so each service operates in its own isolated schema namespace while retaining zero-downtime compatibility.
2. **Nginx `auth_request` on Public vs Protected Endpoints**: Public endpoints (`GET /api/events`, `POST /api/bookings` guest checkout, `GET /api/bookings?email=...`) allow optional auth, whereas `/api/admin/*` requires valid auth unless internal test bypass headers are used. Wait — check `scripts/verify_e2e.py` to see how `/api/admin/*` is called in E2E tests! Make sure `verify_e2e.py` continues to pass 100%.
3. **LLM Runtime Call Circuit Breaker & JSON Parsing**: When calling Gemini/OpenAI HTTP APIs at runtime, handle markdown code fences (` ```json ... ``` `) in LLM responses and fall back cleanly if HTTP status != 200 or timeout expires.
4. **OpenAPI 3.0.3 Schema Validator**: `scripts/validate_openapi.py` must run cleanly with standard Python 3 (`json`, `yaml` / PyYAML) in CI and locally with 0 errors.
5. **Cross-Service Provisioning in `catalog-service`**: When `POST /api/events` creates an event, calling `inventory-service` (`POST /inventory/provision`) must succeed in Docker and gracefully fall back in standalone unit tests where `inventory-service` is mocked or offline.

---

## Step-by-Step TDD Tasks

### Task 1: Database-per-Service Schema Isolation & REST Provisioning Boundary
1. **Test First (RED)**:
   - In `backend/inventory-service/main_test.go`, add a test `TestHandleProvisionInventory` verifying `POST /inventory/provision` initializes `event_inventory` and `seat_items` for a newly created event.
   - In `backend/catalog-service/main_test.go`, add a test `TestBuildPostgresDSNWithSchema` verifying `DB_SCHEMA` appends `search_path=<schema>,public` to the Postgres DSN.
2. **Implementation (GREEN)**:
   - Update `infra/postgres/init.sql` to create isolated bounded-context schemas:
     `CREATE SCHEMA IF NOT EXISTS catalog_schema;`, `CREATE SCHEMA IF NOT EXISTS inventory_schema;`, `CREATE SCHEMA IF NOT EXISTS booking_schema;`, `CREATE SCHEMA IF NOT EXISTS auth_schema;`, `CREATE SCHEMA IF NOT EXISTS notification_schema;` and expose domain-scoped views/tables in each schema.
   - Update `docker-compose.yml` to pass isolated `DB_SCHEMA` environment variables to each service (`catalog_schema`, `inventory_schema`, `booking_schema`, `auth_schema`, `notification_schema`) and `INVENTORY_SERVICE_URL: http://inventory-service:8082` to `catalog-service`.
   - Add `POST /inventory/provision` handler in `backend/inventory-service/main.go` and call it via HTTP from `backend/catalog-service/main.go` when provisioning a new event.

### Task 2: API Gateway Central Authentication (`auth_request`) & Rate Limiting
1. **Test First (RED)**:
   - In `backend/auth-service/main_test.go`, write `TestHandleAuthVerify` testing `GET /auth/verify` with:
     - No token -> `200 OK` with `X-Auth-Status: anonymous`
     - Valid JWT Bearer token -> `200 OK` with `X-Auth-Status: authenticated`, `X-Auth-User-Id`, `X-Auth-User-Role`
     - Malformed/Invalid JWT Bearer token -> `401 Unauthorized`
2. **Implementation (GREEN)**:
   - Implement `handleAuthVerify` on `GET /auth/verify` and `GET /api/auth/verify` in `backend/auth-service/main.go`.
   - Update `backend/api-gateway/nginx.conf` to define `location = /_auth_verify` (`internal; proxy_pass http://auth_service/auth/verify; proxy_pass_request_body off; proxy_set_header Content-Length "";`) and wire `auth_request /_auth_verify;` into API locations so invalid JWT tokens are rejected at the API Gateway perimeter before hitting downstream services, and verified identity headers are forwarded downstream.

### Task 3: Per-Service OpenAPI 3.0 Specs, Validator Script & Path-Filtered CI/CD
1. **Test First (RED)**:
   - Write unit tests in `inventory-service`, `booking-service`, `auth-service`, and `notification-service` asserting `GET /openapi.json` returns `200 OK` with `"openapi": "3.0.3"`.
   - Create `scripts/validate_openapi.py` and run it against all service specs and `docs/openapi.yaml`.
2. **Implementation (GREEN)**:
   - Add `GET /openapi.json` and `GET /api/openapi.json` handlers to `inventory-service`, `booking-service`, `auth-service`, and `notification-service`.
   - Add gateway routes `/api/catalog/openapi.json`, `/api/inventory/openapi.json`, `/api/bookings/openapi.json`, `/api/auth/openapi.json`, `/api/notifications/openapi.json` in `backend/api-gateway/nginx.conf`.
   - Update `.github/workflows/ci.yml` to implement path filtering (`dorny/paths-filter@v3`) and independent per-microservice unit test & build jobs (`catalog-service`, `inventory-service`, `booking-service`, `auth-service`, `notification-service`, `frontend`), plus `openapi-validation` (`python3 scripts/validate_openapi.py`) and `e2e-integration`.

### Task 4: Runtime Non-Deterministic LLM API Integration (`POST /api/concierge/advise`)
1. **Test First (RED)**:
   - In `backend/booking-service/concierge_test.go`, write tests:
     - `TestConciergeRuntimeLLMCallSuccess`: spins up an `httptest.Server` mocking the LLM API (`LLM_API_URL`) and Catalog/Open-Meteo API, sets `GEMINI_API_KEY=test-key`, calls `POST /api/concierge/advise`, and verifies the non-deterministic LLM JSON response is parsed and returned with `fallbackUsed: false` and `engine: "gemini-2.5-flash-runtime"`.
     - `TestConciergeRuntimeLLMFallbackOnTimeoutOrError`: verifies that if the LLM server returns 500 or times out, `handleConciergeAdvise` gracefully falls back to deterministic synthesis with `fallbackUsed: true`.
2. **Implementation (GREEN)**:
   - In `backend/booking-service/concierge.go`:
     - Enrich missing event/weather context at runtime by querying `CATALOG_SERVICE_URL/events/<id>/mashup` (2s timeout) so live Open-Meteo weather and Catalog tier data are fused at runtime.
     - Implement `callRuntimeLLMAPI(ctx, req)` supporting both Gemini (`GEMINI_API_KEY`) and OpenAI (`OPENAI_API_KEY`) or custom `LLM_API_URL` endpoints, sending a structured system/user prompt combining the Catalog event, Open-Meteo weather forecast, FX rate, and user preferences, and unmarshaling the LLM JSON output into `ConciergeAdviceResponse`.
     - Preserve deterministic fallback when no API key is configured or if the runtime LLM call errors/times out.
