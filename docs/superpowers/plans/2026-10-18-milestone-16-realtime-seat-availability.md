# Milestone 16: Real-time Seat Availability Retrieval (R10) Implementation Plan

## Goal
Implement real-time seat availability retrieval across the Seat Inventory Service and API Gateway per **Requirement R10**. Provide both an SSE stream (`/api/inventory/{eventId}/seats/stream`) for event-driven push notifications of seat state changes (`AVAILABLE`, `HOLD`, `BOOKED`) and zero-cache freshness endpoints (`/api/inventory/{eventId}/seats/realtime`) with status filtering (`?status=AVAILABLE`).

---

## Technical Design & Failure Modes
1. **SSE Protocol Integrity**: Set `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, and `X-Accel-Buffering: no` to disable Nginx buffer stalls.
2. **Pub/Sub Broadcast Hub**: Thread-safe channel-based broadcaster in `inventory-service` dispatching mutations (`reserve`, `commit`, `release`, `cancel`, `janitor sweep`) to all active stream clients.
3. **Graceful Disconnection**: Listen on `r.Context().Done()` to unregister subscriber channels and avoid goroutine/memory leaks.
4. **Gateway Ingress**: Configure Nginx reverse proxy to stream chunked SSE frames without timeout disconnections or buffering.
5. **OpenAPI & Contract Compliance**: Formally document stream and freshness routes in Swagger OpenAPI 3.0 specification.

---

## Tasks Breakdown
1. **Task 1: Failing E2E Verification Suite (RED)**:
   - Create `e2e/seat_availability_test.go` asserting SSE stream initial state, real-time update broadcast upon reservation/release, status filtering, and freshness headers.
2. **Task 2: Inventory Broadcaster Hub & SSE Handler**:
   - Create `backend/inventory-service/broadcaster.go`.
   - Update `backend/inventory-service/main.go` to hook mutations into broadcaster and serve `/inventory/{eventId}/seats/stream` and `/inventory/{eventId}/seats/realtime`.
3. **Task 3: API Gateway & OpenAPI Contract**:
   - Update `backend/api-gateway/nginx.conf` for SSE streaming without buffering.
   - Update `openapi.yaml` and `openapi.json` with new endpoints and components.
4. **Task 4: Build, Deploy & Verify (GREEN)**:
   - Rebuild `inventory-service` and `api-gateway` in docker-compose.
   - Run E2E test suite and verify 100% pass.
5. **Task 5: Requirements Sign-Off & PR Push**:
   - Mark R10 as completed in `REQUIREMENTS.md`.
   - Document architecture in `explain.md`.
   - Push branch `feature/milestone-16-realtime-seat-availability` and open GitHub Pull Request.
