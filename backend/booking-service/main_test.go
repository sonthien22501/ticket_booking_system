package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBookingHealthDegradationOnPingFailure(t *testing.T) {
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

func TestBookingHandleMetrics(t *testing.T) {
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

	if res["service"] != "booking-service" {
		t.Errorf("expected service 'booking-service', got '%v'", res["service"])
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
	if res["service"] != "booking-service" {
		t.Errorf("expected service 'booking-service', got '%v'", res["service"])
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

func TestResolveAuthorizedBookingFilter_PreventsCrossUserEnumeration(t *testing.T) {
	// 1. Non-admin customer cannot enumerate other users' bookings
	customerClaims := &Claims{
		UserID: "usr-alice",
		Email:  "alice@test.com",
		Role:   "customer",
	}

	userID, email, isAdmin := resolveAuthorizedBookingFilter("usr-bob", "bob@test.com", "usr-bob", customerClaims)
	if isAdmin {
		t.Errorf("expected isAdmin=false for customer claims")
	}
	if userID != "usr-alice" {
		t.Errorf("expected userID to be clamped to caller's own userID 'usr-alice', got '%s'", userID)
	}
	if email == "bob@test.com" {
		t.Errorf("expected email 'bob@test.com' to be prevented from cross-user enumeration, got '%s'", email)
	}

	// 2. Non-admin customer querying without query params gets their own userID and empty email
	userID, email, isAdmin = resolveAuthorizedBookingFilter("", "", "", customerClaims)
	if isAdmin {
		t.Errorf("expected isAdmin=false for customer claims")
	}
	if userID != "usr-alice" {
		t.Errorf("expected userID to be caller's own userID 'usr-alice', got '%s'", userID)
	}

	// 3. Admin user can query any user or email
	adminClaims := &Claims{
		UserID: "admin-1",
		Email:  "admin@test.com",
		Role:   "admin",
	}

	userID, email, isAdmin = resolveAuthorizedBookingFilter("usr-bob", "bob@test.com", "", adminClaims)
	if !isAdmin {
		t.Errorf("expected isAdmin=true for admin claims")
	}
	if userID != "usr-bob" {
		t.Errorf("expected admin to query 'usr-bob', got '%s'", userID)
	}
	if email != "bob@test.com" {
		t.Errorf("expected admin to query 'bob@test.com', got '%s'", email)
	}

	// 4. Admin query with no filters
	userID, email, isAdmin = resolveAuthorizedBookingFilter("", "", "", adminClaims)
	if !isAdmin {
		t.Errorf("expected isAdmin=true for admin claims")
	}
	if userID != "" || email != "" {
		t.Errorf("expected empty user/email filters for admin broad list, got userID='%s', email='%s'", userID, email)
	}

	// 5. Unauthenticated request strips conflicting spoofed header
	userID, email, isAdmin = resolveAuthorizedBookingFilter("usr-charlie", "charlie@test.com", "usr-spoofed", nil)
	if isAdmin {
		t.Errorf("expected isAdmin=false for unauthenticated request")
	}
	if userID != "usr-charlie" {
		t.Errorf("expected explicit queryParam 'usr-charlie' to take precedence over spoofed header, got '%s'", userID)
	}
	if email != "charlie@test.com" {
		t.Errorf("expected email to be 'charlie@test.com', got '%s'", email)
	}
}

func TestBookingOpenAPISpec(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rr := httptest.NewRecorder()
	s.handleOpenAPISpec(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}
	var spec map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode openapi json: %v", err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi '3.0.3', got '%v'", spec["openapi"])
	}
}
