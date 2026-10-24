import { describe, it, expect, beforeEach, vi } from 'vitest';
import { fetchNotificationDeliveries, dispatchTestNotification } from '../services/api';
import { NotificationDeliveryLog, DispatchNotificationRequest } from '../types';

describe('Asynchronous Multi-Channel Notification Microservice (Milestone 32)', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('fetchNotificationDeliveries', () => {
    it('successfully retrieves delivery audit logs from notification microservice', async () => {
      const mockDeliveries: NotificationDeliveryLog[] = [
        {
          id: 'del-001',
          channel: 'EMAIL',
          recipient: 'alex@example.com',
          eventType: 'booking.confirmed',
          subject: 'Booking Confirmed: Neon Symphony',
          body: '<h2>Confirmed</h2>',
          status: 'DELIVERED',
          attempts: 1,
          createdAt: '2026-10-24T00:00:00Z',
        },
        {
          id: 'del-002',
          channel: 'SMS',
          recipient: '+15551234567',
          eventType: 'waitlist.promoted',
          subject: 'Transactional SMS',
          body: 'TicketVerse: Hold available!',
          status: 'DELIVERED',
          attempts: 1,
          createdAt: '2026-10-24T00:30:00Z',
        },
      ];

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockDeliveries,
      } as Response);

      const result = await fetchNotificationDeliveries();
      expect(result).toHaveLength(2);
      expect(result[0].channel).toBe('EMAIL');
      expect(result[1].channel).toBe('SMS');
      expect(result[0].status).toBe('DELIVERED');
    });

    it('falls back to seed delivery logs when service is temporarily unavailable', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network offline'));

      const result = await fetchNotificationDeliveries();
      expect(result.length).toBeGreaterThanOrEqual(2);
      expect(result.some((d) => d.channel === 'EMAIL')).toBe(true);
      expect(result.some((d) => d.channel === 'SMS')).toBe(true);
    });
  });

  describe('dispatchTestNotification', () => {
    it('dispatches test email payload and returns audited delivery log', async () => {
      const req: DispatchNotificationRequest = {
        eventType: 'booking.confirmed',
        recipient: 'test@example.com',
        customerName: 'Test User',
        eventTitle: 'Neon Symphony',
        bookingRef: 'REF-1234',
        channel: 'EMAIL',
      };

      const mockRes = {
        success: true,
        delivery: {
          id: 'del-mock-123',
          channel: 'EMAIL',
          recipient: 'test@example.com',
          eventType: 'booking.confirmed',
          subject: 'Booking Confirmed: Neon Symphony',
          body: '<p>Confirmed</p>',
          status: 'DELIVERED',
          attempts: 1,
          createdAt: new Date().toISOString(),
        },
      };

      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockRes,
      } as Response);

      const res = await dispatchTestNotification(req);
      expect(res.success).toBe(true);
      expect(res.delivery.id).toBe('del-mock-123');
      expect(res.delivery.channel).toBe('EMAIL');

      expect(fetchSpy).toHaveBeenCalledWith(
        expect.stringContaining('/notifications/dispatch-test'),
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(req),
        })
      );
    });

    it('throws error when dispatch endpoint returns non-200 status', async () => {
      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: false,
        status: 500,
        json: async () => ({ error: 'RabbitMQ connection unavailable' }),
      } as Response);

      await expect(
        dispatchTestNotification({
          eventType: 'booking.confirmed',
          recipient: 'fail@example.com',
        })
      ).rejects.toThrow('RabbitMQ connection unavailable');
    });
  });
});
