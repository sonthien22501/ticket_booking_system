package main

import (
	"database/sql"
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

	var res map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if res["status"] != "UP" {
		t.Errorf("expected status 'UP', got '%v'", res["status"])
	}
	if res["service"] != "catalog-service" {
		t.Errorf("expected service 'catalog-service', got '%v'", res["service"])
	}
}

func TestHealthDegradationOnPingFailure(t *testing.T) {
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

func TestBuildPostgresDSNWithSchema(t *testing.T) {
	// 1. Without schema
	dsn := buildPostgresDSN("localhost", "5432", "postgres", "secret", "ticket_db", "")
	expectedBase := "host=localhost port=5432 user=postgres password=secret dbname=ticket_db sslmode=disable"
	if dsn != expectedBase {
		t.Errorf("expected dsn '%s', got '%s'", expectedBase, dsn)
	}

	// 2. With schema
	dsnWithSchema := buildPostgresDSN("postgres", "5432", "postgres", "secret", "ticket_db", "catalog_schema")
	expectedSchema := "host=postgres port=5432 user=postgres password=secret dbname=ticket_db sslmode=disable search_path=catalog_schema,public"
	if dsnWithSchema != expectedSchema {
		t.Errorf("expected dsn with schema '%s', got '%s'", expectedSchema, dsnWithSchema)
	}
}

func TestCatalogHandleMetrics(t *testing.T) {
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

	if res["service"] != "catalog-service" {
		t.Errorf("expected service 'catalog-service', got '%v'", res["service"])
	}
	if _, ok := res["memory"]; !ok {
		t.Errorf("expected memory section in metrics")
	}
	if _, ok := res["connection_pool"]; !ok {
		t.Errorf("expected connection_pool section in metrics")
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

func TestCatalogOpenAPISpec(t *testing.T) {
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
