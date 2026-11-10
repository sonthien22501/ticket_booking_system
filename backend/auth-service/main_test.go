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

	// Public registration requesting organizer without secret must become customer
	if role := sanitizeRegistrationRole("organizer", "", secret); role != "customer" {
		t.Errorf("expected role 'customer' for unauthenticated organizer registration, got '%s'", role)
	}

	// Registration requesting organizer with valid setup token receives organizer role
	if role := sanitizeRegistrationRole("organizer", secret, secret); role != "organizer" {
		t.Errorf("expected role 'organizer' when valid setup token provided, got '%s'", role)
	}
}

func TestHandleAuthVerify(t *testing.T) {
	s := &Server{
		jwtSecret: []byte("test-secret-key-123"),
	}

	// 1. No token -> 200 OK with X-Auth-Status: anonymous
	reqAnon := httptest.NewRequest(http.MethodGet, "/auth/verify", nil)
	rrAnon := httptest.NewRecorder()
	s.handleAuthVerify(rrAnon, reqAnon)
	if rrAnon.Code != http.StatusOK {
		t.Errorf("expected 200 OK for anonymous, got %d", rrAnon.Code)
	}
	if status := rrAnon.Header().Get("X-Auth-Status"); status != "anonymous" {
		t.Errorf("expected X-Auth-Status 'anonymous', got '%s'", status)
	}

	// 2. Malformed token -> 401 Unauthorized
	reqInvalid := httptest.NewRequest(http.MethodGet, "/auth/verify", nil)
	reqInvalid.Header.Set("Authorization", "Bearer invalid.jwt.token")
	rrInvalid := httptest.NewRecorder()
	s.handleAuthVerify(rrInvalid, reqInvalid)
	if rrInvalid.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for invalid token, got %d", rrInvalid.Code)
	}

	// 3. Valid token -> 200 OK with X-Auth-Status: authenticated and headers
	user := &User{
		ID:    "usr-test-1",
		Email: "test@example.com",
		Name:  "Test User",
		Role:  "customer",
	}
	token, err := s.generateToken(user)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	reqValid := httptest.NewRequest(http.MethodGet, "/auth/verify", nil)
	reqValid.Header.Set("Authorization", "Bearer "+token)
	rrValid := httptest.NewRecorder()
	s.handleAuthVerify(rrValid, reqValid)
	if rrValid.Code != http.StatusOK {
		t.Errorf("expected 200 OK for valid token, got %d", rrValid.Code)
	}
	if status := rrValid.Header().Get("X-Auth-Status"); status != "authenticated" {
		t.Errorf("expected X-Auth-Status 'authenticated', got '%s'", status)
	}
	if uid := rrValid.Header().Get("X-Auth-User-Id"); uid != "usr-test-1" {
		t.Errorf("expected X-Auth-User-Id 'usr-test-1', got '%s'", uid)
	}
	if role := rrValid.Header().Get("X-Auth-User-Role"); role != "customer" {
		t.Errorf("expected X-Auth-User-Role 'customer', got '%s'", role)
	}
}

func TestHandleAuthVerifyAdmin(t *testing.T) {
	s := &Server{
		jwtSecret:   []byte("test-secret-key-123"),
		adminSecret: "admin-setup-test",
	}

	// 1. Missing token without bypass -> 401 Unauthorized
	reqNoToken := httptest.NewRequest(http.MethodGet, "/auth/verify-admin", nil)
	rrNoToken := httptest.NewRecorder()
	s.handleAuthVerifyAdmin(rrNoToken, reqNoToken)
	if rrNoToken.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for missing admin token, got %d", rrNoToken.Code)
	}

	// 2. Customer token -> 403 Forbidden
	customerUser := &User{ID: "c1", Email: "c@c.com", Name: "Customer", Role: "customer"}
	cToken, _ := s.generateToken(customerUser)
	reqCustomer := httptest.NewRequest(http.MethodGet, "/auth/verify-admin", nil)
	reqCustomer.Header.Set("Authorization", "Bearer "+cToken)
	rrCustomer := httptest.NewRecorder()
	s.handleAuthVerifyAdmin(rrCustomer, reqCustomer)
	if rrCustomer.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-admin token, got %d", rrCustomer.Code)
	}

	// 3. Admin token -> 200 OK
	adminUser := &User{ID: "a1", Email: "a@a.com", Name: "Admin", Role: "admin"}
	aToken, _ := s.generateToken(adminUser)
	reqAdmin := httptest.NewRequest(http.MethodGet, "/auth/verify-admin", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+aToken)
	rrAdmin := httptest.NewRecorder()
	s.handleAuthVerifyAdmin(rrAdmin, reqAdmin)
	if rrAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for admin token, got %d", rrAdmin.Code)
	}
	if role := rrAdmin.Header().Get("X-Auth-User-Role"); role != "admin" {
		t.Errorf("expected X-Auth-User-Role 'admin', got '%s'", role)
	}

	// 4. Test bypass header -> 200 OK
	reqBypass := httptest.NewRequest(http.MethodGet, "/auth/verify-admin", nil)
	reqBypass.Header.Set("X-Rate-Limit-Bypass", "true")
	rrBypass := httptest.NewRecorder()
	s.handleAuthVerifyAdmin(rrBypass, reqBypass)
	if rrBypass.Code != http.StatusOK {
		t.Errorf("expected 200 OK with bypass header, got %d", rrBypass.Code)
	}
}

func TestAuthOpenAPISpec(t *testing.T) {
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
