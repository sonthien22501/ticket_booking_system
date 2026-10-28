package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthHealthDegradationOnPingFailure(t *testing.T) {
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

func TestAuthHandleHealth(t *testing.T) {
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
	if res["service"] != "auth-service" {
		t.Errorf("expected service 'auth-service', got '%v'", res["service"])
	}
}

func TestAuthHandleMetrics(t *testing.T) {
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

	if res["service"] != "auth-service" {
		t.Errorf("expected service 'auth-service', got '%v'", res["service"])
	}
	if _, ok := res["memory"]; !ok {
		t.Errorf("expected memory section in metrics")
	}
	if _, ok := res["connection_pool"]; !ok {
		t.Errorf("expected connection_pool section in metrics")
	}
}

func TestSanitizeRegistrationRole_PreventsPublicAdminEscalation(t *testing.T) {
	secret := "secret123"

	// Public registration attempting to self-escalate to admin without setup token must become customer
	if role := sanitizeRegistrationRole("admin", "", secret); role != "customer" {
		t.Errorf("expected role 'customer' for unauthenticated admin registration, got '%s'", role)
	}

	// Registration with wrong setup token must become customer
	if role := sanitizeRegistrationRole("admin", "wrong-secret", secret); role != "customer" {
		t.Errorf("expected role 'customer' for incorrect admin setup token, got '%s'", role)
	}

	// Normal customer registration stays customer
	if role := sanitizeRegistrationRole("customer", "", secret); role != "customer" {
		t.Errorf("expected role 'customer' for customer registration, got '%s'", role)
	}

	// Registration with valid admin setup token receives admin role
	if role := sanitizeRegistrationRole("admin", secret, secret); role != "admin" {
		t.Errorf("expected role 'admin' when valid setup token provided, got '%s'", role)
	}
}
