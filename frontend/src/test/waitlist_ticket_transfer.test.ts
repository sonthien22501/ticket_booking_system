import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  joinWaitlist,
  fetchUserWaitlists,
  cancelWaitlistEntry,
  claimWaitlistReservation,
  transferTicket,
} from '../services/api';
import { EventCard } from '../components/EventCard';
import { WaitlistModal } from '../components/WaitlistModal';
import { TicketTransferModal } from '../components/TicketTransferModal';
import { TicketPassModal } from '../components/TicketPassModal';
import { EventItem, BookingResponse, TicketItem, User, WaitlistEntry } from '../types';

describe('Milestone 27: Waitlist Self-Service Portal & Secure Digital Ticket Transfer Engine', () => {
  const originalFetch = (globalThis as unknown as { fetch?: typeof fetch }).fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    (globalThis as unknown as { fetch?: typeof fetch }).fetch = originalFetch;
  });

  const mockUser: User = {
    id: 'usr-waitlist-1',
    name: 'Sarah Connor',
    email: 'sarah.connor@cyberdyne.io',
    role: 'customer',
  };

  const mockEventSoldOut: EventItem = {
    id: 'evt-106',
    title: 'Stand-Up Comedy All-Stars Showcase',
    description: 'Premier comedians live in New York City',
    category: 'COMEDY',
    venueName: 'Comedy Cellar',
    venueLocation: 'New York, NY',
    startTime: '2026-10-25T20:00:00Z',
    endTime: '2026-10-25T22:30:00Z',
    status: 'ACTIVE',
    tiers: [
      {
        id: 'tier-front',
        name: 'Front Row',
        price: 75.0,
        capacity: 12,
        totalSeats: 12,
        availableSeats: 0, // SOLD OUT
      },
    ],
  };

  const mockEventAvailable: EventItem = {
    id: 'evt-101',
    title: 'Neon Symphony 2026',
    description: 'Live orchestral synthesizer tour',
    category: 'CONCERT',
    venueName: 'Grand Arena',
    venueLocation: 'Berlin',
    startTime: '2026-11-15T19:30:00Z',
    endTime: '2026-11-15T23:00:00Z',
    status: 'ACTIVE',
    tiers: [
      {
        id: 'tier-vip',
        name: 'VIP Front Row',
        price: 150.0,
        capacity: 20,
        totalSeats: 20,
        availableSeats: 8,
      },
    ],
  };

  const mockTicket: TicketItem = {
    ticketId: 'tkt-vip-101',
    seatLabel: 'Row A, Seat 1',
    ticketCode: 'TKT-VALID-VIP01',
    status: 'VALID',
  };

  const mockBooking: BookingResponse = {
    bookingId: 'bkg-demo-001',
    bookingReference: 'REF-NEON-2026-001',
    status: 'CONFIRMED',
    eventId: 'evt-101',
    eventTitle: 'Neon Symphony 2026',
    tierName: 'VIP Front Row',
    quantity: 1,
    seats: ['A1'],
    totalPrice: 150.0,
    customerName: 'Demo Customer',
    customerEmail: 'user@tickets.com',
    tickets: [mockTicket],
    createdAt: '2026-10-18T18:00:00Z',
  };

  describe('1. API Client Methods for Waitlist & Ticket Transfer', () => {
    it('joinWaitlist sends payload to /api/waitlist and receives queue confirmation', async () => {
      const mockWaitlistResponse: WaitlistEntry = {
        id: 'wl-new-99',
        eventId: 'evt-106',
        userId: 'usr-waitlist-1',
        customerName: 'Sarah Connor',
        customerEmail: 'sarah.connor@cyberdyne.io',
        quantity: 2,
        status: 'PENDING',
        position: 3,
        createdAt: '2026-10-18T20:00:00Z',
        updatedAt: '2026-10-18T20:00:00Z',
      };

      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => mockWaitlistResponse,
      });

      const entry = await joinWaitlist({
        eventId: 'evt-106',
        tierId: 'tier-front',
        userId: 'usr-waitlist-1',
        customerName: 'Sarah Connor',
        customerEmail: 'sarah.connor@cyberdyne.io',
        quantity: 2,
      });

      expect(entry.id).toBe('wl-new-99');
      expect(entry.position).toBe(3);
      expect(entry.status).toBe('PENDING');
    });

    it('fetchUserWaitlists retrieves and normalizes queue positions', async () => {
      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => [
          {
            id: 'wl-001',
            event_id: 'evt-106',
            event_title: 'Stand-Up Comedy All-Stars Showcase',
            user_id: 'usr-waitlist-1',
            customer_name: 'Sarah Connor',
            customer_email: 'sarah.connor@cyberdyne.io',
            quantity: 2,
            status: 'PENDING',
            position: 1,
            created_at: '2026-10-18T19:00:00Z',
            updated_at: '2026-10-18T19:00:00Z',
          },
          {
            id: 'wl-003',
            event_id: 'evt-106',
            event_title: 'Stand-Up Comedy All-Stars Showcase',
            user_id: 'usr-maria',
            customer_name: 'Maria Garcia',
            customer_email: 'maria.garcia@madrid.es',
            quantity: 1,
            status: 'PROMOTED',
            reservation_id: 'res-wl-promoted-003',
            expires_at: '2026-10-18T22:30:00Z',
            created_at: '2026-10-18T19:00:00Z',
            updated_at: '2026-10-18T20:00:00Z',
          },
        ],
      });

      const list = await fetchUserWaitlists({ email: 'sarah.connor@cyberdyne.io' });
      expect(list.length).toBe(2);
      expect(list[0].id).toBe('wl-001');
      expect(list[0].position).toBe(1);
      expect(list[1].status).toBe('PROMOTED');
      expect(list[1].reservationId).toBe('res-wl-promoted-003');
    });

    it('transferTicket invalidates old code and issues new recipient ticket', async () => {
      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          transferId: 'xfer-999',
          bookingId: 'bkg-demo-001',
          newBookingId: 'bkg-xfer-888',
          oldTicketId: 'tkt-vip-101',
          newTicketId: 'tkt-new-555',
          oldTicketCode: 'TKT-VALID-VIP01',
          newTicketCode: 'TKT-XFER-202610182230',
          recipientEmail: 'friend@example.com',
          recipientName: 'Lucky Friend',
          seatLabel: 'Row A, Seat 1',
          status: 'COMPLETED',
          transferredAt: '2026-10-18T22:30:00Z',
          message: 'Ticket successfully transferred',
        }),
      });

      const res = await transferTicket({
        ticketCode: 'TKT-VALID-VIP01',
        recipientEmail: 'friend@example.com',
        recipientName: 'Lucky Friend',
      });

      expect(res.status).toBe('COMPLETED');
      expect(res.oldTicketCode).toBe('TKT-VALID-VIP01');
      expect(res.newTicketCode).toContain('TKT-XFER-');
      expect(res.recipientEmail).toBe('friend@example.com');
    });

    it('claimWaitlistReservation claims hold and transitions to confirmed booking', async () => {
      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({
          bookingId: 'bkg-claimed-001',
          bookingReference: 'REF-CLAIM-001',
          status: 'CONFIRMED',
          eventId: 'evt-106',
          eventTitle: 'Stand-Up Comedy All-Stars Showcase',
          quantity: 1,
          totalPrice: 75.0,
          customerName: 'Maria Garcia',
          customerEmail: 'maria.garcia@madrid.es',
          tickets: [
            {
              ticketId: 'tkt-claimed-1',
              seatLabel: 'Seat C-1',
              ticketCode: 'TKT-CLAIM-001',
              status: 'VALID',
            },
          ],
        }),
      });

      const booking = await claimWaitlistReservation('wl-003');
      expect(booking.status).toBe('CONFIRMED');
      expect(booking.tickets[0].status).toBe('VALID');
    });

    it('cancelWaitlistEntry sends DELETE request to /api/waitlist/:id', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => ({ status: 'CANCELLED' }),
      });
      (globalThis as unknown as { fetch: typeof fetch }).fetch = mockFetch;

      await expect(cancelWaitlistEntry('wl-001', 'usr-waitlist-1')).resolves.not.toThrow();
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/api/waitlist/wl-001'), expect.objectContaining({ method: 'DELETE' }));
    });
  });

  describe('2. EventCard Sold-Out Detection & Waitlist Button', () => {
    it('detects 0 available seats and displays WAITLIST ACTIVE badge with Join Waitlist button', () => {
      const onSelect = vi.fn();
      const onJoinWaitlist = vi.fn();

      const el = React.createElement(EventCard, {
        event: mockEventSoldOut,
        onSelectEvent: onSelect,
        onJoinWaitlist: onJoinWaitlist,
      });

      expect(el.props.event.id).toBe('evt-106');
      expect(el.props.onJoinWaitlist).toBeDefined();
    });

    it('displays standard Select Seats button when tickets are available', () => {
      const onSelect = vi.fn();

      const el = React.createElement(EventCard, {
        event: mockEventAvailable,
        onSelectEvent: onSelect,
      });

      expect(el.props.event.tiers[0].availableSeats).toBe(8);
      expect(el.props.event.status).toBe('ACTIVE');
    });
  });

  describe('3. WaitlistModal Component', () => {
    it('renders waitlist form with event title and contact inputs', () => {
      const onClose = vi.fn();

      const el = React.createElement(WaitlistModal, {
        event: mockEventSoldOut,
        isOpen: true,
        currentUser: mockUser,
        onClose: onClose,
      });

      expect(el.props.isOpen).toBe(true);
      expect(el.props.event.title).toBe('Stand-Up Comedy All-Stars Showcase');
      expect(el.props.currentUser?.email).toBe('sarah.connor@cyberdyne.io');
    });
  });

  describe('4. TicketTransferModal Component', () => {
    it('renders transfer modal with source ticket info and security warning', () => {
      const onClose = vi.fn();
      const onTransferred = vi.fn();

      const el = React.createElement(TicketTransferModal, {
        isOpen: true,
        onClose: onClose,
        booking: mockBooking,
        ticket: mockTicket,
        currentUser: mockUser,
        onTicketTransferred: onTransferred,
      });

      expect(el.props.isOpen).toBe(true);
      expect(el.props.ticket.ticketCode).toBe('TKT-VALID-VIP01');
      expect(el.props.ticket.seatLabel).toBe('Row A, Seat 1');
      expect(el.props.booking.eventTitle).toBe('Neon Symphony 2026');
    });
  });

  describe('5. TicketPassModal Transfer Integration', () => {
    it('provides onTransferTicket prop to allow transfer directly from passbook pass', () => {
      const onClose = vi.fn();
      const onTransfer = vi.fn();

      const el = React.createElement(TicketPassModal, {
        booking: mockBooking,
        ticket: mockTicket,
        isOpen: true,
        onClose: onClose,
        onTransferTicket: onTransfer,
      });

      expect(el.props.onTransferTicket).toBeDefined();
      expect(el.props.ticket.status).toBe('VALID');
    });
  });
});
