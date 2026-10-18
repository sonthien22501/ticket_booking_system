# Milestone 17: Interactive 2D Seat Map & Dialog Confirmation (R24, R25) Implementation Plan

## Goal
Implement visual interactive 2D seat map visualization (**Requirement R25**) and dialog confirmation functionality (**Requirement R24**) in the React frontend. Provide an interactive floor grid with dynamic seat statuses (`AVAILABLE`, `HOLD`, `BOOKED`, `SELECTED`), real-time SSE stream integration, seat tooltips, and modal confirmation dialogs for both seat selection and payment submission before state changes occur.

---

## Technical Design & Failure Modes
1. **Interactive 2D Grid (R25)**:
   - Render tiered seating rows (VIP stage front, General admission) with SVG/CSS floor plan styling.
   - Distinct visual state indicators: green (available), amber pulse (held), slate with lock (booked/sold), indigo glow with checkmark (selected).
   - Seat inspection tooltip displaying row, number, tier name, price, and current availability status.
   - Real-time updates: Hook into `/api/inventory/{eventId}/seats/stream` (SSE) to reflect live state changes without page reloads.
2. **Dialog Confirmation Workflows (R24)**:
   - Modal dialogs prompt users before executing state changes:
     - **Seat Selection Confirmation Dialog**: Prompts user when clicking "Proceed to Checkout" with selected seat summary, total price, and action buttons ("Confirm & Proceed to Payment" / "Change Seats").
     - **Payment Authorization Confirmation Dialog**: Prompts user after filling attendee info with full transaction breakdown ("Authorize & Pay $XX.XX" / "Review Order") before dispatching the booking API call.
     - **Selection Reset Confirmation Dialog**: Prompts confirmation if user attempts to clear multi-seat selections.
3. **Graceful Error & Conflict Handling**:
   - If a 409 conflict occurs or user cancels confirmation, dialogs cleanly reset state and refresh the seat map without orphaned holds.

---

## Tasks Breakdown
1. **Task 1: Failing Frontend Test Suite (RED)**:
   - Create `frontend/src/test/seatmap_dialog.test.ts` testing 2D seat map selection, dialog modal triggers, and confirmation flows.
2. **Task 2: Confirmation Dialog Component**:
   - Create `frontend/src/components/ConfirmationDialog.tsx`.
3. **Task 3: Enhance VenueSeatMap & App State**:
   - Update `frontend/src/components/VenueSeatMap.tsx`, `SeatNode.tsx`, and `App.tsx` with dialog gates and SSE stream listener.
4. **Task 4: Build & Verify (GREEN)**:
   - Run `npx vitest run` in `frontend/` and `npm run build`.
   - Verify all tests pass.
5. **Task 5: Documentation & Timed PR Push**:
   - Mark R24 & R25 as completed in `REQUIREMENTS.md`.
   - Update `explain.md` with Milestone 17 guide.
   - Wait 10 minutes from PR 16 push before pushing PR 17 to GitHub.
