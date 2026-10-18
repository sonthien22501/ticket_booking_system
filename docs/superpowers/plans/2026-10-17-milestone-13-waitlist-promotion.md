# Milestone 13: Automatic Waitlist Promotion (R20) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement automated FIFO waitlist promotion (R20) so that when a confirmed booking is cancelled (R21) or an unconfirmed reservation/hold expires (R19), the released seat is immediately reserved for the next eligible user in the waitlist queue.

**Architecture:** A new `waitlist` subsystem in `booking-service` backed by PostgreSQL and Redis provides FIFO waitlist registration, position inspection, and claim endpoints. When `inventory.cancelled`, `inventory.reservation_expired`, or `inventory.released` events arrive at the Saga Orchestrator, a distributed lock is acquired to atomically select the next FIFO `PENDING` waitlist entry, place a hold reservation on the newly available seat, transition the entry to `PROMOTED`, and publish `waitlist.promoted` domain events.

**Tech Stack:** Go 1.22+, PostgreSQL 16, Redis 7 (distributed locking), RabbitMQ 3 (AMQP 0-9-1 Topic Exchange `saga_exchange`), NGINX API Gateway.

**Spec:** `REQUIREMENTS.md` (Requirement R20: Automatic waitlist promotion functionality).

---

## Global Constraints
- Target Requirement: R20 ("When a user cancels a confirmed ticket or abandons a checkout, the system automatically reserves the seat for the next user in a FIFO waitlist queue.")
- Microservice Boundaries: `booking-service` manages waitlists, bookings, tickets, and orchestrates promotion; `inventory-service` manages seat locks and releases.
- Message Broker: RabbitMQ topic exchange `saga_exchange` with dead-letter exchange `saga_dead_letter_exchange`.
- Traceability: All API requests and asynchronous messages must preserve `X-Correlation-ID` and structured logging format (`StructuredLog`).
- No Overbooking: Concurrency safety via Redis distributed lock `waitlist:lock:<eventId>` and Postgres `FOR UPDATE SKIP LOCKED`.

---

## Review Focus
1. **FIFO Ordering Under Concurrency**: When multiple users join the waitlist at different times, the user with the earliest `created_at` timestamp must be promoted first upon seat release.
2. **Double Promotion Prevention**: When multiple cancellations or timeout sweeps occur concurrently for the same event, exactly one waitlist entry must be promoted per released seat without over-promoting.
3. **Hold Lease Expiration for Promoted Users**: When a waitlist entry is promoted, it receives a timed reservation hold. If the user does not claim the ticket before `expires_at`, the janitor expires it and allows promotion to the next in line.
4. **Idempotent Claim & Booking Conversion**: Claiming a promoted reservation via `POST /waitlist/{id}/claim` confirms the booking and marks the waitlist entry as `CLAIMED` without allowing double claims.
5. **Tier & Seat Matching**: If a waitlist entry specifies a specific `tierId` or `seatId`, promotion must only match seats belonging to that tier or seat.

---

### Task 1: Database Schema & Waitlist Repository in Booking Service

**Files:**
- Modify: `infra/postgres/init.sql`
- Create: `backend/booking-service/waitlist.go`
- Create: `backend/booking-service/waitlist_test.go`

**Interfaces:**
- Produces:
  ```go
  type WaitlistStatus string
  const (
      WaitlistStatusPending   WaitlistStatus = "PENDING"
      WaitlistStatusPromoted  WaitlistStatus = "PROMOTED"
      WaitlistStatusClaimed   WaitlistStatus = "CLAIMED"
      WaitlistStatusExpired   WaitlistStatus = "EXPIRED"
      WaitlistStatusCancelled WaitlistStatus = "CANCELLED"
  )

  type WaitlistEntry struct {
      ID            string         `json:"id"`
      EventID       string         `json:"eventId"`
      TierID        string         `json:"tierId,omitempty"`
      SeatID        string         `json:"seatId,omitempty"`
      UserID        string         `json:"userId"`
      CustomerName  string         `json:"customerName"`
      CustomerEmail string         `json:"customerEmail"`
      Quantity      int            `json:"quantity"`
      Status        WaitlistStatus `json:"status"`
      ReservationID string         `json:"reservationId,omitempty"`
      PromotedAt    *time.Time     `json:"promotedAt,omitempty"`
      ExpiresAt     *time.Time     `json:"expiresAt,omitempty"`
      Position      int            `json:"position,omitempty"`
      CreatedAt     time.Time      `json:"createdAt"`
      UpdatedAt     time.Time      `json:"updatedAt"`
  }

  func (s *Server) initWaitlistSchema(ctx context.Context) error
  func (s *Server) CreateWaitlistEntry(ctx context.Context, entry *WaitlistEntry) (*WaitlistEntry, error)
  func (s *Server) GetWaitlistEntry(ctx context.Context, id string) (*WaitlistEntry, error)
  func (s *Server) GetWaitlistPosition(ctx context.Context, entry *WaitlistEntry) (int, error)
  func (s *Server) FindNextWaitlistEntryForPromotion(ctx context.Context, eventID string, tierID string) (*WaitlistEntry, error)
  func (s *Server) PromoteWaitlistEntry(ctx context.Context, waitlistID string, reservationID string, expiresAt time.Time) error
  func (s *Server) CancelWaitlistEntry(ctx context.Context, waitlistID string, userID string) error
  func (s *Server) ListWaitlistEntries(ctx context.Context, eventID, userID, status string) ([]*WaitlistEntry, error)
  ```

- [x] **Step 1: Write the failing unit test**
Create `backend/booking-service/waitlist_test.go` testing waitlist entry creation, FIFO position calculation, and status progression (`PENDING` -> `PROMOTED` -> `CLAIMED`).

- [x] **Step 2: Run test to verify it fails**
Run: `cd backend/booking-service && go test -v -run TestWaitlist ./...`
Expected: FAIL (compilation error: undefined functions/types).

- [x] **Step 3: Implement `init.sql` schema and `backend/booking-service/waitlist.go`**
Add `CREATE TABLE IF NOT EXISTS waitlist (...)` in `infra/postgres/init.sql` and `initWaitlistSchema` in `waitlist.go`. Implement the repository functions with SQL queries and row scanning.

- [x] **Step 4: Run test to verify it passes**
Run: `cd backend/booking-service && go test -v -run TestWaitlist ./...`
Expected: PASS.

- [x] **Step 5: Commit**
```bash
git add infra/postgres/init.sql backend/booking-service/waitlist.go backend/booking-service/waitlist_test.go
git commit -m "feat(waitlist): add waitlist schema and repository operations"
```

---

### Task 2: Waitlist REST API & API Gateway Routing

**Files:**
- Modify: `backend/booking-service/waitlist.go`
- Modify: `backend/booking-service/main.go`
- Modify: `backend/api-gateway/nginx.conf`

**Interfaces:**
- Consumes: `WaitlistEntry` repository methods from Task 1.
- Produces HTTP Endpoints:
  - `POST /waitlist` / `POST /api/waitlist`: Join waitlist (returns 201 Created with queue position).
  - `GET /waitlist` / `GET /api/waitlist`: List waitlist entries filtered by `eventId`, `userId`, `status`.
  - `GET /waitlist/{id}` / `GET /api/waitlist/{id}`: Get entry by ID with live queue position.
  - `DELETE /waitlist/{id}` / `DELETE /api/waitlist/{id}` or `POST /waitlist/{id}/cancel`: Cancel waitlist entry (returns 200 OK).
  - `POST /waitlist/{id}/claim` / `POST /api/waitlist/{id}/claim`: Claim promoted seat and confirm booking.

- [x] **Step 1: Write the failing test for Waitlist HTTP Handlers**
Add tests in `backend/booking-service/waitlist_test.go` testing `handleWaitlist` POST, GET, DELETE, and claim flows with mock HTTP requests.

- [x] **Step 2: Run test to verify it fails**
Run: `cd backend/booking-service && go test -v -run TestWaitlistHTTP ./...`
Expected: FAIL.

- [x] **Step 3: Implement HTTP Handlers and Register Routes in `main.go` and `nginx.conf`**
Implement `handleWaitlist`, `handleWaitlistByID`, `handleWaitlistClaim` in `waitlist.go`.
Register routes in `booking-service/main.go` under `mux`.
Add `/api/waitlist` and `/waitlist` location blocks in `backend/api-gateway/nginx.conf` with `booking_rate_limit`.

- [x] **Step 4: Run test to verify it passes**
Run: `cd backend/booking-service && go test -v -run TestWaitlistHTTP ./...`
Expected: PASS.

- [x] **Step 5: Commit**
```bash
git add backend/booking-service/waitlist.go backend/booking-service/main.go backend/api-gateway/nginx.conf backend/booking-service/waitlist_test.go
git commit -m "feat(waitlist): implement waitlist HTTP endpoints and API gateway routing"
```

---

### Task 3: Automatic Waitlist Promotion Engine on Seat Releases

**Files:**
- Modify: `backend/booking-service/saga_orchestrator.go`
- Modify: `backend/inventory-service/main.go`
- Modify: `backend/inventory-service/saga_worker.go`

**Interfaces:**
- Consumes: RabbitMQ messages on `saga_exchange`:
  - `inventory.cancelled`: When a confirmed booking is cancelled.
  - `inventory.reservation_expired`: When hold timeout sweeps release seats.
  - `inventory.released`: When checkout abandonment or payment compensation releases seats.
- Produces:
  - Calls `inventory/reserve` to place a hold on the released seat for the next waitlist user.
  - Promotes the waitlist entry (`status = 'PROMOTED'`).
  - Publishes `waitlist.promoted` event to RabbitMQ topic exchange.

- [x] **Step 1: Write failing test for promotion trigger logic**
Add test in `backend/booking-service/waitlist_test.go` (or `saga_test.go`) simulating receipt of `inventory.cancelled` or `inventory.reservation_expired` and verifying promotion is invoked.

- [x] **Step 2: Run test to verify it fails**
Run: `cd backend/booking-service && go test -v -run TestWaitlistPromotionTrigger ./...`
Expected: FAIL.

- [x] **Step 3: Implement Promotion Logic in Saga Orchestrator and Inventory Service**
In `saga_orchestrator.go`:
Implement `TriggerWaitlistPromotion(ctx context.Context, corrID, eventID, tierID string, seatIDs []string)`.
When `inventory.cancelled`, `inventory.reservation_expired`, or `inventory.released` occurs:
1. Acquire Redis lock `waitlist:lock:<eventId>` (or DB `FOR UPDATE SKIP LOCKED`).
2. Query `FindNextWaitlistEntryForPromotion(ctx, eventID, tierID)`.
3. If entry found, reserve seat with hold lease (e.g. 600s) via HTTP to inventory-service `/inventory/reserve`.
4. Update waitlist entry: `PromoteWaitlistEntry(ctx, entry.ID, reservationID, expiresAt)`.
5. Publish `waitlist.promoted` message on `saga_exchange`.
In `inventory-service/main.go`:
Include `seatIds` and `tierId` in `inventory.reservation_expired` message payload.

- [x] **Step 4: Run test to verify it passes**
Run: `cd backend/booking-service && go test -v ./...`
Expected: PASS.

- [x] **Step 5: Commit**
```bash
git add backend/booking-service/saga_orchestrator.go backend/inventory-service/main.go backend/inventory-service/saga_worker.go
git commit -m "feat(waitlist): integrate automatic waitlist promotion into saga orchestrator and inventory worker"
```

---

### Task 4: End-to-End Test Suite & Requirements Sign-off

**Files:**
- Create: `e2e/waitlist_test.go`
- Modify: `REQUIREMENTS.md`

**Interfaces:**
- Tests all E2E workflows against running microservice stack:
  1. `TestWaitlistJoinAndQueuePosition`: Verify queue order 1, 2, 3.
  2. `TestWaitlistAutoPromotionOnCancellation`: Confirmed booking -> User waitlists -> Booking cancelled -> Waitlist user promoted -> Claimed.
  3. `TestWaitlistAutoPromotionOnTimeout`: Hold reservation created -> Waitlist user joins -> Hold expires via Janitor -> Waitlist user promoted.
  4. `TestWaitlistConcurrentFIFOOrdering`: Concurrency contention preserving exact FIFO priority.
  5. `TestWaitlistReviewFocusEdgeCases`: Covers the 5 review focus items.

- [x] **Step 1: Write E2E test suite in `e2e/waitlist_test.go`**
Implement the test scenarios exercising gateway endpoints `/waitlist`, `/bookings`, `/inventory/janitor/sweep`.

- [x] **Step 2: Rebuild & Restart Docker Containers**
Run: `docker-compose up -d --build booking-service inventory-service api-gateway`
Verify containers reach healthy status: `docker ps`

- [x] **Step 3: Run E2E test suite**
Run: `cd e2e && go test -v -run TestWaitlist ./...`
Expected: PASS for all waitlist test cases.

- [x] **Step 4: Update `REQUIREMENTS.md`**
Mark Requirement R20 as completed:
`- [x]` / `**R20: The system should provide automatic waitlist promotion functionality.** ✅ (Done successfully)`.

- [x] **Step 5: Commit**
```bash
git add e2e/waitlist_test.go REQUIREMENTS.md
git commit -m "test(waitlist): add comprehensive e2e waitlist promotion test suite (R20)"
```
