# Milestone 23: User Bookings History, Digital Passbook & Gate QR Ticket Verification Implementation Plan

## 1. Overview & Objectives
Currently, users can create bookings and view the initial confirmation dialog, but there is no persistent "My Bookings" portal to retrieve tickets, no scannable QR digital passbook for gate admission, and no UI-driven mechanism to cancel an active booking via the Saga orchestrator.

Milestone 23 delivers:
1. **Gate Ticket Verification API (`backend/booking-service`)**:
   - `POST /api/bookings/verify-ticket`: Validates ticket code, event match, booking status, and supports an atomic `CHECK_IN` action to mark tickets as `USED`.
2. **Frontend User Bookings & Passbook (`frontend/src`)**:
   - `MyBookingsModal`: Interactive booking history drawer with status tabs (All, Confirmed, Cancelled), guest email lookup, and 1-click Saga cancellation.
   - `TicketPassModal`: Cinematic perforated digital ticket pass with high-contrast typography, SVG QR code, entry barcode, and gate check-in scanner simulator.
   - `Header`: Accessible "My Tickets" button with badge indicator.
3. **Automated Verification**:
   - Go backend tests for ticket verification and check-in lifecycle.
   - Vitest frontend tests for `MyBookingsModal` and `TicketPassModal`.

---

## 2. Review Focus (Top 5 Edge Cases & Failure Modes)
1. **Guest vs Authenticated Query**: Guest users can look up bookings via email input; signed-in users automatically query by JWT `userId` or `customerEmail`.
2. **Double Check-In Protection**: Redeeming an already used ticket (`status = 'USED'`) returns HTTP 409 conflict and prevents fraudulent re-entry.
3. **Cancelled Booking Rejection**: A ticket belonging to a cancelled booking returns HTTP 400 and cannot be verified or checked in.
4. **Saga Cancellation Synchronization**: Cancelling a reservation updates the local modal state to `CANCELLED` and triggers inventory release via the backend Saga.
5. **Deterministic QR SVG & Mobile Layout**: The SVG QR matrix must render sharply across retina displays, print stylesheets, and compact mobile viewports without external bundle bloat.

---

## 3. Implementation Steps

### Phase 2: Autonomous Implementation (TDD Loop)

#### Step 1 (RED): Backend Go Test for Ticket Verification
- File: `backend/booking-service/ticket_verify_test.go`
- Test cases:
  - Unknown ticket code returns 404.
  - Ticket with cancelled booking returns 400.
  - Valid ticket verification returns 200 with ticket and booking metadata.
  - Ticket check-in (`action: "CHECK_IN"`) updates status to `USED`.
  - Second check-in attempt returns 409 conflict.

#### Step 2 (GREEN): Backend Implementation in `backend/booking-service/main.go`
- Add `VerifyTicketRequest` and `VerifyTicketResponse` structs.
- Implement `func (s *Server) handleVerifyTicket(w http.ResponseWriter, r *http.Request)`.
- Register routes `/bookings/verify-ticket` and `/api/bookings/verify-ticket`.
- Run `cd backend/booking-service && go test ./...` and format with `gofmt -s -w`.

#### Step 3 (RED): Frontend Unit Tests for MyBookingsModal & TicketPassModal
- Files:
  - `frontend/src/components/__tests__/MyBookingsModal.test.tsx`
  - `frontend/src/components/__tests__/TicketPassModal.test.tsx`
- Assert booking list rendering, status badge, cancellation button, and QR pass rendering.

#### Step 4 (GREEN): Frontend Implementation
- Update `frontend/src/types/index.ts` with ticket verification and booking response types.
- Update `frontend/src/services/api.ts` with `fetchUserBookings`, `cancelBooking`, and `verifyTicket`.
- Add icons in `frontend/src/components/Icons.tsx` (`QrCodeIcon`, `PassbookIcon`, etc.).
- Create `frontend/src/components/QRCodeDisplay.tsx`.
- Create `frontend/src/components/TicketPassModal.tsx`.
- Create `frontend/src/components/MyBookingsModal.tsx`.
- Update `frontend/src/components/Header.tsx` to add "My Tickets" button.
- Wire state in `frontend/src/App.tsx`.
- Run `cd frontend && npm test` and `npm run lint`.

#### Step 5: Verification, Mutation QA & PR Creation
- Check diffs with `git diff <base-sha>..HEAD --stat`.
- Update `docs/ARCHITECTURE.md` with Milestone 23 details.
- Commit, push branch, and create GitHub Pull Request via `gh pr create`.
