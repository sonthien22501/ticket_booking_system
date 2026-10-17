# Milestone 15: Service Health Checks & Metrics Endpoints (R23) Implementation Plan

## Goal
Implement comprehensive actuator endpoints (`/health` and `/metrics`) across all microservices (`auth-service`, `catalog-service`, `inventory-service`, `booking-service`) and the API Gateway according to **Requirement R23**. The endpoints must provide real-time monitoring of connection pool saturation (PostgreSQL `sql.DBStats`, Redis `PoolStats`), memory usage (`runtime.MemStats`, goroutines), component readiness/liveness status, and Prometheus-compatible exposition.

---

## Review Focus (Top 5 Edge Cases & Failure Modes)
1. **Connection Pool Saturation Reporting**: `db.Stats()` and Redis `PoolStats()` accurately reflect open, in-use, idle, wait count, and connection limits.
2. **Dependency Outage & Graceful Health Degradation**: When a backend dependency (PostgreSQL or Redis) is disconnected or unhealthy, `/health` returns `DOWN` with HTTP 503 Service Unavailable instead of returning a false positive HTTP 200.
3. **Format Negotiation (JSON & Prometheus)**: `/metrics` returns structured JSON by default (`application/json`) and Prometheus exposition format (`text/plain; version=0.0.4`) when `Accept: text/plain` or `format=prometheus` is specified.
4. **API Gateway Ingress & Service Routing**: All routes (`/health`, `/metrics`, `/api/auth/{health,metrics}`, `/api/catalog/{health,metrics}`, `/api/inventory/{health,metrics}`, `/api/bookings/{health,metrics}`) route correctly through port 8080 with CORS and tracing headers.
5. **Thread-Safe Metrics Under Concurrency**: Concurrent incoming traffic increments request counters and latency stats safely via atomic operations without race conditions.

---

## Implementation Breakdown

### Task 1: TDD Failing Verification Suite (RED)
- Create `e2e/health_metrics_test.go`:
  - `TestMicroserviceHealthEndpoints`: Verifies `/health` on all services reports `UP`, returns timestamp, uptime, and component statuses (PostgreSQL, Redis, RabbitMQ).
  - `TestMicroserviceMetricsEndpoints`: Verifies `/metrics` returns memory stats (`alloc_bytes`, `heap_inuse`, `num_goroutines`), Go runtime stats, and connection pool stats (`open_connections`, `in_use`, `idle`).
  - `TestPrometheusMetricsFormat`: Verifies `/metrics` with `Accept: text/plain` returns Prometheus format with HELP and TYPE lines.
  - `TestGatewayHealthAndMetricsRouting`: Verifies API Gateway routes `/health`, `/metrics`, and `/api/{service}/{health,metrics}` on port 8080.
  - `TestHealthDegradationOnFailure`: Verifies simulated or unreachable dependency causes `/health` to return HTTP 503 and `status: DOWN`.
- Run tests and witness failure (RED).

### Task 2: Implement Actuator Subsystem in Microservices
- Create `backend/common/actuator` or per-service `actuator.go`:
  - `backend/auth-service/actuator.go`
  - `backend/catalog-service/actuator.go`
  - `backend/inventory-service/actuator.go`
  - `backend/booking-service/actuator.go`
- Provide:
  - `RegisterActuatorEndpoints(mux *http.ServeMux, config ActuatorConfig)`
  - Health check handler: checks DB ping, Redis ping, RabbitMQ channel; returns 200 (UP) or 503 (DOWN).
  - Metrics handler: extracts `sql.DBStats`, `rdb.PoolStats()`, `runtime.ReadMemStats`, `runtime.NumGoroutine()`, request counters; formats as JSON or Prometheus plain text.
- Wire into `main.go` of each microservice.

### Task 3: API Gateway Routing & Aggregated Health/Metrics
- Update `backend/api-gateway/nginx.conf`:
  - Expose `/metrics` (Gateway and upstream metrics).
  - Expose `/api/auth/metrics`, `/api/catalog/metrics`, `/api/inventory/metrics`, `/api/bookings/metrics`.
  - Ensure `/api/auth/health`, `/api/catalog/health`, `/api/inventory/health`, `/api/bookings/health` proxy correctly.
- Update `backend/api-gateway/swagger/openapi.yaml` and `openapi.json`:
  - Document `/health` component breakdown.
  - Document `/metrics` endpoint across services.

### Task 4: Documentation & Milestone Tracking
- Update `REQUIREMENTS.md`: Mark `R23` as `✅ (Done successfully)`.
- Update `explain.md`: Add detailed Milestone 15 technical guide.

### Task 5: QA Anti-Cheat Audit & Mutation Testing
- Mutation testing: Temporarily alter a metric field or health status, verify E2E catches it (RED), revert immediately.
- Verify 100% PASS on `e2e/health_metrics_test.go` and existing test suites.
- Commit, push feature branch `feature/milestone-15-health-metrics`, and open PR via GitHub API.
