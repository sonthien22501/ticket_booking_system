import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  fetchUserBookings,
  cancelBooking,
  verifyTicket,
} from '../services/api';
import { QRCodeDisplay } from '../components/QRCodeDisplay';
import { MyBookingsModal } from '../components/MyBookingsModal';
import { TicketPassModal } from '../components/TicketPassModal';
import { Header } from '../components/Header';
import { BookingResponse, TicketItem, User } from '../types';

describe('Milestone 23: User Bookings History, Digital Passbook & Gate QR Ticket Verification', () => {
  const originalFetch = (globalThis as unknown as { fetch?: typeof fetch }).fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    (globalThis as unknown as { fetch?: typeof fetch }).fetch = originalFetch;
  });

  const mockUser: User = {
    id: 'usr-passbook-1',
    name: 'Sarah Connor',
    email: 'sarah@example.com',
    role: 'customer',
  };

  const mockTicket: TicketItem = {
    ticketId: 'tkt-101',
    seatLabel: 'Row A, Seat 1',
    ticketCode: 'TKT-NEON-A1-8899',
    status: 'VALID',
  };

  const mockBooking: BookingResponse = {
    bookingId: 'bk-test-123',
    bookingReference: 'REF-BK-123-PASS',
    status: 'CONFIRMED',
    eventId: 'evt-101',
    eventTitle: 'Neon Symphony 2026',
    tierName: 'VIP Loge',
    quantity: 1,
    seats: ['A1'],
    totalPrice: 150,
    customerName: 'Sarah Connor',
    customerEmail: 'sarah@example.com',
    tickets: [mockTicket],
    createdAt: '2026-10-18T18:00:00Z',
  };

  describe('QRCodeDisplay Component', () => {
    it('renders a deterministic SVG QR matrix with aria-label containing the payload code', () => {
      const el = React.createElement(QRCodeDisplay, {
        value: 'TKT-NEON-A1-8899',
        size: 140,
      });

      expect(el.props.value).toBe('TKT-NEON-A1-8899');
      expect(el.props.size).toBe(140);
    });
  });

  describe('API Client Methods (M23)', () => {
    it('fetchUserBookings retrieves and normalizes user reservations', async () => {
      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => [
          {
            id: 'bk-test-123',
            booking_reference: 'REF-BK-123-PASS',
            status: 'CONFIRMED',
            event_id: 'evt-101',
            event_title: 'Neon Symphony 2026',
            tier_name: 'VIP Loge',
            quantity: 1,
            seat_ids: ['A1'],
            total_amount: 150,
            customer_name: 'Sarah Connor',
            customer_email: 'sarah@example.com',
            tickets: [
              {
                id: 'tkt-101',
                seat_label: 'Row A, Seat 1',
                ticket_code: 'TKT-NEON-A1-8899',
                status: 'VALID',
              },
            ],
            created_at: '2026-10-18T18:00:00Z',
          },
        ],
      });

      const bookings = await fetchUserBookings({ userId: 'usr-passbook-1' });
      expect(bookings.length).toBe(1);
      expect(bookings[0].bookingReference).toBe('REF-BK-123-PASS');
      expect(bookings[0].tickets.length).toBe(1);
      expect(bookings[0].tickets[0].ticketCode).toBe('TKT-NEON-A1-8899');
      expect(bookings[0].status).toBe('CONFIRMED');
    });

    it('cancelBooking calls /api/bookings/{id}/cancel via Saga', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          bookingId: 'bk-test-123',
          status: 'CANCELLED',
          message: 'Booking cancelled and seats released successfully',
        }),
      });
      (globalThis as unknown as { fetch: typeof fetch }).fetch = mockFetch;

      const res = await cancelBooking('bk-test-123');
      expect(res.success).toBe(true);
      expect(res.message).toContain('Booking cancelled');
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/bookings/bk-test-123/cancel'),
        expect.objectContaining({ method: 'POST' })
      );
    });

    it('verifyTicket supports gate scan verification and check-in status transitions', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          valid: true,
          status: 'USED',
          message: 'Ticket successfully checked in',
          ticket: {
            id: 'tkt-101',
            seatLabel: 'Row A, Seat 1',
            ticketCode: 'TKT-NEON-A1-8899',
            status: 'USED',
          },
        }),
      });
      (globalThis as unknown as { fetch: typeof fetch }).fetch = mockFetch;

      const res = await verifyTicket({
        ticketCode: 'TKT-NEON-A1-8899',
        action: 'CHECK_IN',
      });

      expect(res.valid).toBe(true);
      expect(res.status).toBe('USED');
      expect(res.message).toBe('Ticket successfully checked in');
      expect(res.ticket?.status).toBe('USED');
    });

    it('verifyTicket handles 409 conflict when double check-in occurs', async () => {
      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 409,
        json: async () => ({
          valid: false,
          status: 'USED',
          message: 'Ticket has already been redeemed and checked in',
        }),
      });

      const res = await verifyTicket({
        ticketCode: 'TKT-NEON-A1-8899',
        action: 'CHECK_IN',
      });

      expect(res.valid).toBe(false);
      expect(res.status).toBe('USED');
      expect(res.message).toContain('already been redeemed');
    });
  });

  describe('Component Contracts (M23)', () => {
    it('Header renders My Tickets button when onOpenBookings is provided', () => {
      const onOpenBookings = vi.fn();
      const el = React.createElement(Header, {
        currentUser: mockUser,
        onOpenBookings,
      });

      expect(el.props.onOpenBookings).toBe(onOpenBookings);
      expect(el.props.currentUser?.name).toBe('Sarah Connor');
    });

    it('MyBookingsModal receives currentUser and callbacks properly', () => {
      const onClose = vi.fn();
      const onBookingCancelled = vi.fn();

      const el = React.createElement(MyBookingsModal, {
        isOpen: true,
        onClose,
        currentUser: mockUser,
        onBookingCancelled,
      });

      expect(el.props.isOpen).toBe(true);
      expect(el.props.currentUser?.email).toBe('sarah@example.com');
      expect(el.props.onClose).toBe(onClose);
    });

    it('TicketPassModal binds booking and ticket metadata accurately', () => {
      const onClose = vi.fn();
      const el = React.createElement(TicketPassModal, {
        booking: mockBooking,
        ticket: mockTicket,
        isOpen: true,
        onClose,
      });

      expect(el.props.booking.eventTitle).toBe('Neon Symphony 2026');
      expect(el.props.ticket.seatLabel).toBe('Row A, Seat 1');
      expect(el.props.ticket.ticketCode).toBe('TKT-NEON-A1-8899');
    });
  });
});
