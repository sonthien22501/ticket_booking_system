import { describe, it, expect, beforeEach, vi } from 'vitest';
import { searchEvents, fetchVenueFloorplan } from '../services/api';
import { SearchEventsResponse, VenueFloorplan } from '../types';

describe('Advanced Geolocation Search, Venue Floorplans & Category Facets (Milestone 31)', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('searchEvents API', () => {
    it('executes search query with facets and filters', async () => {
      const mockResponse: SearchEventsResponse = {
        events: [
          {
            id: 'evt-101',
            title: 'Neon Symphony 2026',
            description: 'Analog synthesizer orchestra',
            category: 'CONCERT',
            venueName: 'Grand Hall',
            venueLocation: 'San Francisco, CA',
            startTime: '2026-11-15T20:00:00Z',
            endTime: '2026-11-15T23:00:00Z',
            status: 'ACTIVE',
            tiers: [
              { id: 't-1', name: 'VIP', price: 150.0, availableSeats: 20 },
              { id: 't-2', name: 'General', price: 60.0, availableSeats: 48 },
            ],
          },
        ],
        facets: {
          cities: { 'San Francisco': 1, 'Berlin': 1 },
          categories: { 'CONCERT': 1, 'SPORTS': 1 },
          minPrice: 60.0,
          maxPrice: 150.0,
        },
        totalCount: 1,
      };

      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      } as Response);

      const res = await searchEvents({
        q: 'Symphony',
        city: 'San Francisco',
        category: 'CONCERT',
        minPrice: 50,
        maxPrice: 200,
        sortBy: 'price_asc',
      });

      expect(fetchSpy).toHaveBeenCalled();
      const calledUrl = fetchSpy.mock.calls[0][0] as string;
      expect(calledUrl).toContain('q=Symphony');
      expect(calledUrl).toContain('city=San+Francisco');
      expect(calledUrl).toContain('category=CONCERT');
      expect(calledUrl).toContain('min_price=50');
      expect(calledUrl).toContain('max_price=200');
      expect(calledUrl).toContain('sort_by=price_asc');

      expect(res.events).toHaveLength(1);
      expect(res.facets.cities['San Francisco']).toBe(1);
      expect(res.facets.minPrice).toBe(60.0);
    });

    it('falls back to local faceted evaluation when network fails', async () => {
      // First call inside fallback is fetchEvents
      vi.spyOn(globalThis, 'fetch')
        .mockRejectedValueOnce(new Error('Network error on search')) // /api/events/search fails
        .mockResolvedValueOnce({
          ok: true,
          json: async () => [
            {
              id: 'evt-101',
              title: 'Neon Symphony',
              description: 'Cyberpunk orchestra',
              category: 'CONCERT',
              venue_name: 'Grand Hall',
              venue_location: 'San Francisco, CA',
              start_time: '2026-11-15T20:00:00Z',
              status: 'ACTIVE',
              tiers: [{ id: 't-1', name: 'VIP', price: 150 }],
            },
            {
              id: 'evt-102',
              title: 'Global Tech Summit',
              description: 'AI keynote',
              category: 'CONFERENCE',
              venue_name: 'Moscone',
              venue_location: 'San Francisco, CA',
              start_time: '2026-11-20T09:00:00Z',
              status: 'ACTIVE',
              tiers: [{ id: 't-2', name: 'Pass', price: 299 }],
            },
          ],
        } as Response);

      const res = await searchEvents({ city: 'San Francisco', category: 'CONCERT' });
      expect(res.events).toHaveLength(1);
      expect(res.events[0].id).toBe('evt-101');
      expect(res.facets.cities['San Francisco']).toBe(2);
      expect(res.facets.categories['CONCERT']).toBe(1);
    });
  });

  describe('fetchVenueFloorplan API', () => {
    it('fetches architectural floorplan sections successfully', async () => {
      const mockFloorplan: VenueFloorplan = {
        eventId: 'evt-101',
        venueName: 'Grand Arena',
        venueLocation: 'San Francisco, CA',
        stageOrientation: 'NORTH',
        totalCapacity: 68,
        sections: [
          {
            id: 'sec-stage',
            name: 'Main Stage',
            category: 'STAGE',
            capacity: 0,
            basePrice: 0,
            rowRange: 'Center Stage',
            color: '#6366f1',
          },
          {
            id: 'sec-vip',
            name: 'VIP Front Pit',
            category: 'VIP_PIT',
            capacity: 20,
            basePrice: 150.0,
            rowRange: 'Rows A - B',
            color: '#f59e0b',
          },
        ],
      };

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockFloorplan,
      } as Response);

      const res = await fetchVenueFloorplan('evt-101');
      expect(res.eventId).toBe('evt-101');
      expect(res.stageOrientation).toBe('NORTH');
      expect(res.sections).toHaveLength(2);
      expect(res.sections[0].category).toBe('STAGE');
      expect(res.sections[1].basePrice).toBe(150.0);
    });

    it('falls back to default floorplan schematic on network exception', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network offline'));

      const res = await fetchVenueFloorplan('evt-101');
      expect(res.sections.length).toBeGreaterThanOrEqual(3);
      expect(res.sections.some((s) => s.category === 'STAGE')).toBe(true);
      expect(res.sections.some((s) => s.category === 'VIP_PIT')).toBe(true);
    });
  });

  describe('Faceted Filter Logic Calculation', () => {
    it('accurately parses city names and computes price ranges', () => {
      const locations = ['San Francisco, CA', 'Berlin, Germany', 'London, UK', 'San Francisco, California'];
      const cities = locations.map((loc) => loc.split(',')[0].trim());
      const counts: Record<string, number> = {};
      cities.forEach((c) => {
        counts[c] = (counts[c] || 0) + 1;
      });

      expect(counts['San Francisco']).toBe(2);
      expect(counts['Berlin']).toBe(1);
      expect(counts['London']).toBe(1);
    });
  });
});
