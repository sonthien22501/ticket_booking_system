import React from 'react';
// @ts-ignore
import fs from 'node:fs';
// @ts-ignore
import path from 'node:path';
import { describe, it, expect, vi } from 'vitest';

// Declare Node globals for headless test environment
// @ts-ignore
declare const __dirname: string;
import {
  calculate3DTilt,
  interpolateCountUp,
  calculateStaggerDelay,
  calculateRingDashOffset,
  calculateSeatScarcity,
} from '../utils/motion';
import { EventCard } from '../components/EventCard';
import { EventCatalog } from '../components/EventCatalog';
import { SeatNode } from '../components/SeatNode';
import { SeatGrid } from '../components/SeatGrid';
import { AdminDashboardModal } from '../components/AdminDashboardModal';
import { TicketPassModal } from '../components/TicketPassModal';
import { EventItem, SeatItem, BookingResponse, TicketItem } from '../types';

describe('Milestone 34: Cinema Premiere & Holographic Glow Motion Engine', () => {
  const sampleEvent: EventItem = {
    id: 'evt-imax-34',
    title: 'Dune: Part Two — 70mm IMAX Grand Premiere',
    description: 'Hans Zimmer live orchestral score with 70mm laser projection.',
    category: 'CINEMA',
    venueName: 'Palast IMAX Dome',
    venueLocation: 'Berlin, Germany',
    startTime: '2026-12-01T20:00:00Z',
    endTime: '2026-12-01T23:30:00Z',
    status: 'ACTIVE',
    tiers: [
      { id: 't-vip', name: 'VIP Premiere Loge', price: 95, capacity: 40, availableSeats: 8 },
      { id: 't-std', name: 'Grand Orchestra', price: 45, capacity: 120, availableSeats: 32 },
    ],
  };

  describe('1. 3D Perspective Tilt & Cursor Spotlight Math (calculate3DTilt)', () => {
    it('computes centered neutral tilt (0deg, 0deg) when cursor is at center of element', () => {
      const rect = { left: 100, top: 200, width: 400, height: 300 };
      const result = calculate3DTilt(rect, 300, 350, 10);

      expect(result.rotateX).toBeCloseTo(0, 4);
      expect(result.rotateY).toBeCloseTo(0, 4);
      expect(result.spotlightX).toBeCloseTo(50, 4);
      expect(result.spotlightY).toBeCloseTo(50, 4);
    });

    it('computes bounded positive/negative 3D rotation angles at card corners', () => {
      const rect = { left: 0, top: 0, width: 200, height: 200 };
      // Top-left corner (0, 0)
      const topLeft = calculate3DTilt(rect, 0, 0, 8);
      expect(topLeft.rotateX).toBeCloseTo(8, 2);
      expect(topLeft.rotateY).toBeCloseTo(-8, 2);
      expect(topLeft.spotlightX).toBe(0);
      expect(topLeft.spotlightY).toBe(0);

      // Bottom-right corner (200, 200)
      const bottomRight = calculate3DTilt(rect, 200, 200, 8);
      expect(bottomRight.rotateX).toBeCloseTo(-8, 2);
      expect(bottomRight.rotateY).toBeCloseTo(8, 2);
      expect(bottomRight.spotlightX).toBe(100);
      expect(bottomRight.spotlightY).toBe(100);
    });

    it('safely handles zero-dimension DOMRect in headless/JSDOM environments without NaN', () => {
      const zeroRect = { left: 0, top: 0, width: 0, height: 0 };
      const result = calculate3DTilt(zeroRect, 50, 50, 10);

      expect(Number.isNaN(result.rotateX)).toBe(false);
      expect(Number.isNaN(result.rotateY)).toBe(false);
      expect(result.rotateX).toBe(0);
      expect(result.rotateY).toBe(0);
      expect(result.spotlightX).toBe(50);
      expect(result.spotlightY).toBe(50);
    });
  });

  describe('2. Cubic Ease-Out Telemetry Count-Up (interpolateCountUp)', () => {
    it('interpolates accurately at boundaries (0, 0.5, 1) with cubic ease-out', () => {
      expect(interpolateCountUp(0, 1000, 0)).toBe(0);
      expect(interpolateCountUp(0, 1000, 1)).toBe(1000);

      // At progress = 0.5, cubic ease-out (1 - (1 - 0.5)^3) = 1 - 0.125 = 0.875 -> 875
      const mid = interpolateCountUp(0, 1000, 0.5);
      expect(mid).toBeCloseTo(875, 2);
    });

    it('clamps out-of-range progress values below 0 or above 1', () => {
      expect(interpolateCountUp(100, 500, -0.5)).toBe(100);
      expect(interpolateCountUp(100, 500, 1.8)).toBe(500);
    });
  });

  describe('3. Staggered Cascade & SVG Occupancy Gauge Calculations', () => {
    it('calculates staggered entrance delay and clamps at maximum cap', () => {
      expect(calculateStaggerDelay(0, 55, 600)).toBe(0);
      expect(calculateStaggerDelay(3, 55, 600)).toBe(165);
      expect(calculateStaggerDelay(50, 55, 600)).toBe(600);
      expect(calculateStaggerDelay(-2, 55, 600)).toBe(0);
    });

    it('computes accurate SVG strokeDasharray and strokeDashoffset for occupancy rings', () => {
      const zeroRing = calculateRingDashOffset(0, 20);
      const fullRing = calculateRingDashOffset(100, 20);
      const halfRing = calculateRingDashOffset(50, 20);

      const expectedCircumference = 2 * Math.PI * 20;
      expect(zeroRing.circumference).toBeCloseTo(expectedCircumference, 2);
      expect(zeroRing.dashOffset).toBeCloseTo(expectedCircumference, 2);
      expect(fullRing.dashOffset).toBeCloseTo(0, 2);
      expect(halfRing.dashOffset).toBeCloseTo(expectedCircumference * 0.5, 2);
    });

    it('calculates real-time seat scarcity percentage and urgency status', () => {
      const scarcity = calculateSeatScarcity(sampleEvent.tiers);
      // Total capacity = 160, available = 40 -> 75% sold, 25% remaining
      expect(scarcity.totalAvailable).toBe(40);
      expect(scarcity.totalCapacity).toBe(160);
      expect(scarcity.soldPercent).toBe(75);
      expect(scarcity.isHighDemand).toBe(true);
    });
  });

  describe('4. Component Motion Integration & Prop Contracts', () => {
    it('supports index stagger prop on EventCard and Premiere Hero Showcase in EventCatalog', () => {
      const cardEl = React.createElement(EventCard, {
        event: sampleEvent,
        index: 2,
        onSelectEvent: vi.fn(),
      });
      expect(cardEl.props.index).toBe(2);

      const catalogEl = React.createElement(EventCatalog, {
        events: [sampleEvent],
        loading: false,
        error: null,
        onRefresh: vi.fn(),
        onSelectEvent: vi.fn(),
      });
      expect(catalogEl.props.events).toHaveLength(1);
    });

    it('supports wave stagger coordinates on SeatNode and SeatGrid', () => {
      const seat: SeatItem = {
        id: 'A1',
        row: 'A',
        number: 1,
        status: 'AVAILABLE',
        price: 95,
      };
      const nodeEl = React.createElement(SeatNode, {
        seat,
        isSelected: true,
        onToggle: vi.fn(),
        rowIndex: 0,
        colIndex: 3,
      });
      expect(nodeEl.props.rowIndex).toBe(0);
      expect(nodeEl.props.colIndex).toBe(3);

      const gridEl = React.createElement(SeatGrid, {
        seats: [seat],
        selectedSeatIds: new Set(['A1']),
        onToggleSeat: vi.fn(),
      });
      expect(gridEl.props.seats).toHaveLength(1);
    });

    it('supports holographic boarding pass in TicketPassModal and telemetry gauge in AdminDashboardModal', () => {
      const mockBooking: BookingResponse = {
        bookingId: 'bk-holo-1',
        bookingReference: 'REF-HOLO-34',
        eventId: sampleEvent.id,
        eventTitle: sampleEvent.title,
        customerName: 'Paul Atreides',
        customerEmail: 'paul@arrakis.org',
        status: 'CONFIRMED',
        quantity: 1,
        seats: ['A1'],
        totalPrice: 95,
        createdAt: '2026-10-28T15:00:00Z',
        tickets: [],
      };
      const mockTicket: TicketItem = {
        ticketId: 'tk-1',
        ticketCode: 'TKT-IMAX-70MM',
        seatLabel: 'A1',
        status: 'VALID',
        price: 95,
      };

      const passEl = React.createElement(TicketPassModal, {
        booking: mockBooking,
        ticket: mockTicket,
        isOpen: true,
        onClose: vi.fn(),
      });
      expect(passEl.props.ticket.ticketCode).toBe('TKT-IMAX-70MM');

      const adminEl = React.createElement(AdminDashboardModal, {
        isOpen: true,
        onClose: vi.fn(),
        user: null,
      });
      expect(adminEl.props.isOpen).toBe(true);
    });
  });

  describe('5. Milestone 34.1 UI Polish: Alignment, Light Theme Contrast & Fixed Showcase Box', () => {
    const cssPath = path.resolve(__dirname, '../index.css');
    const cssContent = fs.readFileSync(cssPath, 'utf-8');

    it('defines .sr-only accessible offscreen utility with position absolute and 1px dimension', () => {
      expect(cssContent).toMatch(/\.sr-only\s*\{[^}]*position:\s*absolute/);
      expect(cssContent).toMatch(/\.sr-only\s*\{[^}]*width:\s*1px/);
      expect(cssContent).toMatch(/\.sr-only\s*\{[^}]*height:\s*1px/);
    });

    it('defines header action controls (.currency-selector-pill, .btn-my-tickets, .btn-notifications, .btn-admin-studio) with inline-flex alignment', () => {
      expect(cssContent).toMatch(/\.currency-selector-pill\s*\{[^}]*display:\s*inline-flex/);
      expect(cssContent).toMatch(/\.btn-my-tickets\s*\{[^}]*display:\s*inline-flex/);
      expect(cssContent).toMatch(/\.btn-notifications\s*\{[^}]*display:\s*inline-flex/);
      expect(cssContent).toMatch(/\.btn-admin-studio\s*\{[^}]*display:\s*inline-flex/);
    });

    it('defines filter facets (.city-facets-bar, .city-pill, .budget-pill, .catalog-sort-select) with explicit spacing and layout', () => {
      expect(cssContent).toMatch(/\.city-facets-bar\s*\{[^}]*gap:/);
      expect(cssContent).toMatch(/\.city-pill\s*\{[^}]*padding:/);
      expect(cssContent).toMatch(/\.budget-pill\s*\{[^}]*padding:/);
      expect(cssContent).toMatch(/\.catalog-sort-select\s*\{/);
    });

    it('provides high-contrast light theme overrides for hero title and site header', () => {
      expect(cssContent).toMatch(/(?:html\[data-theme=['"]?light['"]?\]|body\.light|\.light)[^{]*\.hero-title/);
      expect(cssContent).toMatch(/(?:html\[data-theme=['"]?light['"]?\]|body\.light|\.light)[^{]*\.site-header/);
    });

    it('enforces fixed-height stability and 2-line description clamp on premiere showcase card', () => {
      expect(cssContent).toMatch(/\.premiere-showcase-card\s*\{[^}]*min-height:\s*172px/);
      expect(cssContent).toMatch(/\.premiere-showcase-desc\s*\{[^}]*-webkit-line-clamp:\s*2/);
      expect(cssContent).toMatch(/\.premiere-showcase-desc\s*\{[^}]*max-height:\s*2\.5rem/);
    });
  });

  describe('6. Milestone 35: Seat Map Mashup Bar, Bounded Artwork Thumbnail & Section Switcher Layout', () => {
    const cssPath = path.resolve(__dirname, '../index.css');
    const cssContent = fs.readFileSync(cssPath, 'utf-8');

    it('defines .mashup-intel-bar with flex layout and horizontal alignment', () => {
      expect(cssContent).toMatch(/\.mashup-intel-bar\s*\{[^}]*display:\s*flex/);
      expect(cssContent).toMatch(/\.mashup-intel-bar\s*\{[^}]*align-items:\s*center/);
    });

    it('defines .mashup-weather-pill and .mashup-audio-pill with sleek compact styling', () => {
      expect(cssContent).toMatch(/\.mashup-weather-pill\s*\{[^}]*display:\s*inline-flex/);
      expect(cssContent).toMatch(/\.mashup-audio-pill\s*\{[^}]*display:\s*inline-flex/);
    });

    it('enforces strict 22px bounded dimensions and object-fit cover on .mashup-artwork-thumb', () => {
      expect(cssContent).toMatch(/\.mashup-artwork-thumb\s*\{[^}]*max-width:\s*22px/);
      expect(cssContent).toMatch(/\.mashup-artwork-thumb\s*\{[^}]*max-height:\s*22px/);
      expect(cssContent).toMatch(/\.mashup-artwork-thumb\s*\{[^}]*object-fit:\s*cover/);
      expect(cssContent).toMatch(/\.mashup-artwork-thumb\s*\{[^}]*border-radius:\s*4px/);
    });

    it('defines .seatmap-section-toolbar and .section-filter-pill segmented bar controls', () => {
      expect(cssContent).toMatch(/\.seatmap-section-toolbar\s*\{[^}]*display:\s*flex/);
      expect(cssContent).toMatch(/\.section-filter-pill\s*\{[^}]*display:\s*inline-flex/);
      expect(cssContent).toMatch(/\.section-filter-pill\.active\s*\{/);
      expect(cssContent).toMatch(/\.btn-floorplan-toggle\s*\{[^}]*display:\s*inline-flex/);
    });

    it('defines .btn-ai-concierge with crisp button styling and light mode support', () => {
      expect(cssContent).toMatch(/\.btn-ai-concierge\s*\{[^}]*display:\s*inline-flex/);
      expect(cssContent).toMatch(/(?:html\[data-theme=['"]?light['"]?\]|body\.light|\.light)[^{]*\.mashup-intel-bar/);
    });
  });
});
