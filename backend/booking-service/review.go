package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// EventReview represents an attendee review and rating
type EventReview struct {
	ID            string `json:"id"`
	EventID       string `json:"eventId"`
	UserID        string `json:"userId,omitempty"`
	BookingID     string `json:"bookingId,omitempty"`
	ReviewerName  string `json:"reviewerName"`
	ReviewerEmail string `json:"reviewerEmail"`
	Rating        int    `json:"rating"`
	Title         string `json:"title"`
	Comment       string `json:"comment"`
	VerifiedBuyer bool   `json:"verifiedBuyer"`
	HelpfulVotes  int    `json:"helpfulVotes"`
	CreatedAt     string `json:"createdAt"`
}

// CreateReviewRequest payload for submitting a review
type CreateReviewRequest struct {
	EventID       string `json:"eventId"`
	UserID        string `json:"userId,omitempty"`
	BookingID     string `json:"bookingId,omitempty"`
	ReviewerName  string `json:"reviewerName"`
	ReviewerEmail string `json:"reviewerEmail"`
	Rating        int    `json:"rating"`
	Title         string `json:"title"`
	Comment       string `json:"comment"`
}

// ReviewSummary holds aggregated rating statistics
type ReviewSummary struct {
	AverageRating      float64     `json:"averageRating"`
	TotalReviews       int         `json:"totalReviews"`
	RatingDistribution map[int]int `json:"ratingDistribution"`
}

// EventReviewsResponse combines summary statistics with the list of reviews
type EventReviewsResponse struct {
	EventID string        `json:"eventId"`
	Summary ReviewSummary `json:"summary"`
	Reviews []EventReview `json:"reviews"`
}

var (
	reviewsMutex sync.RWMutex
	memReviews   = make(map[string][]EventReview) // eventID -> list of reviews
)

func init() {
	// Seed realistic reviews for demo events
	now := time.Now().UTC()
	memReviews["evt-101"] = []EventReview{
		{
			ID:            "rev-101-1",
			EventID:       "evt-101",
			ReviewerName:  "Elena Vance",
			ReviewerEmail: "elena@example.com",
			Rating:        5,
			Title:         "Mind-blowing synthesizer acoustics!",
			Comment:       "The laser lighting and surround spatial audio in row A was worth every penny. VIP parking was super fast.",
			VerifiedBuyer: true,
			HelpfulVotes:  18,
			CreatedAt:     now.Add(-48 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:            "rev-101-2",
			EventID:       "evt-101",
			ReviewerName:  "Marcus Brody",
			ReviewerEmail: "marcus@example.com",
			Rating:        5,
			Title:         "Epic encore and pristine venue",
			Comment:       "Loved the contactless gate scan pass. Sound engineer deserves a Grammy.",
			VerifiedBuyer: true,
			HelpfulVotes:  7,
			CreatedAt:     now.Add(-24 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:            "rev-101-3",
			EventID:       "evt-101",
			ReviewerName:  "Sarah Connor",
			ReviewerEmail: "sarah@example.com",
			Rating:        4,
			Title:         "Incredible performance, slight queue at merchandise",
			Comment:       "Great show from start to finish. Concessions were excellent.",
			VerifiedBuyer: false,
			HelpfulVotes:  3,
			CreatedAt:     now.Add(-12 * time.Hour).Format(time.RFC3339),
		},
	}

	memReviews["evt-102"] = []EventReview{
		{
			ID:            "rev-102-1",
			EventID:       "evt-102",
			ReviewerName:  "David Kim",
			ReviewerEmail: "david@example.com",
			Rating:        5,
			Title:         "The best keynote in years",
			Comment:       "Crisp AV presentation and great networking lounge hospitality.",
			VerifiedBuyer: true,
			HelpfulVotes:  12,
			CreatedAt:     now.Add(-72 * time.Hour).Format(time.RFC3339),
		},
	}
}

// validateReviewRequest ensures inputs meet criteria
func validateReviewRequest(req *CreateReviewRequest) error {
	if req.Rating < 1 || req.Rating > 5 {
		return errors.New("rating must be an integer between 1 and 5")
	}
	if strings.TrimSpace(req.ReviewerName) == "" {
		return errors.New("reviewer name is required")
	}
	if strings.TrimSpace(req.Comment) == "" {
		return errors.New("review comment is required")
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = fmt.Sprintf("%d-Star Experience", req.Rating)
	}
	return nil
}

// calculateReviewSummary aggregates review stats
func calculateReviewSummary(reviews []EventReview) ReviewSummary {
	dist := map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0}
	if len(reviews) == 0 {
		return ReviewSummary{
			AverageRating:      0.0,
			TotalReviews:       0,
			RatingDistribution: dist,
		}
	}

	sum := 0
	for _, r := range reviews {
		if r.Rating >= 1 && r.Rating <= 5 {
			dist[r.Rating]++
			sum += r.Rating
		}
	}

	avg := float64(sum) / float64(len(reviews))
	roundedAvg := math.Round(avg*10) / 10

	return ReviewSummary{
		AverageRating:      roundedAvg,
		TotalReviews:       len(reviews),
		RatingDistribution: dist,
	}
}

// checkVerifiedBuyer checks whether the attendee has a confirmed booking for the event
func checkVerifiedBuyer(ctx context.Context, db *sql.DB, eventID, userID, email string) bool {
	if db == nil {
		// Mock logic: if email is provided, simulate verified for known users
		return email != "" && (strings.Contains(email, "example.com") || strings.Contains(email, "buyer") || userID != "")
	}

	var count int
	query := `
		SELECT COUNT(*)
		FROM bookings
		WHERE event_id = $1
		  AND (
		      (user_id = $2 AND $2 != '')
		      OR (LOWER(customer_email) = LOWER($3) AND $3 != '')
		  )
		  AND status = 'CONFIRMED'
	`
	err := db.QueryRowContext(ctx, query, eventID, userID, email).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// fetchReviewsFromDB retrieves reviews for an event from Postgres
func fetchReviewsFromDB(ctx context.Context, db *sql.DB, eventID string) ([]EventReview, error) {
	query := `
		SELECT id, event_id, COALESCE(user_id, ''), COALESCE(booking_id, ''),
		       reviewer_name, reviewer_email, rating, title, comment,
		       verified_buyer, helpful_votes, created_at
		FROM reviews
		WHERE event_id = $1
		ORDER BY created_at DESC
	`
	rows, err := db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []EventReview
	for rows.Next() {
		var r EventReview
		var t time.Time
		if err := rows.Scan(
			&r.ID, &r.EventID, &r.UserID, &r.BookingID,
			&r.ReviewerName, &r.ReviewerEmail, &r.Rating, &r.Title, &r.Comment,
			&r.VerifiedBuyer, &r.HelpfulVotes, &t,
		); err != nil {
			return nil, err
		}
		r.CreatedAt = t.Format(time.RFC3339)
		reviews = append(reviews, r)
	}
	return reviews, nil
}

// insertReviewToDB inserts a review into Postgres
func insertReviewToDB(ctx context.Context, db *sql.DB, r *EventReview) error {
	query := `
		INSERT INTO reviews (
			id, event_id, user_id, booking_id,
			reviewer_name, reviewer_email, rating, title, comment,
			verified_buyer, helpful_votes, created_at
		) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11, NOW())
	`
	_, err := db.ExecContext(
		ctx, query,
		r.ID, r.EventID, r.UserID, r.BookingID,
		r.ReviewerName, r.ReviewerEmail, r.Rating, r.Title, r.Comment,
		r.VerifiedBuyer, r.HelpfulVotes,
	)
	return err
}

// handleEventReviews handles GET and POST on /api/events/{id}/reviews
func (s *Server) handleEventReviews(w http.ResponseWriter, r *http.Request, eventID string) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		var reviews []EventReview
		var err error

		if s != nil && s.db != nil {
			reviews, err = fetchReviewsFromDB(r.Context(), s.db, eventID)
		}
		if s == nil || s.db == nil || err != nil {
			reviewsMutex.RLock()
			reviews = memReviews[eventID]
			reviewsMutex.RUnlock()
		}

		if reviews == nil {
			reviews = []EventReview{}
		}

		summary := calculateReviewSummary(reviews)
		resp := EventReviewsResponse{
			EventID: eventID,
			Summary: summary,
			Reviews: reviews,
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	if r.Method == http.MethodPost {
		var req CreateReviewRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid request payload"}`, http.StatusBadRequest)
			return
		}
		if req.EventID == "" {
			req.EventID = eventID
		}

		if err := validateReviewRequest(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		var isVerified bool
		if s != nil && s.db != nil {
			isVerified = checkVerifiedBuyer(r.Context(), s.db, eventID, req.UserID, req.ReviewerEmail)
		} else {
			isVerified = checkVerifiedBuyer(r.Context(), nil, eventID, req.UserID, req.ReviewerEmail)
		}

		reviewID := fmt.Sprintf("rev-%d-%d", time.Now().UnixNano()%1000000, req.Rating)

		newReview := EventReview{
			ID:            reviewID,
			EventID:       eventID,
			UserID:        req.UserID,
			BookingID:     req.BookingID,
			ReviewerName:  strings.TrimSpace(req.ReviewerName),
			ReviewerEmail: strings.TrimSpace(req.ReviewerEmail),
			Rating:        req.Rating,
			Title:         strings.TrimSpace(req.Title),
			Comment:       strings.TrimSpace(req.Comment),
			VerifiedBuyer: isVerified,
			HelpfulVotes:  0,
			CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		}

		if s != nil && s.db != nil {
			_ = insertReviewToDB(r.Context(), s.db, &newReview)
		}

		// Also persist in memory store
		reviewsMutex.Lock()
		memReviews[eventID] = append([]EventReview{newReview}, memReviews[eventID]...)
		reviewsMutex.Unlock()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(newReview)
		return
	}

	http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
}

// handleHelpfulVote increments the helpful vote count for a review
func (s *Server) handleHelpfulVote(w http.ResponseWriter, r *http.Request, reviewID string) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if s != nil && s.db != nil {
		_, _ = s.db.ExecContext(r.Context(), `UPDATE reviews SET helpful_votes = helpful_votes + 1 WHERE id = $1`, reviewID)
	}

	reviewsMutex.Lock()
	for eID, list := range memReviews {
		for i, rev := range list {
			if rev.ID == reviewID {
				memReviews[eID][i].HelpfulVotes++
				break
			}
		}
	}
	reviewsMutex.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"reviewId": reviewID,
		"message":  "Helpful vote registered successfully",
	})
}

// handleEventsDispatcher dispatches sub-resources under /api/events/ and /events/
func (s *Server) handleEventsDispatcher(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/events/")
	path = strings.TrimPrefix(path, "/events/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 {
		eventID := parts[0]
		sub := parts[1]
		if sub == "addons" {
			s.handleEventAddons(w, r)
			return
		}
		if sub == "reviews" {
			s.handleEventReviews(w, r, eventID)
			return
		}
	}
	http.NotFound(w, r)
}

// handleReviewsDispatcher dispatches /api/reviews/ and /reviews/
func (s *Server) handleReviewsDispatcher(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/reviews/")
	path = strings.TrimPrefix(path, "/reviews/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[1] == "vote" {
		reviewID := parts[0]
		s.handleHelpfulVote(w, r, reviewID)
		return
	}
	http.NotFound(w, r)
}
