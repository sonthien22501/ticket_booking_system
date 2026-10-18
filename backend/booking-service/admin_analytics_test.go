package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAdminAnalytics_MethodNotAllowed(t *testing.T) {
	s := &Server{db: nil}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/analytics", nil)
	rr := httptest.NewRecorder()

	s.handleAdminAnalytics(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d Method Not Allowed, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestHandleAdminAnalytics_EmptyDB(t *testing.T) {
	s := &Server{db: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics", nil)
	rr := httptest.NewRecorder()

	s.handleAdminAnalytics(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK even without DB, got %d", rr.Code)
	}

	var res AdminAnalyticsResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if res.RecentBookings == nil {
		t.Errorf("expected recentBookings to be non-nil slice, got nil")
	}
	if res.EventsSummary == nil {
		t.Errorf("expected eventsSummary to be non-nil slice, got nil")
	}
}
