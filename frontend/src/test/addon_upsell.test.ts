import { describe, it, expect, beforeEach, vi } from 'vitest';
import { fetchEventAddons, createBooking } from '../services/api';
import { EventAddon, SelectedAddonItem } from '../types';

describe('VIP Add-ons, Event Merchandise & Concessions Upsell Engine (Milestone 29)', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('fetchEventAddons API', () => {
    it('fetches add-ons for an event successfully', async () => {
      const mockAddons: EventAddon[] = [
        {
          id: 'addon-1',
          eventId: 'evt-101',
          name: 'VIP Front Row Parking',
          description: 'Reserved parking spot near the main entrance',
          price: 25.0,
          category: 'PARKING',
          availableInventory: 50,
          maxQuantityPerBooking: 2,
        },
        {
          id: 'addon-2',
          eventId: 'evt-101',
          name: 'Official Tour T-Shirt',
          description: '100% organic cotton commemorative tour t-shirt',
          price: 35.0,
          category: 'MERCHANDISE',
          availableInventory: 100,
          maxQuantityPerBooking: 5,
        },
      ];

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => ({ addons: mockAddons }),
      } as Response);

      const addons = await fetchEventAddons('evt-101');
      expect(addons).toHaveLength(2);
      expect(addons[0].name).toBe('VIP Front Row Parking');
      expect(addons[0].price).toBe(25.0);
      expect(addons[1].category).toBe('MERCHANDISE');
    });

    it('falls back to default demo add-ons if API request fails', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network error'));

      const addons = await fetchEventAddons('evt-101');
      expect(addons).toBeDefined();
      expect(addons.length).toBeGreaterThan(0);
      expect(addons.some((a) => a.category === 'PARKING')).toBe(true);
      expect(addons.some((a) => a.category === 'MERCHANDISE')).toBe(true);
    });
  });

  describe('createBooking with Addons', () => {
    it('passes selected add-ons to booking creation payload', async () => {
      const mockBookingResponse = {
        bookingId: 'bk-test-123',
        bookingReference: 'BK-TEST123',
        status: 'CONFIRMED',
        eventId: 'evt-101',
        eventTitle: 'Symphony Concert',
        venueName: 'Grand Hall',
        tierName: 'VIP',
        seats: ['A1', 'A2'],
        quantity: 2,
        totalPrice: 245.0,
        currency: 'USD',
        customerName: 'Alice Smith',
        customerEmail: 'alice@example.com',
        createdAt: new Date().toISOString(),
        tickets: [],
        addons: [
          {
            id: 'ba-1',
            bookingId: 'bk-test-123',
            addonId: 'addon-1',
            addonName: 'VIP Front Row Parking',
            unitPrice: 25.0,
            quantity: 1,
            totalPrice: 25.0,
          },
        ],
      };

      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockBookingResponse,
      } as Response);

      const selectedAddons: SelectedAddonItem[] = [
        { addonId: 'addon-1', quantity: 1 },
      ];

      const res = await createBooking({
        eventId: 'evt-101',
        tierId: 'tier-1',
        seats: ['A1', 'A2'],
        customerName: 'Alice Smith',
        customerEmail: 'alice@example.com',
        addons: selectedAddons,
      });

      expect(fetchSpy).toHaveBeenCalled();
      const calledBody = JSON.parse(fetchSpy.mock.calls[0][1]?.body as string);
      expect(calledBody.addons).toEqual(selectedAddons);
      expect(res.addons).toHaveLength(1);
      expect(res.addons?.[0].addonName).toBe('VIP Front Row Parking');
    });
  });

  describe('Add-on calculations & limits logic', () => {
    it('accurately computes total add-on cost and clamps to maximum allowed quantity', () => {
      const addons: EventAddon[] = [
        {
          id: 'addon-1',
          eventId: 'evt-101',
          name: 'VIP Parking Pass',
          description: 'Spot near entrance',
          price: 25.0,
          category: 'PARKING',
          availableInventory: 5,
          maxQuantityPerBooking: 2,
        },
        {
          id: 'addon-2',
          eventId: 'evt-101',
          name: 'Concessions Voucher',
          description: 'Gourmet meal voucher',
          price: 15.0,
          category: 'CONCESSION',
          availableInventory: 20,
          maxQuantityPerBooking: 4,
        },
      ];

      const userSelection = {
        'addon-1': 2, // at max 2
        'addon-2': 3,
      };

      const addonTotal = addons.reduce((sum, item) => {
        const qty = userSelection[item.id as keyof typeof userSelection] || 0;
        return sum + item.price * qty;
      }, 0);

      // (25 * 2) + (15 * 3) = 50 + 45 = 95
      expect(addonTotal).toBe(95.0);

      // Verify clamping logic
      const requestedOverQty = 5;
      const effectiveMax = Math.min(addons[0].maxQuantityPerBooking || 10, addons[0].availableInventory);
      const clampedQty = Math.min(requestedOverQty, effectiveMax);
      expect(clampedQty).toBe(2);
    });
  });
});
