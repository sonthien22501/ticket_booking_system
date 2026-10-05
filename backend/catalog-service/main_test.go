package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleHealth(t *testing.T) {
	s := &Server{db: nil}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var res map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if res["status"] != "UP" {
		t.Errorf("expected status 'UP', got '%s'", res["status"])
	}
	if res["service"] != "catalog-service" {
		t.Errorf("expected service 'catalog-service', got '%s'", res["service"])
	}
}

func TestEventJSONSerialization(t *testing.T) {
	now := time.Now()
	event := Event{
		ID:            "evt-test-1",
		EventID:       "evt-test-1",
		Title:         "Sample Concert",
		Description:   "A test concert",
		Category:      "Music",
		VenueName:     "Grand Arena",
		VenueNameAlt:  "Grand Arena",
		VenueLocation: "Berlin",
		VenueLocAlt:   "Berlin",
		StartTime:     now,
		StartTimeAlt:  now,
		EndTime:       now.Add(2 * time.Hour),
		EndTimeAlt:    now.Add(2 * time.Hour),
		Status:        "ACTIVE",
		ImageURL:      "http://example.com/img.jpg",
		ImageURLAlt:   "http://example.com/img.jpg",
		Tiers: []TicketTier{
			{
				ID:       "tier-vip",
				TierID:   "tier-vip",
				EventID:  "evt-test-1",
				Name:     "VIP",
				Price:    100.0,
				Capacity: 50,
			},
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify both camelCase and snake_case fields exist
	if parsed["id"] != "evt-test-1" || parsed["eventId"] != "evt-test-1" {
		t.Errorf("expected id and eventId to match evt-test-1")
	}
	if parsed["venueName"] != "Grand Arena" || parsed["venue_name"] != "Grand Arena" {
		t.Errorf("expected venueName and venue_name to match Grand Arena")
	}
}
