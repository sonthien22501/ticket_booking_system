import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fetchSeats } from '../services/api';
import { EventItem } from '../types';
// @ts-ignore
import fs from 'node:fs';
// @ts-ignore
import path from 'node:path';

// @ts-ignore
declare const __dirname: string;
import { EventCard } from '../components/EventCard';
import { VenueSeatMap } from '../components/VenueSeatMap';
import { TicketPassModal } from '../components/TicketPassModal';
import { MyBookingsModal } from '../components/MyBookingsModal';
import { BookingResponse, TicketItem } from '../types';

describe('M40 Fixes', () => {
  const originalFetch = globalThis.fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
  });

  describe('Bug 1: Seat price & tierName enrichment in fetchSeats', () => {
    it('enriches seats with price and tierName from eventDetails.tiers when server returns only tierId', async () => {
      globalThis.fetch = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => [
          { id: 'A1', tierId: 'tier-vip-101', row: 'A', number: 1, status: 'AVAILABLE' },
        ],
      });

      const eventWithTiers: EventItem = {
        id: 'evt-101',
        title: 'Neon Symphony',
        description: 'Symphony Concert',
        venueName: 'Concert Hall',
        venueLocation: 'NYC',
        startTime: '2026-12-01T20:00:00Z',
        endTime: '2026-12-01T23:00:00Z',
        category: 'CONCERT',
        status: 'ACTIVE',
        tiers: [
          { id: 'tier-vip-101', name: 'VIP', price: 150 },
          { id: 'tier-ga-101', name: 'General Admission', price: 50 },
        ],
      };

      const seats = await fetchSeats('evt-101', eventWithTiers);
      expect(seats.length).toBe(1);
      expect(seats[0].price).toBe(150);
      expect(seats[0].tierName).toBe('VIP');
    });
  });

  describe('Bug 2: Currency selector propagates to displayed prices', () => {
    it('renders EventCard with currentCurrency="EUR" and tier price 100 showing €92.00', () => {
      const mockEvent: EventItem = {
        id: 'evt-currency-1',
        title: 'Euro Concert',
        description: 'Concert in EUR',
        category: 'CONCERT',
        venueName: 'Euro Arena',
        venueLocation: 'Paris, France',
        startTime: '2026-12-01T20:00:00Z',
        endTime: '2026-12-01T23:00:00Z',
        status: 'ACTIVE',
        tiers: [{ id: 'tier-1', name: 'Standard', price: 100 }],
      };

      const html = renderToStaticMarkup(
        React.createElement(EventCard as any, {
          event: mockEvent,
          onSelectEvent: vi.fn(),
          currentCurrency: 'EUR',
        })
      );

      expect(html).toContain('€92.00');
    });

    it('renders VenueSeatMap selection footer with selected seat price 100 and currentCurrency="GBP" showing £79.00', () => {
      const mockEvent: EventItem = {
        id: 'evt-currency-2',
        title: 'London Gig',
        description: 'Gig in GBP',
        category: 'CONCERT',
        venueName: 'London Hall',
        venueLocation: 'London, UK',
        startTime: '2026-12-01T20:00:00Z',
        endTime: '2026-12-01T23:00:00Z',
        status: 'ACTIVE',
        tiers: [{ id: 'tier-1', name: 'Standard', price: 100 }],
      };

      const seats = [
        { id: 'A1', row: 'A', number: 1, status: 'AVAILABLE' as const, price: 100 },
      ];

      const html = renderToStaticMarkup(
        React.createElement(VenueSeatMap as any, {
          event: mockEvent,
          seats,
          selectedSeatIds: new Set(['A1']),
          loadingSeats: false,
          onToggleSeat: vi.fn(),
          onClearSelection: vi.fn(),
          onProceedToCheckout: vi.fn(),
          onBackToCatalog: vi.fn(),
          onRefreshSeats: vi.fn(),
          currentCurrency: 'GBP',
        })
      );

      expect(html).toContain('£79.00');
    });
  });

  describe('Bug 4: Passbook / Ticket Pass UI Layout & Light Theme Polish', () => {
    const mockTicket: TicketItem = {
      ticketId: 'tkt-test-1',
      seatLabel: 'Row A, Seat 1',
      ticketCode: 'TKT-TEST-A1-001',
      status: 'VALID',
    };

    const mockBooking: BookingResponse = {
      bookingId: 'bk-test-456',
      bookingReference: 'REF-BK-456-PASSBOOK-LONG-REF',
      status: 'CONFIRMED',
      eventId: 'evt-101',
      eventTitle: 'Neon Symphony 2026',
      quantity: 1,
      seats: ['A1'],
      totalPrice: 150,
      customerName: 'Alice Tester',
      customerEmail: 'alice@example.com',
      tickets: [mockTicket],
      createdAt: '2026-10-20T10:00:00Z',
    };

    it('TicketPassModal action container has flex-wrap', () => {
      const html = renderToStaticMarkup(
        React.createElement(TicketPassModal, {
          booking: mockBooking,
          ticket: mockTicket,
          isOpen: true,
          onClose: vi.fn(),
          onTransferTicket: vi.fn(),
        })
      );

      expect(html).toContain('flex-wrap');
    });

    it('MyBookingsModal ticket action buttons have whitespace-nowrap and single-column full-width grid', () => {
      const html = renderToStaticMarkup(
        React.createElement(MyBookingsModal as any, {
          isOpen: true,
          onClose: vi.fn(),
          currentUser: { id: 'u1', name: 'Alice', email: 'alice@example.com', role: 'customer' },
          initialBookings: [mockBooking],
        })
      );

      expect(html).toContain('whitespace-nowrap');
      expect(html).toContain('grid-cols-1');
      expect(html).not.toContain('sm:grid-cols-2');
    });

    it('defines light-theme surface adapters for hex modal backgrounds and gradient headers in index.css', () => {
      const cssPath = path.resolve(__dirname, '../index.css');
      const cssContent = fs.readFileSync(cssPath, 'utf-8');

      expect(cssContent).toMatch(/\[class\*=['"]?bg-\[#16162A\]['"]?\][^{]*\{[^}]*background-color:\s*var\(--bg-surface\)/);
      expect(cssContent).toMatch(/\[class\*=['"]?from-indigo-950['"]?\][^{]*\{[^}]*background-image:\s*linear-gradient/);
      expect(cssContent).toMatch(/\.bg-amber-600[^{]*\{[^}]*color:\s*#0f172a/);
    });
  });
});
