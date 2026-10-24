# Milestone 30: Attendee Reviews, Verified Buyer Ratings & Social Proof Matrix

## Overview
Implement an attendee review, star rating, and social proof system where event-goers can leave authentic ratings (1-5 stars) and testimonials. Automatically marks attendees as "Verified Buyer" if they possess a confirmed booking for the event. Computes aggregate event rating scores, rating distributions, and helpful vote interactions.

---

## Architecture & Data Flow

```
[ Frontend: Event Catalog / Details / Reviews Modal ]
                        │
                        ▼ :8080 (API Gateway)
        /api/events/:id/reviews  /api/reviews
                        │
                        ▼ :8083 (Booking Service)
            [ Review Handlers & Verifier ]
           /                              \
          ▼                                ▼
[ bookings / tickets DB ]           [ reviews DB ]
(verify purchaser status)       (ratings, comments, votes)
```

---

## 1. Database Schema
- Table: `reviews`
  - `id VARCHAR(64) PRIMARY KEY`
  - `event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE`
  - `user_id VARCHAR(64) REFERENCES users(id) ON DELETE SET NULL`
  - `booking_id VARCHAR(64) REFERENCES bookings(id) ON DELETE SET NULL`
  - `reviewer_name VARCHAR(128) NOT NULL`
  - `reviewer_email VARCHAR(128) NOT NULL`
  - `rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5)`
  - `title VARCHAR(255) NOT NULL`
  - `comment TEXT NOT NULL`
  - `verified_buyer BOOLEAN DEFAULT FALSE`
  - `helpful_votes INT DEFAULT 0`
  - `created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP`
- Indexes:
  - `idx_reviews_event (event_id, created_at DESC)`
  - `idx_reviews_user (user_id)`
  - `idx_reviews_verified (event_id, verified_buyer)`

---

## 2. API Endpoints
- `GET /api/events/:id/reviews`: Retrieves all reviews for event + summary stats (`averageRating`, `totalReviews`, `ratingDistribution: { 1: n, 2: n, ... }`).
- `POST /api/events/:id/reviews` or `POST /api/reviews`: Submits a review. If user email/ID has a confirmed booking for the event, automatically marks `verified_buyer = true`.
- `POST /api/reviews/:id/vote`: Increments `helpful_votes` for a review.

---

## 3. TDD Plan
- Go RED tests: `backend/booking-service/review_test.go`
  - Test review submission with rating validation (1 to 5).
  - Test verified buyer detection against booking records.
  - Test rating summary and breakdown calculations.
  - Test helpful vote increment.
- Go GREEN implementation: `backend/booking-service/review.go` and route registration in `main.go`.
- Frontend Vitest: `frontend/src/test/reviews_ratings.test.ts`
  - Test review list display with star ratings and verified buyer badge.
  - Test review submission form validation.
  - Test average rating badge computation.
- Frontend UI:
  - `frontend/src/types/index.ts`: `EventReview`, `ReviewSummary`, `CreateReviewRequest`.
  - `frontend/src/services/api.ts`: `fetchEventReviews`, `submitEventReview`, `voteHelpfulReview`.
  - `frontend/src/components/ReviewModal.tsx`: Comprehensive review browser with star breakdown, filter by rating, verified attendee badge, and review submission form.
  - `frontend/src/components/EventCatalog.tsx`: Show star rating badge on each event card (e.g. `★ 4.9 (42)`).
