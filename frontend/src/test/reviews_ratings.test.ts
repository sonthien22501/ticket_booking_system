import { describe, it, expect, beforeEach, vi } from 'vitest';
import { fetchEventReviews, submitEventReview, voteHelpfulReview } from '../services/api';
import { EventReviewsResponse, CreateReviewRequest } from '../types';

describe('Attendee Reviews, Verified Buyer Ratings & Social Proof Matrix (Milestone 30)', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  describe('fetchEventReviews API', () => {
    it('successfully retrieves event reviews and rating summary', async () => {
      const mockResponse: EventReviewsResponse = {
        eventId: 'evt-101',
        summary: {
          averageRating: 4.8,
          totalReviews: 2,
          ratingDistribution: { 1: 0, 2: 0, 3: 0, 4: 1, 5: 1 },
        },
        reviews: [
          {
            id: 'rev-1',
            eventId: 'evt-101',
            reviewerName: 'Alice',
            reviewerEmail: 'alice@example.com',
            rating: 5,
            title: 'Fantastic show',
            comment: 'Stage lighting was brilliant.',
            verifiedBuyer: true,
            helpfulVotes: 12,
            createdAt: '2026-10-24T00:00:00Z',
          },
          {
            id: 'rev-2',
            eventId: 'evt-101',
            reviewerName: 'Bob',
            reviewerEmail: 'bob@example.com',
            rating: 4,
            title: 'Great sound',
            comment: 'Concessions line was long.',
            verifiedBuyer: false,
            helpfulVotes: 3,
            createdAt: '2026-10-23T00:00:00Z',
          },
        ],
      };

      vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      } as Response);

      const res = await fetchEventReviews('evt-101');
      expect(res.eventId).toBe('evt-101');
      expect(res.summary.averageRating).toBe(4.8);
      expect(res.summary.totalReviews).toBe(2);
      expect(res.reviews).toHaveLength(2);
      expect(res.reviews[0].verifiedBuyer).toBe(true);
    });

    it('falls back to demo reviews when network fails', async () => {
      vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network offline'));

      const res = await fetchEventReviews('evt-101');
      expect(res.summary.totalReviews).toBeGreaterThan(0);
      expect(res.reviews.length).toBeGreaterThan(0);
      expect(res.reviews.some((r) => r.verifiedBuyer)).toBe(true);
    });
  });

  describe('submitEventReview API', () => {
    it('submits review and returns created review with verified status', async () => {
      const payload: CreateReviewRequest = {
        eventId: 'evt-101',
        reviewerName: 'Charlie',
        reviewerEmail: 'charlie@tickets.com',
        rating: 5,
        title: 'Masterpiece',
        comment: 'Acoustics were exceptional.',
      };

      const mockCreated = {
        id: 'rev-new-123',
        ...payload,
        verifiedBuyer: true,
        helpfulVotes: 0,
        createdAt: new Date().toISOString(),
      };

      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => mockCreated,
      } as Response);

      const created = await submitEventReview(payload);
      expect(fetchSpy).toHaveBeenCalled();
      expect(created.id).toBe('rev-new-123');
      expect(created.rating).toBe(5);
      expect(created.verifiedBuyer).toBe(true);
    });
  });

  describe('voteHelpfulReview API', () => {
    it('sends POST request to upvote helpfulness', async () => {
      const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: true,
        json: async () => ({ success: true }),
      } as Response);

      const ok = await voteHelpfulReview('rev-101-1');
      expect(fetchSpy).toHaveBeenCalledWith(
        '/api/reviews/rev-101-1/vote',
        expect.objectContaining({ method: 'POST' })
      );
      expect(ok).toBe(true);
    });
  });

  describe('Social Proof Distribution Calculation Logic', () => {
    it('calculates average star rating and verified buyer percentage', () => {
      const reviews = [
        { rating: 5, verifiedBuyer: true },
        { rating: 5, verifiedBuyer: true },
        { rating: 4, verifiedBuyer: false },
        { rating: 4, verifiedBuyer: true },
        { rating: 2, verifiedBuyer: false },
      ];

      const sum = reviews.reduce((acc, r) => acc + r.rating, 0);
      const avg = Number((sum / reviews.length).toFixed(1));
      // (5+5+4+4+2)/5 = 20/5 = 4.0
      expect(avg).toBe(4.0);

      const verifiedCount = reviews.filter((r) => r.verifiedBuyer).length;
      const verifiedPct = Math.round((verifiedCount / reviews.length) * 100);
      // 3 of 5 = 60%
      expect(verifiedPct).toBe(60);
    });
  });
});
