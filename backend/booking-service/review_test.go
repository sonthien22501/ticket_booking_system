package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReviewValidationAndCalculation(t *testing.T) {
	// Test Rating Range Validation
	invalidReqLow := CreateReviewRequest{
		EventID:       "evt-101",
		ReviewerName:  "John Doe",
		ReviewerEmail: "john@example.com",
		Rating:        0,
		Title:         "Too low",
		Comment:       "Loved it",
	}
	if err := validateReviewRequest(&invalidReqLow); err == nil {
		t.Fatalf("expected error for rating 0, got nil")
	}

	invalidReqHigh := CreateReviewRequest{
		EventID:       "evt-101",
		ReviewerName:  "John Doe",
		ReviewerEmail: "john@example.com",
		Rating:        6,
		Title:         "Too high",
		Comment:       "Loved it",
	}
	if err := validateReviewRequest(&invalidReqHigh); err == nil {
		t.Fatalf("expected error for rating 6, got nil")
	}

	invalidReqEmptyName := CreateReviewRequest{
		EventID:       "evt-101",
		ReviewerName:  "",
		ReviewerEmail: "john@example.com",
		Rating:        5,
		Title:         "Great show",
		Comment:       "Loved it",
	}
	if err := validateReviewRequest(&invalidReqEmptyName); err == nil {
		t.Fatalf("expected error for empty name, got nil")
	}

	validReq := CreateReviewRequest{
		EventID:       "evt-101",
		ReviewerName:  "Jane Smith",
		ReviewerEmail: "jane@example.com",
		Rating:        5,
		Title:         "Unforgettable night!",
		Comment:       "The sound acoustics and lighting were world-class.",
	}
	if err := validateReviewRequest(&validReq); err != nil {
		t.Fatalf("expected valid review request, got %v", err)
	}

	// Test Rating Summary Aggregation
	reviews := []EventReview{
		{Rating: 5},
		{Rating: 5},
		{Rating: 4},
		{Rating: 2},
	}
	summary := calculateReviewSummary(reviews)
	if summary.TotalReviews != 4 {
		t.Errorf("expected 4 total reviews, got %d", summary.TotalReviews)
	}
	// (5 + 5 + 4 + 2) / 4 = 16 / 4 = 4.0
	if summary.AverageRating != 4.0 {
		t.Errorf("expected average rating 4.0, got %.2f", summary.AverageRating)
	}
	if summary.RatingDistribution[5] != 2 {
		t.Errorf("expected 2 5-star reviews, got %d", summary.RatingDistribution[5])
	}
	if summary.RatingDistribution[4] != 1 {
		t.Errorf("expected 1 4-star reviews, got %d", summary.RatingDistribution[4])
	}
	if summary.RatingDistribution[2] != 1 {
		t.Errorf("expected 1 2-star reviews, got %d", summary.RatingDistribution[2])
	}
}

func TestReviewEndpointsWithMockFallback(t *testing.T) {
	srv := &Server{}
	eventID := "evt-test-m30"

	// 1. GET reviews for event
	req := httptest.NewRequest(http.MethodGet, "/events/"+eventID+"/reviews", nil)
	w := httptest.NewRecorder()
	srv.handleEventReviews(w, req, eventID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", w.Code)
	}

	var resp EventReviewsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 2. POST a new review
	newReviewPayload := CreateReviewRequest{
		EventID:       eventID,
		ReviewerName:  "Dave Grohl",
		ReviewerEmail: "dave@example.com",
		Rating:        5,
		Title:         "Absolute masterpiece performance",
		Comment:       "Best live production I have ever experienced.",
	}
	bodyBytes, _ := json.Marshal(newReviewPayload)
	postReq := httptest.NewRequest(http.MethodPost, "/events/"+eventID+"/reviews", bytes.NewReader(bodyBytes))
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	srv.handleEventReviews(postW, postReq, eventID)

	if postW.Code != http.StatusCreated && postW.Code != http.StatusOK {
		t.Fatalf("expected HTTP 201 or 200, got %d (body: %s)", postW.Code, postW.Body.String())
	}

	var createdReview EventReview
	if err := json.NewDecoder(postW.Body).Decode(&createdReview); err != nil {
		t.Fatalf("failed to decode created review: %v", err)
	}
	if createdReview.Rating != 5 {
		t.Errorf("expected rating 5, got %d", createdReview.Rating)
	}
	if createdReview.ReviewerName != "Dave Grohl" {
		t.Errorf("expected reviewer name Dave Grohl, got %s", createdReview.ReviewerName)
	}

	// 3. Upvote review
	voteReq := httptest.NewRequest(http.MethodPost, "/reviews/"+createdReview.ID+"/vote", nil)
	voteW := httptest.NewRecorder()
	srv.handleHelpfulVote(voteW, voteReq, createdReview.ID)

	if voteW.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on vote, got %d", voteW.Code)
	}
}
