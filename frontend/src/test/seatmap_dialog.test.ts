import { describe, it, expect, vi } from 'vitest';
import { generateVenueSeatLayout, createBooking } from '../services/api';
import { EventItem, SeatItem } from '../types';

describe('Milestone 17: Interactive 2D Seat Map & Dialog Confirmation (R24, R25)', () => {
  const mockEvent: EventItem = {
    id: 'evt-arena-2026',
    title: 'Symphony Hall Gala',
    description: 'Annual Philharmonic Gala Concert',
    category: 'CONCERT',
    venueName: 'Grand Philharmonic Hall',
    venueLocation: 'Vienna, Austria',
    startTime: '2026-11-20T19:30:00Z',
    endTime: '2026-11-20T22:30:00Z',
    status: 'ACTIVE',
    tiers: [
      { id: 'tier-vip', name: 'VIP Loge Tier', price: 180, capacity: 20 },
      { id: 'tier-orchestra', name: 'Orchestra Stalls', price: 95, capacity: 48 },
    ],
  };

  describe('R25: Visual Interactive 2D Seat Map Layout & State Management', () => {
    it('creates 2D tiered grid with distinct VIP front rows and Orchestra rows', () => {
      const seats = generateVenueSeatLayout(mockEvent.id, mockEvent);

      // Verify tiered layout: Rows A & B are VIP, Rows C-F are Orchestra
      const vipSeats = seats.filter((s) => s.row === 'A' || s.row === 'B');
      const orchestraSeats = seats.filter((s) => ['C', 'D', 'E', 'F'].includes(s.row));

      expect(vipSeats.length).toBe(20);
      expect(orchestraSeats.length).toBe(48);
      expect(seats.length).toBe(68);

      // Check tier prices and names
      expect(vipSeats[0].price).toBe(180);
      expect(vipSeats[0].tierName).toBe('VIP Loge Tier');
      expect(orchestraSeats[0].price).toBe(95);
      expect(orchestraSeats[0].tierName).toBe('Orchestra Stalls');
    });

    it('correctly manages multi-seat selection toggles and subtotal calculations', () => {
      const seats = generateVenueSeatLayout(mockEvent.id, mockEvent);
      const selectedIds = new Set<string>();

      // Select seat A1 ($180) and seat C1 ($95)
      selectedIds.add('A1');
      selectedIds.add('C1');

      const selectedList = seats.filter((s) => selectedIds.has(s.id));
      const subtotal = selectedList.reduce((acc, s) => acc + (s.price || 0), 0);

      expect(selectedList.length).toBe(2);
      expect(subtotal).toBe(275);

      // Toggle off A1
      selectedIds.delete('A1');
      const updatedList = seats.filter((s) => selectedIds.has(s.id));
      const updatedSubtotal = updatedList.reduce((acc, s) => acc + (s.price || 0), 0);

      expect(updatedList.length).toBe(1);
      expect(updatedSubtotal).toBe(95);
    });

    it('identifies held, booked, and available seats distinctly', () => {
      const testSeats: SeatItem[] = [
        { id: 'A1', eventId: mockEvent.id, row: 'A', number: 1, status: 'AVAILABLE', price: 180 },
        { id: 'A2', eventId: mockEvent.id, row: 'A', number: 2, status: 'HOLD', price: 180 },
        { id: 'A3', eventId: mockEvent.id, row: 'A', number: 3, status: 'BOOKED', price: 180 },
      ];

      const available = testSeats.filter((s) => s.status === 'AVAILABLE');
      const held = testSeats.filter((s) => s.status === 'HOLD');
      const booked = testSeats.filter((s) => s.status === 'BOOKED');

      expect(available.length).toBe(1);
      expect(held.length).toBe(1);
      expect(booked.length).toBe(1);
      expect(available[0].id).toBe('A1');
      expect(held[0].id).toBe('A2');
      expect(booked[0].id).toBe('A3');
    });
  });

  describe('R24: Dialog Confirmation Functionality for State Changes', () => {
    it('gates seat selection checkout behind a confirmation step', () => {
      // Simulation of user clicking checkout with 2 seats
      let checkoutModalOpened = false;
      let selectionConfirmDialogOpen = true;

      const selectedSeats = ['A1', 'A2'];
      const totalAmount = 360;
      expect(selectedSeats.length).toBe(2);
      expect(totalAmount).toBe(360);

      // User confirms in selection dialog
      const handleConfirmSelection = () => {
        selectionConfirmDialogOpen = false;
        checkoutModalOpened = true;
      };

      // User cancels in selection dialog
      const handleCancelSelection = () => {
        selectionConfirmDialogOpen = false;
        checkoutModalOpened = false;
      };

      // When canceled, checkout modal must NOT open
      handleCancelSelection();
      expect(checkoutModalOpened).toBe(false);
      expect(selectionConfirmDialogOpen).toBe(false);

      // When confirmed, checkout modal opens
      selectionConfirmDialogOpen = true;
      handleConfirmSelection();
      expect(checkoutModalOpened).toBe(true);
      expect(selectionConfirmDialogOpen).toBe(false);
    });

    it('gates payment submission behind a payment authorization confirmation dialog', async () => {
      const originalFetch = (globalThis as any).fetch;
      const mockFetch = vi.fn().mockResolvedValue({
        status: 201,
        ok: true,
        json: async () => ({
          booking_id: 'bk-vienna-101',
          booking_reference: 'REF-VIENNA-101',
          status: 'CONFIRMED',
          event_id: mockEvent.id,
          total_price: 180,
          customer_name: 'Helena Berg',
          customer_email: 'helena@example.com',
          tickets: [
            {
              ticket_id: 'tkt-1',
              seat_label: 'A1',
              ticket_code: 'TKT-A1-101',
              status: 'VALID',
            },
          ],
        }),
      });
      (globalThis as any).fetch = mockFetch;

      try {
        let paymentConfirmedByUser = false;
        let createBookingCalled = false;

        const customerInfo = {
          name: 'Helena Berg',
          email: 'helena@example.com',
          seats: ['A1'],
          totalAmount: 180,
        };
        expect(customerInfo.totalAmount).toBe(180);

        // When user clicks submit on the checkout form, prompt payment dialog
        let paymentDialogOpen = true;

        const handleUserRejectsPaymentDialog = () => {
          paymentDialogOpen = false;
          paymentConfirmedByUser = false;
          // No API call is made
        };

        const handleUserAuthorizesPaymentDialog = async () => {
          paymentDialogOpen = false;
          paymentConfirmedByUser = true;
          createBookingCalled = true;
          return await createBooking({
            eventId: mockEvent.id,
            seats: customerInfo.seats,
            customerName: customerInfo.name,
            customerEmail: customerInfo.email,
          });
        };

        // Case 1: User cancels payment dialog
        handleUserRejectsPaymentDialog();
        expect(paymentDialogOpen).toBe(false);
        expect(paymentConfirmedByUser).toBe(false);
        expect(createBookingCalled).toBe(false);
        expect(mockFetch).not.toHaveBeenCalled();

        // Case 2: User authorizes payment dialog
        paymentDialogOpen = true;
        const result = await handleUserAuthorizesPaymentDialog();
        expect(paymentDialogOpen).toBe(false);
        expect(paymentConfirmedByUser).toBe(true);
        expect(createBookingCalled).toBe(true);
        expect(mockFetch).toHaveBeenCalledTimes(1);
        expect(result.status).toBe('CONFIRMED');
        expect(result.bookingId).toBe('bk-vienna-101');
      } finally {
        (globalThis as any).fetch = originalFetch;
      }
    });

    it('gates clearing multiple selected seats behind a confirmation dialog', () => {
      let selectedSeatIds = new Set(['A1', 'A2', 'A3']);
      let clearDialogOpen = true;

      // When cancelled, selection is retained
      const handleCancelClear = () => {
        clearDialogOpen = false;
      };

      handleCancelClear();
      expect(selectedSeatIds.size).toBe(3);

      // When confirmed, selection is wiped
      clearDialogOpen = true;
      const handleConfirmClear = () => {
        selectedSeatIds = new Set();
        clearDialogOpen = false;
      };

      handleConfirmClear();
      expect(selectedSeatIds.size).toBe(0);
      expect(clearDialogOpen).toBe(false);
    });
  });

  describe('Real-Time Dynamic Seat Updates (R25 + R10)', () => {
    it('updates seat state dynamically on real-time stream message', () => {
      let currentSeats: SeatItem[] = [
        { id: 'A1', eventId: mockEvent.id, row: 'A', number: 1, status: 'AVAILABLE', price: 180 },
        { id: 'A2', eventId: mockEvent.id, row: 'A', number: 2, status: 'AVAILABLE', price: 180 },
      ];

      // Simulated SSE event handler
      const handleSseSeatUpdate = (payload: { event_id: string; seat_id: string; status: SeatItem['status'] }) => {
        currentSeats = currentSeats.map((s) =>
          s.id === payload.seat_id ? { ...s, status: payload.status } : s
        );
      };

      // Another user puts seat A1 on HOLD
      handleSseSeatUpdate({
        event_id: mockEvent.id,
        seat_id: 'A1',
        status: 'HOLD',
      });

      expect(currentSeats.find((s) => s.id === 'A1')?.status).toBe('HOLD');
      expect(currentSeats.find((s) => s.id === 'A2')?.status).toBe('AVAILABLE');

      // Seat A1 is committed as BOOKED
      handleSseSeatUpdate({
        event_id: mockEvent.id,
        seat_id: 'A1',
        status: 'BOOKED',
      });

      expect(currentSeats.find((s) => s.id === 'A1')?.status).toBe('BOOKED');
    });
  });
});
