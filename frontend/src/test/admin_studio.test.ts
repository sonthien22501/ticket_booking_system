import React from 'react';
import ReactDOMServer from 'react-dom/server';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  fetchAdminAnalytics,
  createAdminEvent,
  updateEventStatus,
} from '../services/api';
import { Header } from '../components/Header';
import { AuthModal } from '../components/AuthModal';
import { AdminDashboardModal } from '../components/AdminDashboardModal';
import { AdminAnalyticsData, CreateEventPayload, User, EventItem } from '../types';

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

  describe('Milestone 39: 4-Role RBAC Workspace, Customer VIP Hub, Organizer Studio & System Admin Command Center', () => {
    const sampleEvents: EventItem[] = [
      {
        id: 'evt-101',
        title: 'Neon Symphony 2026',
        description: 'Futuristic orchestral performance.',
        category: 'Music & Concerts',
        venueName: 'Grand Arena Berlin',
        venueLocation: 'Berlin, Germany',
        startTime: '2026-11-20T20:00:00Z',
        endTime: '2026-11-20T23:00:00Z',
        status: 'ACTIVE',
        tiers: [{ id: 't1', name: 'VIP Loge', price: 120, capacity: 50 }],
      },
    ];

    const guestUser = null;
    const customerUser: User = {
      id: 'usr-customer-1',
      name: 'Jane Doe',
      email: 'user@tickets.com',
      role: 'customer',
    };
    const organizerUser: User = {
      id: 'usr-organizer-1',
      name: 'Event Host',
      email: 'organizer@tickets.com',
      role: 'organizer',
    };
    const adminUser: User = {
      id: 'usr-admin-1',
      name: 'Sys Admin',
      email: 'admin@tickets.com',
      role: 'admin',
    };

    describe('1. Role-Specific Header Gating (Header.tsx)', () => {
      it('Guest (currentUser: null) hides admin-studio-btn, role-workspace-btn, and my-tickets-btn', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(Header, {
            currentUser: guestUser,
            onOpenAdmin: vi.fn(),
            onOpenBookings: vi.fn(),
            onOpenAuth: vi.fn(),
          })
        );
        expect(html).toContain('data-testid="signin-btn"');
        expect(html).not.toContain('data-testid="admin-studio-btn"');
        expect(html).not.toContain('data-testid="role-workspace-btn"');
        expect(html).not.toContain('data-testid="my-tickets-btn"');
      });

      it('Customer (role: customer) renders VIP Hub (role-workspace-btn) and My Tickets', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(Header, {
            currentUser: customerUser,
            onOpenAdmin: vi.fn(),
            onOpenBookings: vi.fn(),
          })
        );
        expect(html).toContain('data-testid="role-workspace-btn"');
        expect(html).toContain('VIP Hub');
        expect(html).toContain('data-testid="my-tickets-btn"');
        expect(html).not.toContain('data-testid="admin-studio-btn"');
      });

      it('Organizer (role: organizer) renders Organizer Studio (admin-studio-btn) and My Tickets', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(Header, {
            currentUser: organizerUser,
            onOpenAdmin: vi.fn(),
            onOpenBookings: vi.fn(),
          })
        );
        expect(html).toContain('data-testid="admin-studio-btn"');
        expect(html).toContain('Organizer Studio');
        expect(html).toContain('data-testid="my-tickets-btn"');
        expect(html).not.toContain('data-testid="role-workspace-btn"');
      });

      it('Admin (role: admin) renders Admin Command (admin-studio-btn) and My Tickets', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(Header, {
            currentUser: adminUser,
            onOpenAdmin: vi.fn(),
            onOpenBookings: vi.fn(),
          })
        );
        expect(html).toContain('data-testid="admin-studio-btn"');
        expect(html).toContain('Admin Command');
        expect(html).toContain('data-testid="my-tickets-btn"');
        expect(html).not.toContain('data-testid="role-workspace-btn"');
      });
    });

    describe('2. 1-Click Demo Role Switcher (AuthModal.tsx)', () => {
      it('renders customer, organizer, and admin demo quick-login buttons and 3 role options', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(AuthModal, {
            isOpen: true,
            onClose: vi.fn(),
            onAuthSuccess: vi.fn(),
            initialTab: 'register',
          })
        );
        expect(html).toContain('data-testid="demo-login-customer"');
        expect(html).toContain('data-testid="demo-login-organizer"');
        expect(html).toContain('data-testid="demo-login-admin"');
        expect(html).toContain('value="customer"');
        expect(html).toContain('value="organizer"');
        expect(html).toContain('value="admin"');
      });
    });

    describe('3. Full-Page Role Workspace & Customer VIP Hub (AdminDashboardModal.tsx)', () => {
      it('supports inlinePage full-page workspace layout with back to catalog button', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(AdminDashboardModal, {
            isOpen: true,
            inlinePage: true,
            onClose: vi.fn(),
            user: organizerUser,
            events: sampleEvents,
          })
        );
        expect(html).toContain('role-workspace-container');
        expect(html).toContain('data-testid="back-to-catalog-btn"');
        expect(html).toContain('Back to Live Arena Catalog');
      });

      it('renders Customer VIP Hub when role is customer (Loyalty status, countdown, breakdown)', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(AdminDashboardModal, {
            isOpen: true,
            inlinePage: true,
            onClose: vi.fn(),
            user: customerUser,
            events: sampleEvents,
          })
        );
        expect(html).toContain('VIP Hub');
        expect(html).toContain('Loyalty');
        expect(html).toContain('Upcoming Next Event');
        expect(html).toContain('Spending');
        expect(html).toContain('Attendance by Category');
        expect(html).not.toContain('data-testid="tab-system-health"');
      });

      it('renders Category Breakdown and Occupancy Heat Bars for organizer', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(AdminDashboardModal, {
            isOpen: true,
            inlinePage: true,
            onClose: vi.fn(),
            user: organizerUser,
            events: sampleEvents,
          })
        );
        expect(html).toContain('Organizer Studio');
        expect(html).toContain('data-testid="category-breakdown-panel"');
        expect(html).toContain('data-testid="occupancy-heat-bar"');
        expect(html).not.toContain('data-testid="tab-system-health"');
      });

      it('renders Platform Health & RBAC Governance tab exclusively for admin', () => {
        const html = ReactDOMServer.renderToStaticMarkup(
          React.createElement(AdminDashboardModal, {
            isOpen: true,
            inlinePage: true,
            onClose: vi.fn(),
            user: adminUser,
            events: sampleEvents,
          })
        );
        expect(html).toContain('data-testid="tab-system-health"');
        expect(html).toContain('data-testid="microservice-mesh-matrix"');
        expect(html).toContain('data-testid="rbac-governance-matrix"');
        expect(html).toContain('API Gateway');
        expect(html).toContain('Notification Worker');
      });
    });
  });
});
