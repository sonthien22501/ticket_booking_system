import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  fetchEventMashup,
  requestConciergeAdvice,
  fetchOpenApiSpec,
} from '../services/api';
import {
  EventMashupData,
  ConciergeAdviceRequest,
  ConciergeAdviceResponse,
} from '../types';

describe('External API Mash-Up Hub & Runtime AI Concierge (Milestone 33)', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('fetchEventMashup API', () => {
    it('successfully fetches event mashup with weather and iTunes preview', async () => {
      const mockMashup: EventMashupData = {
        eventId: 'evt-101',
        eventTitle: 'Neon Symphony 2026',
        venueName: 'Grand Hall',
        venueCity: 'San Francisco',
        latitude: 37.7749,
        longitude: -122.4194,
        weather: {
          temperatureC: 18.5,
          temperatureF: 65.3,
          condition: 'Clear Skies',
          weatherCode: 0,
          precipitationProb: 0,
          windSpeedKmh: 12.0,
          isOutdoorFriendly: true,
          advisory: 'Ideal clear skies for live events.',
          source: 'open-meteo',
        },
        mediaPreview: {
          trackName: 'Midnight City',
          artistName: 'M83',
          albumName: 'Hurry Up',
          previewUrl: 'https://audio-ssl.itunes.apple.com/preview.m4a',
          artworkUrl: 'https://is1-ssl.mzstatic.com/image.jpg',
          genre: 'Electronic',
          releaseDate: '2011-10-18T07:00:00Z',
          source: 'itunes',
        },
        fallbackUsed: false,
        cached: false,
        fetchedAt: '2026-10-27T18:00:00Z',
      };

      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockMashup,
      } as Response);

      const res = await fetchEventMashup('evt-101');
      expect(fetchSpy).toHaveBeenCalled();
      const calledUrl = fetchSpy.mock.calls[0][0] as string;
      expect(calledUrl).toContain('/events/evt-101/mashup');
      expect(res.eventId).toBe('evt-101');
      expect(res.weather.condition).toBe('Clear Skies');
      expect(res.mediaPreview.trackName).toBe('Midnight City');
      expect(res.fallbackUsed).toBe(false);
    });

    it('falls back gracefully to offline fallback when endpoint fails or times out', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network offline or gateway timeout'));

      const res = await fetchEventMashup('evt-101');
      expect(res.eventId).toBe('evt-101');
      expect(res.fallbackUsed).toBe(true);
      expect(res.weather.source).toContain('fallback');
      expect(res.mediaPreview.previewUrl).toBeTruthy();
    });
  });

  describe('requestConciergeAdvice API', () => {
    it('submits advice request and parses structured concierge recommendation', async () => {
      const mockReq: ConciergeAdviceRequest = {
        eventId: 'evt-101',
        eventTitle: 'Neon Symphony 2026',
        category: 'CONCERT',
        venueName: 'Grand Hall',
        currency: 'USD',
        budgetMax: 200,
        weatherCondition: 'Clear',
        rainProbability: 5,
        preferences: 'vip, cocktails',
      };

      const mockAdvice: ConciergeAdviceResponse = {
        eventId: 'evt-101',
        headline: 'Concierge Itinerary: Neon Symphony 2026 at Grand Hall',
        weatherAdvisory: 'Clear skies expected with low precipitation.',
        recommendedTier: 'VIP Lounge & Front Stage Tier',
        recommendedAddonName: 'Artisan Food & Craft Cocktail Package',
        recommendedAddonId: 'addon-concessions-combo',
        localizedBudgetNote: 'Fits within your 200.00 USD budget.',
        insiderTips: ['Arrive 45 mins early', 'Digital QR ticketing'],
        composedSources: ['catalog-service', 'open-meteo', 'frankfurter-ecb'],
        engine: 'runtime-heuristic',
        fallbackUsed: false,
        generatedAt: '2026-10-27T18:00:00Z',
      };

      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockAdvice,
      } as Response);

      const res = await requestConciergeAdvice(mockReq);
      expect(fetchSpy).toHaveBeenCalled();
      const calledUrl = fetchSpy.mock.calls[0][0] as string;
      expect(calledUrl).toContain('/concierge/advise');
      expect(res.eventId).toBe('evt-101');
      expect(res.recommendedTier).toBe('VIP Lounge & Front Stage Tier');
      expect(res.recommendedAddonName).toBe('Artisan Food & Craft Cocktail Package');
    });

    it('returns client-side synthesized fallback if concierge endpoint fails', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Connection refused'));

      const res = await requestConciergeAdvice({
        eventId: 'evt-999',
        eventTitle: 'Acoustic Night',
        budgetMax: 100,
        currency: 'EUR',
      });

      expect(res.eventId).toBe('evt-999');
      expect(res.fallbackUsed).toBe(true);
      expect(res.recommendedTier).toBeTruthy();
      expect(res.insiderTips.length).toBeGreaterThan(0);
    });
  });

  describe('fetchOpenApiSpec API', () => {
    it('fetches OpenAPI 3.0 specification successfully', async () => {
      const mockSpec = {
        openapi: '3.0.3',
        info: { title: 'Ticket Booking System API', version: '1.0.0' },
        paths: {
          '/api/events': {},
          '/api/events/{id}/mashup': {},
          '/api/bookings': {},
          '/api/currencies/rates': {},
          '/api/concierge/advise': {},
        },
      };

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockSpec,
      } as Response);

      const spec = await fetchOpenApiSpec();
      expect(spec.openapi).toBe('3.0.3');
      expect(spec.paths).toBeDefined();
    });
  });
});
