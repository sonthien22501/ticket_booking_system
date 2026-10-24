# Milestone 28: In-App Notification Center & Digital Calendar (.ics) Pass Synchronization Implementation Plan

## 1. Overview & Objectives
Milestone 28 introduces the **In-App Real-Time Notification Center & Digital Calendar (.ics) Pass Synchronization** for the Distributed Event Ticket Booking System. As our ticketing platform expands with high-velocity waitlist promotions, ticket transfers, dynamic surge updates, and global multi-currency orders, attendees and organizers need:
1. **In-App Notification Center**: Instant visibility into critical events—such as waitlist promotions with active claim windows, incoming ticket gifts from peers, booking confirmations, and organizer alerts—without relying solely on external email.
2. **Standard RFC 5545 iCalendar (.ics) Synchronization**: 1-click calendar export directly from digital ticket passes and checkout confirmations into Google Calendar, Apple Calendar, and Microsoft Outlook, complete with venue geolocation, seat assignments, and reminder alarms.

### Delivered Capabilities:
1. **Persistent Notification Store & API (`backend/booking-service`)**:
   - `notifications` relational table in PostgreSQL with indexes on `recipient_email`, `user_id`, and `is_read`.
   - `GET /api/notifications` (and `/notifications`): Queries user notifications with live unread counter (`unreadCount`).
   - `POST /api/notifications/:id/read`: Marks an individual notification as read.
   - `POST /api/notifications/read-all`: Marks all notifications for a user/email as read in one atomic transaction.
   - `POST /api/notifications`: System/admin endpoint to publish announcements or custom alerts.
   - **Automated Lifecycle Triggers**:
     - Booking Confirmation -> automatically creates `BOOKING_CONFIRMED` notification.
     - Digital Ticket Transfer -> automatically generates `TICKET_TRANSFERRED` alert for the recipient and transfer confirmation for the sender.
     - Waitlist Promotion -> emits `WAITLIST_PROMOTED` notification with active hold claim link and 15-minute countdown alert.
2. **Standard RFC 5545 iCalendar (.ics) Generation (`backend/booking-service`)**:
   - `GET /api/bookings/:id/calendar.ics`: Returns binary/text `text/calendar; charset=utf-8` stream formatted according to RFC 5545 specifications (`BEGIN:VCALENDAR`, `BEGIN:VEVENT`, `UID`, `SUMMARY`, `LOCATION`, `DTSTART`, `DTEND`, `DESCRIPTION` with seat labels & ticket references, `VALARM` 2 hours prior).
3. **API Gateway Routing (`backend/api-gateway`)**:
   - Route `/api/notifications` and `/notifications` to `booking_service`.
4. **Cinematic Frontend Notification Center (`frontend/src`)**:
   - `NotificationCenter.tsx`: Glassmorphism dropdown panel attached to header bell.
   - Live badge counter in `Header.tsx` indicating unread notifications with luminous glow.
   - Actionable notifications: clicking a waitlist promotion directly opens the Passbook waitlist claim tab; clicking a ticket transfer opens the ticket pass.
   - 1-click "Mark all as read".
5. **1-Click Calendar Pass Export (`frontend/src`)**:
   - Calendar export button in `TicketPassModal.tsx` and `BookingConfirmation.tsx` invoking `downloadBookingCalendarICS(bookingId, eventTitle)`.
6. **Automated Verification**:
   - Go unit tests in `backend/booking-service/notification_calendar_test.go`.
   - Vitest unit tests in `frontend/src/test/notification_calendar.test.ts`.

---

## 2. Review Focus (Top 5 Edge Cases & Failure Modes)
1. **RFC 5545 Date-Time Syntax & CRLF Endings**: The calendar `.ics` format strictly requires `\r\n` (CRLF) line endings and UTC `YYYYMMDDTHHMMSSZ` format; failure to adhere prevents Apple/Google Calendar from parsing the invitation.
2. **Zero-Unread Counter Accuracy**: Marking individual notifications or "read-all" must accurately decrement `unreadCount` to 0 without ghost badge counts.
3. **Recipient Passbook & Notification Routing**: When tickets are transferred via `POST /api/tickets/transfer`, the recipient must immediately receive a `TICKET_TRANSFERRED` notification queryable under their email.
4. **Missing or Corrupted Booking Handling in Calendar Export**: Exporting `.ics` for a nonexistent booking ID must cleanly return HTTP 404 with structured JSON error, rather than an empty 200 calendar file.
5. **Anonymous vs Authenticated Notification Scoping**: Fetching `/api/notifications` must support both `userId` and `email` query parameters to accommodate registered users and guest checkout attendees.

---

## 3. Implementation Steps (Strict RED-GREEN TDD)

### Step 1 (RED): Backend Tests for Notifications & Calendar Export
- Create `backend/booking-service/notification_calendar_test.go`:
  - Test `GET /api/notifications` with unread count calculation.
  - Test `POST /api/notifications/:id/read` and `POST /api/notifications/read-all`.
  - Test automatic notification dispatch upon booking confirmation and ticket transfer.
  - Test `GET /api/bookings/:id/calendar.ics` asserting RFC 5545 calendar headers, VEVENT properties, and seat details.
  - Test 404 handling on invalid booking calendar export.
- Run Go tests and witness failure (RED).

### Step 2 (GREEN): Backend Implementation
- Create `backend/booking-service/notification.go`:
  - Structs: `Notification`, `NotificationResponse`, `CreateNotificationRequest`.
  - In-memory fallback and SQL query implementation for Postgres `notifications` table.
  - Handlers: `handleNotifications`, `handleNotificationRead`, `handleNotificationReadAll`.
  - Helper functions: `dispatchNotification(db, notif)`.
- Create `backend/booking-service/calendar.go`:
  - `generateCalendarICS(booking, event, tickets)` returning standard RFC 5545 string.
  - Handler: `handleBookingCalendarICS(w, r)`.
- Wire routes and handlers in `backend/booking-service/main.go`.
- Wire lifecycle notification triggers in `createBooking` and `handleTransferTicket`.
- Add `/api/notifications` to `backend/api-gateway/nginx.conf`.
- Add `notifications` schema to `infra/postgres/init.sql` and `scripts/seed_test_dataset.sql`.
- Run Go tests and verify passing (GREEN).

### Step 3 (RED & GREEN): Frontend Implementation & Vitest Suites
- Update `frontend/src/types/index.ts` with `AppNotification` interface.
- Add notification and calendar API methods to `frontend/src/services/api.ts`.
- Create `frontend/src/components/NotificationCenter.tsx`.
- Update `frontend/src/components/Header.tsx` to include notification bell and popover toggle.
- Update `frontend/src/components/TicketPassModal.tsx` and `BookingConfirmation.tsx` to add "Add to Calendar" button.
- Create `frontend/src/test/notification_calendar.test.ts`.
- Run `npx --no-install vitest run` and confirm GREEN.

### Step 4: End-to-End & Anti-Cheat QA Verification
- Test against live API Gateway port 8080.
- Verify `python3 scripts/verify_e2e.py` passes 100%.
- Format modified files with `gofmt` and `prettier`.
- Local git commit with conventional message on `feature/milestone-28-notifications-calendar-sync`.

---

## 4. Definition of Done Checklist
- [ ] RED test written and failure witnessed.
- [ ] Backend Go implementation passing unit tests (`go test -timeout 30s ./...`).
- [ ] Frontend TypeScript and Vitest tests passing (`npx --no-install vitest run`).
- [ ] Calendar `.ics` output complies with RFC 5545 specifications.
- [ ] Notification bell and unread badge operate deterministically.
- [ ] Master test dataset reset and `scripts/verify_e2e.py` passing 100%.
- [ ] Update `docs/ARCHITECTURE.md` with Milestone 28.
- [ ] Clean local git commit on branch `feature/milestone-28-notifications-calendar-sync`.
