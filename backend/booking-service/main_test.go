package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
	if res["service"] != "booking-service" {
		t.Errorf("expected service 'booking-service', got '%s'", res["service"])
	}
}

func TestBookingOverbookingValidation(t *testing.T) {
	s := &Server{
		db:         nil,
		httpClient: &http.Client{},
	}

	body := map[string]interface{}{
		"eventId":     "evt-101",
		"ticketCount": 9999,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bookings", bytes.NewReader(bodyBytes))
	rr := httptest.NewRecorder()

	s.createBooking(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for overbooking, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestBookingMissingEventIDValidation(t *testing.T) {
	s := &Server{
		db:         nil,
		httpClient: &http.Client{},
	}

	body := map[string]interface{}{
		"ticketCount": 2,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/bookings", bytes.NewReader(bodyBytes))
	rr := httptest.NewRecorder()

	s.createBooking(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for missing eventId, got %d", http.StatusBadRequest, rr.Code)
	}
}
