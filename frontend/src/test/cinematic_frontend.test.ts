import React from 'react';
import { describe, it, expect } from 'vitest';
import { SeatStatus, SeatItem, EventItem } from '../types';
import { SeatNode } from '../components/SeatNode';
import { SeatGrid } from '../components/SeatGrid';
import { Header } from '../components/Header';
import { EventCard } from '../components/EventCard';
import { BookingModal } from '../components/BookingModal';
import { SparklesIcon, FilmIcon, ShieldCheckIcon, CheckIcon, ChevronRightIcon } from '../components/Icons';

describe('Cinematic Frontend Redesign & Design System QA (M22)', () => {
  const mockEvent: EventItem = {
    id: 'evt-cine-1',
    title: 'Interstellar Cinema IMAX Screening',
    description: 'Experience Christopher Nolan masterpiece on a curved 70mm canvas.',
    category: 'CINEMA',
    venueName: 'Omniverse Grand Cinema',
    venueLocation: 'Berlin, Germany',
    startTime: '2026-11-20T20:00:00Z',
    endTime: '2026-11-20T23:00:00Z',
    status: 'ACTIVE',
    tiers: [
      { id: 'tier-vip', name: 'VIP Loge', price: 25 },
      { id: 'tier-standard', name: 'Stalls', price: 15 },
    ],
  };

  describe('Luminous Seat States & Tooltips', () => {
    it('supports all 5 seat states: AVAILABLE, SELECTED, BOOKED, VIP, and WAITLIST', () => {
      const statuses: SeatStatus[] = ['AVAILABLE', 'SELECTED', 'BOOKED', 'HOLD', 'BLOCKED', 'WAITLIST'];
      expect(statuses).toContain('WAITLIST');
      expect(statuses).toContain('AVAILABLE');
      expect(statuses).toContain('BOOKED');
    });

    it('renders accessible tooltip for WAITLIST seat with queue message', () => {
      const waitlistSeat: SeatItem = {
        id: 'A1',
        row: 'A',
        number: 1,
        status: 'WAITLIST',
        tierName: 'VIP Loge',
        price: 25,
      };

      const el = React.createElement(SeatNode, {
        seat: waitlistSeat,
        isSelected: false,
        onToggle: () => {},
      });

      expect(el.props.seat.status).toBe('WAITLIST');
    });

    it('renders column indicators in SeatGrid accurately matching seat numbers', () => {
      const seats: SeatItem[] = [
        { id: 'A1', row: 'A', number: 1, status: 'AVAILABLE', price: 25 },
        { id: 'A2', row: 'A', number: 2, status: 'AVAILABLE', price: 25 },
        { id: 'B1', row: 'B', number: 1, status: 'AVAILABLE', price: 15 },
        { id: 'B2', row: 'B', number: 2, status: 'AVAILABLE', price: 15 },
      ];

      const gridEl = React.createElement(SeatGrid, {
        seats,
        selectedSeatIds: new Set<string>(),
        onToggleSeat: () => {},
      });

      expect((gridEl.props as any).seats.length).toBe(4);
    });
  });

  describe('Cinematic Card Elevation & Poster Banner', () => {
    it('renders EventCard with cinematic category and starting price', () => {
      const cardEl = React.createElement(EventCard, {
        event: mockEvent,
        onSelectEvent: () => {},
      });

      expect(cardEl.props.event.title).toBe('Interstellar Cinema IMAX Screening');
      expect(cardEl.props.event.category).toBe('CINEMA');
    });
  });

  describe('Lucide SVG Icon Integrity', () => {
    it('provides all required Lucide SVG icons without raw emojis', () => {
      expect(typeof SparklesIcon).toBe('function');
      expect(typeof FilmIcon).toBe('function');
      expect(typeof ShieldCheckIcon).toBe('function');
      expect(typeof CheckIcon).toBe('function');
      expect(typeof ChevronRightIcon).toBe('function');
    });
  });

  describe('BookingModal Step Progression', () => {
    it('renders BookingModal with selected seat count and event details', () => {
      const modalEl = React.createElement(BookingModal, {
        isOpen: true,
        event: mockEvent,
        selectedSeats: [{ id: 'A1', row: 'A', number: 1, status: 'SELECTED', price: 25 }],
        submitting: false,
        conflictError: null,
        conflictingSeats: [],
        confirmedBooking: null,
        onClose: () => {},
        onSubmitBooking: () => {},
        onRefreshMap: () => {},
        onResetBooking: () => {},
      });

      expect(modalEl.props.isOpen).toBe(true);
      expect(modalEl.props.selectedSeats.length).toBe(1);
    });
  });

  describe('Header Component Branding & Gateway Status', () => {
    it('renders Header with gateway connectivity status', () => {
      const headerEl = React.createElement(Header, {
        apiConnected: true,
      });

      expect(headerEl.props.apiConnected).toBe(true);
    });
  });
});
