import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  fetchAdminAnalytics,
  createAdminEvent,
  updateEventStatus,
} from '../services/api';
import { AdminDashboardModal } from '../components/AdminDashboardModal';
import { AdminAnalyticsData, CreateEventPayload, User } from '../types';

describe('Milestone 24: Admin & Organizer Studio & Live Sales Telemetry', () => {
  const originalFetch = (globalThis as unknown as { fetch?: typeof fetch }).fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    (globalThis as unknown as { fetch?: typeof fetch }).fetch = originalFetch;
  });

  const mockAdminUser: User = {
    id: 'usr-admin-1',
    name: 'Arena Manager',
    email: 'admin@ticketverse.com',
    role: 'admin',
  };

  const mockAnalytics: AdminAnalyticsData = {
    totalRevenue: 24500.5,
    totalBookings: 64,
    confirmedBookings: 58,
    cancelledBookings: 6,
    totalTicketsSold: 120,
    activeEventsCount: 4,
    averageOccupancy: 68.4,
    recentBookings: [
      {
        id: 'bk-adm-1',
        bookingReference: 'REF-ADM-99',
        customerName: 'Marcus Aurelius',
        customerEmail: 'marcus@rome.org',
        eventTitle: 'Neon Symphony 2026',
        totalAmount: 300,
        status: 'CONFIRMED',
        quantity: 2,
        createdAt: '2026-10-18T18:30:00Z',
      },
    ],
    eventsSummary: [
      {
        eventId: 'evt-101',
        title: 'Neon Symphony 2026',
        category: 'Music & Concerts',
        venueName: 'Grand Arena Berlin',
        status: 'ACTIVE',
        totalSeats: 150,
        bookedSeats: 110,
        occupancyPercent: 73.33,
        grossRevenue: 16500,
      },
    ],
  };

  describe('API Service Functions', () => {
    it('fetchAdminAnalytics calls /api/admin/analytics with Bearer token if present', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockAnalytics,
      });
      (globalThis as unknown as { fetch?: typeof fetch }).fetch = mockFetch as unknown as typeof fetch;

      const data = await fetchAdminAnalytics();
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/admin/analytics'),
        expect.objectContaining({
          headers: expect.objectContaining({ Accept: 'application/json' }),
        })
      );
      expect(data.totalRevenue).toBe(24500.5);
      expect(data.confirmedBookings).toBe(58);
      expect(data.recentBookings.length).toBe(1);
    });

    it('createAdminEvent posts event payload to /api/events', async () => {
      const payload: CreateEventPayload = {
        title: 'Cyberpunk Rave',
        description: 'Underground cyber synth live experience.',
        category: 'Music',
        venueName: 'Substation X',
        venueLocation: 'Neo-Tokyo',
        startTime: '2026-11-20T21:00:00Z',
        endTime: '2026-11-21T03:00:00Z',
        tiers: [{ name: 'VIP Pass', price: 120, capacity: 60 }],
      };

      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ id: 'evt-cyber', status: 'CREATED' }),
      });
      (globalThis as unknown as { fetch?: typeof fetch }).fetch = mockFetch as unknown as typeof fetch;

      const res = await createAdminEvent(payload);
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/events'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(payload),
        })
      );
      expect(res.status).toBe('CREATED');
    });

    it('updateEventStatus sends PUT to /api/events/:id/status', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ eventId: 'evt-101', status: 'PAUSED', updated: true }),
      });
      (globalThis as unknown as { fetch?: typeof fetch }).fetch = mockFetch as unknown as typeof fetch;

      await updateEventStatus('evt-101', 'PAUSED');
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/events/evt-101/status'),
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ status: 'PAUSED' }),
        })
      );
    });
  });

  describe('AdminDashboardModal Component', () => {
    it('instantiates and defines AdminDashboardModal with valid props', () => {
      expect(typeof AdminDashboardModal).toBe('function');
      const element = React.createElement(AdminDashboardModal, {
        isOpen: true,
        onClose: vi.fn(),
        user: mockAdminUser,
        onEventCreated: vi.fn(),
      });
      expect(element).toBeDefined();
      expect(element.props.isOpen).toBe(true);
    });
  });
});
