# Milestone 27: Waitlist Self-Service Portal & Secure Digital Ticket Transfer Engine Implementation Plan

## 1. Overview & Objectives
Milestone 27 delivers **Waitlist Self-Service Portal & Secure Digital Ticket Transfer Engine** for the Distributed Event Ticket Booking System. As demonstrated in the master test dataset (`docs/TEST_DATASET_GUIDE.md`), high-demand events (such as `evt-106` Stand-Up Comedy All-Stars) sell out completely, requiring automated queue management. Additionally, attendees who purchase tickets often need to securely gift or transfer individual tickets to friends or colleagues.

### Delivered Capabilities:
1. **Frontend Sold-Out Detection & Waitlist Join Modal (`frontend/src`)**:
   - Dynamic sold-out detection on event catalog cards (`availableSeats === 0` or sold-out tiers).
   - "Join Waitlist" action replacing standard seat selection on sold-out events.
   - Dedicated `WaitlistModal` allowing attendees to specify contact info, target tier, and requested quantity.
   - Real-time allocated queue position confirmation (e.g. "You are #3 in line!").
2. **Interactive Waitlist Tracker & Promoted Hold Claiming (`frontend/src/components/MyBookingsModal.tsx`)**:
   - New "Waitlist" tab in user Passbook tracking active waitlist requests.
   - Displays real-time queue position badges (`#1 in Queue`, `#2 in Queue`).
   - For `PROMOTED` entries (e.g. Maria Garcia with active hold `res-wl-promoted-003`): displays active 15-minute countdown clock and 1-click "Claim & Book" action.
   - "Leave Waitlist" cancellation flow (`DELETE /api/waitlist/:id`).
3. **Secure Digital Ticket Transfer Engine (`backend/booking-service`)**:
   - `POST /api/tickets/transfer` and `POST /api/bookings/:id/tickets/:ticketId/transfer`:
     - Validates ownership, active booking status, and ticket eligibility (`VALID` status only).
     - Transitions original ticket to `TRANSFERRED` status, permanently invalidating its QR ticket code.
     - Generates a new cryptographically unique ticket code (`TKT-XFER-...`) assigned to the recipient.
     - Logs comprehensive audit trail in database `ticket_transfers` table.
     - Associated booking records enable recipients to view transferred tickets in their own passbook.
4. **Gate Admission Anti-Passback & Fraud Verification (`backend/booking-service`)**:
   - Gate scanner (`POST /api/bookings/verify-ticket`) returns `valid: false, status: "TRANSFERRED"` when scanning the original transferred QR code.
   - Gate scanner admits recipient with new QR code (`valid: true, status: "VALID"`).
5. **Frontend Ticket Transfer Modal (`frontend/src/components/TicketTransferModal.tsx`)**:
   - Accessible from both `MyBookingsModal` and `TicketPassModal`.
   - Displays sender ticket details, recipient input form, and security warning about QR invalidation.
6. **Admin Organizer Studio Waitlist Telemetry (`frontend/src/components/AdminDashboardModal.tsx`)**:
   - New Waitlist Management tab displaying all event waitlists, queue positions, and manual promotion triggers.
7. **Automated Verification**:
   - Go unit and integration tests in `backend/booking-service/ticket_transfer_test.go`.
   - Vitest unit tests in `frontend/src/test/waitlist_ticket_transfer.test.ts`.

---

## 2. Review Focus (Top 5 Edge Cases & Failure Modes)
1. **Gate Rejection of Transferred Tickets**: Scanning an invalidated transferred ticket code must immediately return `valid: false, status: "TRANSFERRED"` and reject check-in (preventing double entry or fraudulent re-use).
2. **Cannot Transfer Non-Valid Tickets**: Attempts to transfer tickets with status `USED`, `CANCELLED`, or already `TRANSFERRED` must be rejected with HTTP 409 Conflict.
3. **Recipient Passbook Visibility**: When a ticket is transferred to `recipientEmail`, querying `/api/bookings?email=recipientEmail` must include the transferred ticket in the recipient's passbook.
4. **Zero-Oversell Sold-Out Locking**: Sold-out events (`availableSeats === 0`) must never allow direct seat reservation checkout; users must be routed to the waitlist queue.
5. **Promoted Hold Expiry Handling**: Promoted waitlist entries past their `expiresAt` timestamp must show expired status and prevent checkout claiming.

---

## 3. Implementation Steps (Strict RED-GREEN TDD)

### Step 1 (RED): Backend Tests for Ticket Transfer & Waitlist Enhancement
- Create `backend/booking-service/ticket_transfer_test.go`:
  - Test valid ticket transfer, code rotation, and audit record creation.
  - Test rejection when attempting to transfer `USED` or `CANCELLED` ticket.
  - Test gate scanner rejection of old code and acceptance of new code.
  - Test waitlist queue querying and claim flow.
- Run tests and witness failure (RED).

### Step 2 (GREEN): Backend Implementation
- Create `backend/booking-service/ticket_transfer.go`:
  - `TicketTransferRequest`, `TicketTransferResponse`, `TicketTransferRecord`.
  - Implement `handleTransferTicket(w http.ResponseWriter, r *http.Request)`.
  - Update `handleVerifyTicket` to handle `TRANSFERRED` status with explicit error message.
  - Ensure waitlist claim endpoint `handleWaitlistClaim` creates booking and ticket records smoothly.
- Register routes in `backend/booking-service/main.go`:
  - `POST /api/tickets/transfer`
  - `POST /api/bookings/{id}/tickets/{ticketId}/transfer`
  - Ensure `/api/waitlist` claim and list routes support recipient lookups.
- Update `backend/api-gateway/nginx.conf` if route adjustments are needed.
- Update `infra/postgres/init.sql` with `ticket_transfers` table.
- Run tests and witness passing (GREEN).

### Step 3 (RED & GREEN): Frontend Implementation & Vitest Suites
- Update `frontend/src/types/index.ts` with waitlist and ticket transfer interfaces.
- Update `frontend/src/services/api.ts` with API client methods:
  - `joinWaitlist`, `fetchUserWaitlists`, `cancelWaitlist`, `claimWaitlistReservation`, `transferTicket`.
- Create `frontend/src/components/WaitlistModal.tsx` for joining waitlist on sold-out events.
- Create `frontend/src/components/TicketTransferModal.tsx` for initiating secure ticket transfers.
- Update `frontend/src/components/EventCard.tsx` to detect sold-out state and trigger `WaitlistModal`.
- Update `frontend/src/components/MyBookingsModal.tsx` to add Waitlist tab and Transfer button on tickets.
- Update `frontend/src/components/TicketPassModal.tsx` to add Transfer button.
- Update `frontend/src/components/AdminDashboardModal.tsx` with Waitlist telemetry tab.
- Create `frontend/src/test/waitlist_ticket_transfer.test.ts` verifying all component states.
- Run `npx --no-install vitest run` and confirm GREEN.

### Step 4: End-to-End & Anti-Cheat QA Verification
- Test against live API Gateway port 8080.
- Verify `python3 scripts/verify_e2e.py` passes 100%.
- Audit diffs against DoD.

---

## 4. Definition of Done Checklist
- [ ] RED test written and failure witnessed.
- [ ] Backend Go implementation passing unit tests (`go test -timeout 30s ./...`).
- [ ] Frontend TypeScript and Vitest tests passing (`npx --no-install vitest run`).
- [ ] Gate scanner verifies transferred tickets are invalidated.
- [ ] Master test dataset (`evt-106`, `wl-001`–`wl-004`) verified in UI.
- [ ] Update `docs/ARCHITECTURE.md` with Milestone 27.
- [ ] Clean local git commit on branch `feature/milestone-27-waitlist-ticket-transfer`.
