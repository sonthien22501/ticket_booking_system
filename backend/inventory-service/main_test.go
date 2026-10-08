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
	if res["service"] != "seat-inventory-service" {
		t.Errorf("expected service 'seat-inventory-service', got '%s'", res["service"])
	}
}



func TestReserveRequestMissingEventID(t *testing.T) {
	s := &Server{
		db:      nil,
		lockMgr: nil,
	}

	body := map[string]interface{}{
		"ticketCount": 2,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/inventory/reserve", bytes.NewReader(bodyBytes))
	rr := httptest.NewRecorder()

	s.handleReserve(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for missing eventId, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestCommitRequestMissingReservationID(t *testing.T) {
	s := &Server{
		db:      nil,
		lockMgr: nil,
	}

	body := map[string]interface{}{
		"bookingId": "bk-123",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/inventory/commit", bytes.NewReader(bodyBytes))
	rr := httptest.NewRecorder()

	s.handleCommit(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for missing reservationId, got %d", http.StatusBadRequest, rr.Code)
	}
}
