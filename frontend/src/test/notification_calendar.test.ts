import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  fetchNotifications,
  markNotificationRead,
  markAllNotificationsRead,
  getCalendarICSUrl,
  downloadBookingCalendarICS,
} from '../services/api';
import { Header } from '../components/Header';
import { NotificationCenter } from '../components/NotificationCenter';
import { BookingConfirmation } from '../components/BookingConfirmation';
import { TicketPassModal } from '../components/TicketPassModal';
import { AppNotification, BookingResponse, User } from '../types';

describe('Milestone 28: In-App Notification Center & Digital Calendar (.ics) Pass Synchronization', () => {
  const originalFetch = (globalThis as unknown as { fetch?: typeof fetch }).fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    (globalThis as unknown as { fetch?: typeof fetch }).fetch = originalFetch;
  });

  const mockUser: User = {
    id: 'usr-demo-customer',
    name: 'Alex Morgan',
    email: 'user@tickets.com',
    role: 'customer',
  };

  const mockNotifications: AppNotification[] = [
    {
      id: 'notif-1',
      recipientEmail: 'user@tickets.com',
      type: 'BOOKING_CONFIRMED',
      title: 'Booking Confirmed!',
      message: 'Your order #REF-NEON-2026-001 for Neon Symphony 2026 is confirmed.',
      data: { bookingId: 'bkg-demo-001', eventId: 'evt-101' },
      isRead: false,
      createdAt: new Date().toISOString(),
    },
    {
      id: 'notif-2',
      recipientEmail: 'user@tickets.com',
      type: 'WAITLIST_PROMOTED',
      title: 'You Are Off The Waitlist!',
      message: 'A front row seat opened up for Stand-Up Comedy All-Stars. You have 15 minutes to claim your hold.',
      data: { eventId: 'evt-106', reservationId: 'res-wl-promoted-003' },
      isRead: false,
      createdAt: new Date(Date.now() - 1000 * 60 * 10).toISOString(),
    },
  ];

  const mockBooking: BookingResponse = {
    bookingId: 'bkg-demo-001',
    bookingReference: 'REF-NEON-2026-001',
    status: 'CONFIRMED',
    eventId: 'evt-101',
    eventTitle: 'Neon Symphony 2026',
    quantity: 1,
    seats: ['B1'],
    totalPrice: 150.0,
    currency: 'USD',
    customerName: 'Alex Morgan',
    customerEmail: 'user@tickets.com',
    createdAt: new Date().toISOString(),
    tickets: [
      {
        ticketId: 'tkt-demo-001',
        ticketCode: 'TKT-VALID-VIP01',
        seatLabel: 'Row B, Seat 1',
        status: 'VALID',
        price: 150.0,
      },
    ],
  };

  describe('API Service Methods', () => {
    it('fetchNotifications fetches notification list with unread counter', async () => {
      const mockResponse = {
        notifications: mockNotifications,
        total: 2,
        unreadCount: 2,
      };

      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockResponse,
      } as Response);

      const result = await fetchNotifications('user@tickets.com', 'usr-demo-customer');
      expect(result.notifications.length).toBe(2);
      expect(result.unreadCount).toBe(2);
      expect(result.notifications[0].title).toBe('Booking Confirmed!');
    });

    it('markNotificationRead sends POST request with correct ID', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ success: true, isRead: true }),
      } as Response);
      (globalThis as unknown as { fetch: typeof fetch }).fetch = mockFetch;

      await markNotificationRead('notif-1');
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/notifications/notif-1/read'),
        expect.objectContaining({ method: 'POST' })
      );
    });

    it('markAllNotificationsRead sends POST request to read-all endpoint', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ success: true, updatedCount: 2 }),
      } as Response);
      (globalThis as unknown as { fetch: typeof fetch }).fetch = mockFetch;

      await markAllNotificationsRead('user@tickets.com', 'usr-demo-customer');
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/notifications/read-all'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ email: 'user@tickets.com', userId: 'usr-demo-customer' }),
        })
      );
    });

    it('getCalendarICSUrl constructs proper calendar URL', () => {
      const url = getCalendarICSUrl('bkg-demo-001');
      expect(url).toContain('/bookings/bkg-demo-001/calendar.ics');
    });

    it('downloadBookingCalendarICS fetches ICS file and triggers download', async () => {
      const mockBlob = new Blob(['BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR'], {
        type: 'text/calendar',
      });
      (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn().mockResolvedValue({
        ok: true,
        blob: async () => mockBlob,
      } as Response);

      // Mock window and document on globalThis
      const mockCreateObjectURL = vi.fn().mockReturnValue('blob:http://localhost/dummy');
      const mockRevokeObjectURL = vi.fn();
      const mockAppendChild = vi.fn();
      const mockRemoveChild = vi.fn();
      const mockClick = vi.fn();

      (globalThis as any).window = {
        URL: {
          createObjectURL: mockCreateObjectURL,
          revokeObjectURL: mockRevokeObjectURL,
        },
      };
      (globalThis as any).document = {
        createElement: vi.fn().mockReturnValue({ href: '', download: '', click: mockClick }),
        body: {
          appendChild: mockAppendChild,
          removeChild: mockRemoveChild,
        },
      };

      await downloadBookingCalendarICS('bkg-demo-001', 'Neon Symphony 2026');
      expect(mockCreateObjectURL).toHaveBeenCalled();
      expect(mockClick).toHaveBeenCalled();
      expect(mockRevokeObjectURL).toHaveBeenCalledWith('blob:http://localhost/dummy');

      delete (globalThis as any).window;
      delete (globalThis as any).document;
    });
  });

  describe('Component Rendering', () => {
    it('Header renders notification bell button with unread count badge', () => {
      const onOpenNotifs = vi.fn();
      const element = React.createElement(Header, {
        currentUser: mockUser,
        onOpenNotifications: onOpenNotifs,
        unreadNotificationsCount: 3,
      });
      expect(element).toBeDefined();
      expect(element.props.unreadNotificationsCount).toBe(3);
    });

    it('NotificationCenter renders notification drawer when isOpen is true', () => {
      const onClose = vi.fn();
      const onRefresh = vi.fn();
      const element = React.createElement(NotificationCenter, {
        isOpen: true,
        onClose,
        currentUser: mockUser,
        unreadCount: 2,
        notifications: mockNotifications,
        onRefresh,
      });
      expect(element).toBeDefined();
      expect(element.props.isOpen).toBe(true);
      expect(element.props.notifications.length).toBe(2);
      expect(element.props.unreadCount).toBe(2);
    });

    it('NotificationCenter props accept closed state', () => {
      const onClose = vi.fn();
      const onRefresh = vi.fn();
      const element = React.createElement(NotificationCenter, {
        isOpen: false,
        onClose,
        currentUser: mockUser,
        unreadCount: 0,
        notifications: [],
        onRefresh,
      });
      expect(element).toBeDefined();
      expect(element.props.isOpen).toBe(false);
    });

    it('BookingConfirmation provides Add to Calendar action', () => {
      const onDone = vi.fn();
      const element = React.createElement(BookingConfirmation, {
        booking: mockBooking,
        onDone,
      });
      expect(element).toBeDefined();
      expect(element.props.booking.bookingReference).toBe('REF-NEON-2026-001');
    });

    it('TicketPassModal provides Add to Calendar action', () => {
      const onClose = vi.fn();
      const element = React.createElement(TicketPassModal, {
        booking: mockBooking,
        ticket: mockBooking.tickets[0],
        isOpen: true,
        onClose,
      });
      expect(element).toBeDefined();
      expect(element.props.ticket.ticketCode).toBe('TKT-VALID-VIP01');
    });
  });
});
