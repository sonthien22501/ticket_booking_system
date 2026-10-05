import { describe, it, expect, vi } from 'vitest';
import {
  generateVenueSeatLayout,
  createBooking,
  fetchInventory,
} from '../services/api';
import { EventItem } from '../types';

describe('Seat Layout Generation', () => {
  const mockEvent: EventItem = {
    id: 'evt-test-1',
    title: 'Rock Concert 2026',
    description: 'Live test show',
    category: 'CONCERT',
    venueName: 'Test Arena',
    venueLocation: 'New York, USA',
    startTime: '2026-12-01T20:00:00Z',
    endTime: '2026-12-01T23:00:00Z',
    status: 'ACTIVE',
    tiers: [
      { id: 'tier-vip', name: 'VIP Front Row', price: 200, capacity: 20 },
      { id: 'tier-ga', name: 'General Admission', price: 75, capacity: 48 },
    ],
  };

  it('generates rows A through F with VIP rows having 10 seats and GA rows having 12 seats', () => {
    const seats = generateVenueSeatLayout(mockEvent.id, mockEvent);

    // Rows A & B (VIP): 2 * 10 = 20 seats
    // Rows C, D, E, F (GA): 4 * 12 = 48 seats
    // Total = 68 seats
    expect(seats.length).toBe(68);

    const rowASeats = seats.filter((s) => s.row === 'A');
    expect(rowASeats.length).toBe(10);
    expect(rowASeats[0].tierId).toBe('tier-vip');
    expect(rowASeats[0].price).toBe(200);

    const rowCSeats = seats.filter((s) => s.row === 'C');
    expect(rowCSeats.length).toBe(12);
    expect(rowCSeats[0].tierId).toBe('tier-ga');
    expect(rowCSeats[0].price).toBe(75);
  });

  it('contains pre-booked seats to ensure realistic venue state', () => {
    const seats = generateVenueSeatLayout(mockEvent.id, mockEvent);
    const bookedSeats = seats.filter((s) => s.status === 'BOOKED');
    expect(bookedSeats.length).toBeGreaterThan(0);
    expect(bookedSeats.some((s) => s.id === 'A3')).toBe(true);
  });
});

describe('Booking API Client', () => {
  it('throws 409 ApiError when seat conflict is returned by backend', async () => {
    // Mock global fetch to return 409
    const originalFetch = (globalThis as any).fetch;
    (globalThis as any).fetch = vi.fn().mockResolvedValue({
      status: 409,
      ok: false,
      json: async () => ({
        error: 'SEAT_UNAVAILABLE',
        message: 'The requested seat(s) are no longer available',
        conflicting_seats: ['A1'],
      }),
    } as unknown as Response);

    try {
      await expect(
        createBooking({
          eventId: 'evt-test-1',
          seats: ['A1'],
          customerName: 'Alice',
          customerEmail: 'alice@example.com',
        })
      ).rejects.toMatchObject({
        status: 409,
        error: 'SEAT_UNAVAILABLE',
        conflictingSeats: ['A1'],
      });
    } finally {
      (globalThis as any).fetch = originalFetch;
    }
  });

  it('returns confirmed booking when reservation succeeds with 201', async () => {
    const originalFetch = (globalThis as any).fetch;
    (globalThis as any).fetch = vi.fn().mockResolvedValue({
      status: 201,
      ok: true,
      json: async () => ({
        booking_id: 'bk-999',
        booking_reference: 'REF-TEST-999',
        status: 'CONFIRMED',
        event_id: 'evt-test-1',
        total_price: 200,
        customer_name: 'Bob',
        customer_email: 'bob@example.com',
        tickets: [
          {
            ticket_id: 'tkt-1',
            seat_label: 'Seat A1',
            ticket_code: 'TKT-TEST-A1',
            status: 'VALID',
          },
        ],
      }),
    } as unknown as Response);

    try {
      const result = await createBooking({
        eventId: 'evt-test-1',
        seats: ['A1'],
        customerName: 'Bob',
        customerEmail: 'bob@example.com',
      });

      expect(result.status).toBe('CONFIRMED');
      expect(result.bookingReference).toBe('REF-TEST-999');
      expect(result.tickets.length).toBe(1);
      expect(result.totalPrice).toBe(200);
    } finally {
      (globalThis as any).fetch = originalFetch;
    }
  });

  it('fetches and normalizes inventory summary accurately', async () => {
    const originalFetch = (globalThis as any).fetch;
    (globalThis as any).fetch = vi.fn().mockResolvedValue({
      status: 200,
      ok: true,
      json: async () => ({
        event_id: 'evt-test-1',
        summary: {
          total_seats: 100,
          available_seats: 98,
          reserved_seats: 1,
          booked_seats: 1,
        },
        tiers: [
          {
            tier_id: 'tier-vip',
            tier_name: 'VIP Front Row',
            total_seats: 20,
            available_seats: 19,
            price: 200,
          },
        ],
      }),
    } as unknown as Response);

    try {
      const inv = await fetchInventory('evt-test-1');
      expect(inv.eventId).toBe('evt-test-1');
      expect(inv.availableSeats).toBe(98);
      expect(inv.totalSeats).toBe(100);
      expect(inv.tiers?.length).toBe(1);
      expect(inv.tiers?.[0].availableSeats).toBe(19);
    } finally {
      (globalThis as any).fetch = originalFetch;
    }
  });
});

