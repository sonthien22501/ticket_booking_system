import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  validateVoucher,
  fetchAdminVouchers,
  createAdminVoucher,
  createBooking,
} from '../services/api';

describe('Promotional Voucher & Coupon Engine (Milestone 25)', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('validateVoucher API', () => {
    it('successfully validates percentage voucher and calculates discounted total', async () => {
      const mockResponse = {
        valid: true,
        code: 'EARLYBIRD20',
        discountType: 'PERCENTAGE',
        discountValue: 20,
        discountAmount: 30,
        finalTotal: 120,
        message: "Voucher 'EARLYBIRD20' applied successfully! Saved $30.00",
        minSpend: 50,
      };

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      } as Response);

      const res = await validateVoucher('EARLYBIRD20', 150, 'evt-101');
      expect(res.valid).toBe(true);
      expect(res.code).toBe('EARLYBIRD20');
      expect(res.discountAmount).toBe(30);
      expect(res.finalTotal).toBe(120);
    });

    it('rejects voucher when minimum spend requirement is not met', async () => {
      const mockResponse = {
        valid: false,
        code: 'EARLYBIRD20',
        message: "minimum spend of $50.00 required for voucher 'EARLYBIRD20'",
      };

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: async () => mockResponse,
      } as Response);

      const res = await validateVoucher('EARLYBIRD20', 30, 'evt-101');
      expect(res.valid).toBe(false);
      expect(res.message).toContain('minimum spend');
    });

    it('falls back to local client evaluation when network fails for demo convenience', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'));

      const res = await validateVoucher('SAVE10', 50, 'evt-101');
      expect(res.valid).toBe(true);
      expect(res.discountAmount).toBe(10);
      expect(res.finalTotal).toBe(40);
    });

    it('rejects unknown voucher codes in fallback mode', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'));

      const res = await validateVoucher('UNKNOWN_CODE_XYZ', 100);
      expect(res.valid).toBe(false);
      expect(res.message).toContain('not found');
    });
  });

  describe('createBooking with voucher integration', () => {
    it('transmits voucherCode in booking creation payload', async () => {
      let capturedBody: Record<string, unknown> | null = null;
      vi.spyOn(globalThis, 'fetch').mockImplementationOnce(async (_url: unknown, options?: RequestInit) => {
        capturedBody = JSON.parse((options?.body as string) || '{}');
        return {
          ok: true,
          json: async () => ({
            id: 'bk-12345',
            bookingReference: 'REF-EVT101-12345',
            status: 'CONFIRMED',
            totalAmount: 120,
            voucherCode: 'EARLYBIRD20',
            discountAmount: 30,
            quantity: 1,
            seats: ['A1'],
            tickets: [],
          }),
        } as Response;
      });

      const booking = await createBooking({
        eventId: 'evt-101',
        customerName: 'Jane Doe',
        customerEmail: 'jane@example.com',
        seats: ['A1'],
        voucherCode: 'EARLYBIRD20',
      });

      expect(capturedBody).not.toBeNull();
      expect(capturedBody?.['voucherCode']).toBe('EARLYBIRD20');
      expect(capturedBody?.['voucher_code']).toBe('EARLYBIRD20');
      expect(booking.voucherCode).toBe('EARLYBIRD20');
    });
  });

  describe('Organizer Studio Voucher Management', () => {
    it('fetches list of promotional vouchers', async () => {
      const mockVouchers = [
        {
          id: 'vch-1',
          code: 'EARLYBIRD20',
          discountType: 'PERCENTAGE',
          discountValue: 20,
          minSpend: 50,
          maxUses: 200,
          currentUses: 14,
          active: true,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ];

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockVouchers,
      } as Response);

      const list = await fetchAdminVouchers();
      expect(list.length).toBeGreaterThan(0);
      expect(list[0].code).toBe('EARLYBIRD20');
    });

    it('creates a new campaign voucher code', async () => {
      const newVoucher = {
        code: 'FESTIVAL30',
        discountType: 'PERCENTAGE' as const,
        discountValue: 30,
        minSpend: 80,
        maxUses: 100,
        expiresInDays: 30,
      };

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: 'vch-new',
          ...newVoucher,
          currentUses: 0,
          active: true,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        }),
      } as Response);

      const created = await createAdminVoucher(newVoucher);
      expect(created.code).toBe('FESTIVAL30');
      expect(created.discountValue).toBe(30);
    });
  });
});
