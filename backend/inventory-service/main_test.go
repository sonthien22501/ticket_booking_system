package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInventoryHealthDegradationOnPingFailure(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://invalid:invalid@127.0.0.1:9999/nonexistent?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("failed to create dummy db: %v", err)
	}
	defer db.Close()

	s := &Server{db: db}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.handleHealth(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d on db failure, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if res["status"] != "DOWN" {
		t.Errorf("expected status 'DOWN', got '%v'", res["status"])
	}
}

func TestInventoryHandleMetrics(t *testing.T) {
	s := &Server{db: nil}
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	s.handleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if res["service"] != "seat-inventory-service" {
		t.Errorf("expected service 'seat-inventory-service', got '%v'", res["service"])
	}
	if _, ok := res["memory"]; !ok {
		t.Errorf("expected memory section in metrics")
	}
	if _, ok := res["connection_pool"]; !ok {
		t.Errorf("expected connection_pool section in metrics")
	}
}

func TestHandleHealth(t *testing.T) {
	s := &Server{db: nil}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if res["status"] != "UP" {
		t.Errorf("expected status 'UP', got '%v'", res["status"])
	}
	if res["service"] != "seat-inventory-service" {
		t.Errorf("expected service 'seat-inventory-service', got '%v'", res["service"])
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
