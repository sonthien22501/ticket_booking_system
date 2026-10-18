import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  formatCurrency,
  calculateTax,
  getStoredCurrency,
  setStoredCurrency,
  SUPPORTED_CURRENCIES,
} from '../services/currency';
import {
  getInvoice,
  getInvoicePdfUrl,
  createBooking,
} from '../services/api';

describe('Multi-Currency Checkout & PDF Invoicing (Milestone 26)', () => {
  let mockStorage: Record<string, string> = {};

  beforeEach(() => {
    mockStorage = {};
    const localStorageMock = {
      getItem: vi.fn((key: string) => mockStorage[key] ?? null),
      setItem: vi.fn((key: string, value: string) => {
        mockStorage[key] = value;
      }),
      removeItem: vi.fn((key: string) => {
        delete mockStorage[key];
      }),
      clear: vi.fn(() => {
        mockStorage = {};
      }),
    };
    vi.stubGlobal('localStorage', localStorageMock);
    vi.restoreAllMocks();
  });

  describe('formatCurrency', () => {
    it('formats USD with standard 2 decimal places and dollar symbol', () => {
      expect(formatCurrency(100, 'USD')).toBe('$100.00');
      expect(formatCurrency(49.99, 'USD')).toBe('$49.99');
      expect(formatCurrency(0, 'USD')).toBe('$0.00');
    });

    it('converts and formats EUR with euro symbol', () => {
      // 100 USD * 0.92 = 92.00 EUR
      expect(formatCurrency(100, 'EUR')).toBe('€92.00');
    });

    it('converts and formats GBP with pound symbol', () => {
      // 100 USD * 0.79 = 79.00 GBP
      expect(formatCurrency(100, 'GBP')).toBe('£79.00');
    });

    it('enforces zero-decimal integer precision for Japanese Yen (JPY)', () => {
      // 100 USD * 155.0 = 15500 JPY (no decimal places permitted)
      const formatted = formatCurrency(100, 'JPY');
      expect(formatted).toBe('¥15,500');
      expect(formatted).not.toContain('.');
    });

    it('handles negative or NaN gracefully by defaulting to 0', () => {
      expect(formatCurrency(-50, 'USD')).toBe('$0.00');
      expect(formatCurrency(NaN, 'USD')).toBe('$0.00');
    });
  });

  describe('calculateTax', () => {
    it('calculates standard regional tax on full subtotal', () => {
      // EUR tax rate is 20%
      const res = calculateTax(100, 0, 'EUR');
      expect(res.taxRate).toBe(0.20);
      expect(res.taxRatePercent).toBe('20%');
      expect(res.taxAmount).toBe(18.4); // 92 * 0.20
      expect(res.taxableAmount).toBe(92);
      expect(res.finalTotal).toBe(110.4);
      expect(res.symbol).toBe('€');
    });

    it('applies tax strictly AFTER subtracting promotional voucher discounts', () => {
      // Subtotal 100 USD, Voucher discount 40 USD -> Taxable 60 USD
      // In GBP (Rate 0.79, VAT 20%):
      // Taxable GBP = 60 * 0.79 = 47.40 GBP
      // Tax GBP = 47.40 * 0.20 = 9.48 GBP
      // Final Total = 56.88 GBP
      const res = calculateTax(100, 40, 'GBP');
      expect(res.taxableAmount).toBe(47.40);
      expect(res.taxAmount).toBe(9.48);
      expect(res.finalTotal).toBe(56.88);
      expect(res.taxAmountFormatted).toBe('£9.48');
      expect(res.finalTotalFormatted).toBe('£56.88');
    });

    it('handles fully discounted tickets with 0 tax', () => {
      const res = calculateTax(100, 100, 'USD');
      expect(res.taxableAmount).toBe(0);
      expect(res.taxAmount).toBe(0);
      expect(res.finalTotal).toBe(0);
    });

    it('rounds JPY tax and total to whole integers', () => {
      const res = calculateTax(100, 0, 'JPY');
      expect(Number.isInteger(res.taxAmount)).toBe(true);
      expect(Number.isInteger(res.finalTotal)).toBe(true);
      expect(res.taxAmountFormatted).not.toContain('.');
    });
  });

  describe('Currency Persistence', () => {
    it('defaults to USD when no currency stored', () => {
      expect(getStoredCurrency()).toBe('USD');
    });

    it('persists and retrieves valid supported currency', () => {
      setStoredCurrency('EUR');
      expect(getStoredCurrency()).toBe('EUR');

      setStoredCurrency('JPY');
      expect(getStoredCurrency()).toBe('JPY');
    });

    it('falls back to USD when invalid currency code provided', () => {
      setStoredCurrency('INVALID_CURRENCY');
      expect(getStoredCurrency()).toBe('USD');
    });
  });

  describe('Invoice API & PDF Endpoints', () => {
    it('constructs correct PDF invoice URL', () => {
      expect(getInvoicePdfUrl('bk-999')).toContain('/bookings/bk-999/invoice/pdf');
    });

    it('fetches structured invoice metadata', async () => {
      const mockInvoice = {
        invoiceNumber: 'INV-20261018-REF12345',
        bookingId: 'bk-12345',
        bookingReference: 'REF12345',
        issueDate: '2026-10-18',
        dueDate: '2026-10-18',
        status: 'CONFIRMED',
        paymentStatus: 'PAID',
        customerName: 'Acme International',
        customerEmail: 'billing@acme.org',
        companyName: 'Acme International Ltd',
        taxId: 'EU-VAT-12345678',
        billingAddress: '100 Innovation Way',
        eventTitle: 'Galactic Summit 2026',
        currency: 'EUR',
        currencySymbol: '€',
        exchangeRate: 0.92,
        subtotal: 92.00,
        discount: 0,
        taxableAmount: 92.00,
        taxRate: 0.20,
        taxAmount: 18.40,
        totalAmount: 110.40,
        issuerName: 'TicketVerse Global Entertainment Inc.',
        issuerTaxId: 'EU-VAT-987654321',
        issuerAddress: '100 Innovation Plaza, Suite 800',
        items: [],
      };

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockInvoice,
      } as Response);

      const inv = await getInvoice('bk-12345');
      expect(inv.invoiceNumber).toBe('INV-20261018-REF12345');
      expect(inv.companyName).toBe('Acme International Ltd');
      expect(inv.taxId).toBe('EU-VAT-12345678');
      expect(inv.currency).toBe('EUR');
      expect(inv.totalAmount).toBe(110.40);
    });

    it('propagates currency and corporate billing details during checkout createBooking', async () => {
      const mockBookingResponse = {
        bookingId: 'bk-corp-123',
        bookingReference: 'REF-CORP-123',
        status: 'CONFIRMED',
        eventId: 'evt-2026',
        quantity: 2,
        seats: ['A1', 'A2'],
        totalPrice: 184.00,
        currency: 'EUR',
        exchangeRate: 0.92,
        taxRate: 0.20,
        taxAmount: 36.80,
        companyName: 'Starlight Media Corp',
        taxId: 'FR-VAT-98765432',
        billingAddress: '12 Avenue des Champs-Elysees, Paris',
        customerName: 'Pierre Dupont',
        customerEmail: 'pierre@starlight.fr',
        tickets: [],
        createdAt: '2026-10-18T12:00:00Z',
      };

      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockBookingResponse,
      } as Response);

      const res = await createBooking({
        eventId: 'evt-2026',
        seats: ['A1', 'A2'],
        customerName: 'Pierre Dupont',
        customerEmail: 'pierre@starlight.fr',
        currency: 'EUR',
        companyName: 'Starlight Media Corp',
        taxId: 'FR-VAT-98765432',
        billingAddress: '12 Avenue des Champs-Elysees, Paris',
      });

      expect(fetchSpy).toHaveBeenCalled();
      const callBody = JSON.parse(fetchSpy.mock.calls[0][1]?.body as string);
      expect(callBody.currency).toBe('EUR');
      expect(callBody.companyName).toBe('Starlight Media Corp');
      expect(callBody.taxId).toBe('FR-VAT-98765432');

      expect(res.bookingId).toBe('bk-corp-123');
      expect(res.companyName).toBe('Starlight Media Corp');
      expect(res.currency).toBe('EUR');
    });
  });

  describe('Registry Integrity', () => {
    it('contains all 6 core world currencies with valid configuration', () => {
      const expectedCodes = ['USD', 'EUR', 'GBP', 'JPY', 'CAD', 'AUD'];
      for (const code of expectedCodes) {
        const c = SUPPORTED_CURRENCIES[code];
        expect(c).toBeDefined();
        expect(c.symbol).toBeTruthy();
        expect(c.rate).toBeGreaterThan(0);
        expect(c.taxRate).toBeGreaterThan(0);
      }
    });
  });
});
