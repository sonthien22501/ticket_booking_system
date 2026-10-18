package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

type DynamicPricingResponse struct {
	EventID           string    `json:"eventId"`
	BasePrice         float64   `json:"basePrice"`
	CurrentPrice      float64   `json:"currentPrice"`
	SurgeMultiplier   float64   `json:"surgeMultiplier"`
	SurgeFactor       string    `json:"surgeFactor"`
	Capacity          int       `json:"capacity"`
	Available         int       `json:"available"`
	SoldPercentage    float64   `json:"soldPercentage"`
	VelocityPerMinute float64   `json:"velocityPerMinute"`
	Degraded          bool      `json:"degraded,omitempty"`
	CalculatedAt      time.Time `json:"calculatedAt"`
}

func TestSurgePricingCalculationAndExpositionE2E(t *testing.T) {
	// First get list of events to pick a valid event ID
	resp, err := http.Get(gatewayURL + "/events")
	if err != nil {
		t.Fatalf("Failed to fetch events from gateway: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for /events, got %d", resp.StatusCode)
	}

	var events []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil || len(events) == 0 {
		t.Fatalf("Failed to decode events list or empty events: %v", err)
	}

	targetEventID := events[0].ID

	// Query dynamic surge pricing for target event (R27)
	pricingURL := fmt.Sprintf("%s/events/%s/pricing", gatewayURL, targetEventID)
	pResp, err := http.Get(pricingURL)
	if err != nil {
		t.Fatalf("Failed to fetch event pricing from %s: %v", pricingURL, err)
	}
	defer pResp.Body.Close()

	if pResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for event pricing endpoint, got %d", pResp.StatusCode)
	}

	var pricing DynamicPricingResponse
	if err := json.NewDecoder(pResp.Body).Decode(&pricing); err != nil {
		t.Fatalf("Failed to decode pricing response: %v", err)
	}

	if pricing.EventID != targetEventID {
		t.Errorf("Expected eventId %s, got %s", targetEventID, pricing.EventID)
	}
	if pricing.BasePrice <= 0 {
		t.Errorf("Expected positive base price, got %.2f", pricing.BasePrice)
	}
	if pricing.SurgeMultiplier < 1.0 {
		t.Errorf("Expected surge multiplier >= 1.0, got %.2f", pricing.SurgeMultiplier)
	}
	if pricing.CurrentPrice <= 0 {
		t.Errorf("Expected positive current price, got %.2f", pricing.CurrentPrice)
	}
	if pricing.SurgeFactor == "" {
		t.Errorf("Expected non-empty surge factor label")
	}
}

func TestServiceIsolationAndFaultToleranceE2E(t *testing.T) {
	// Service Isolation & Fault Tolerance (R33):
	// Verifies that querying non-existent or degraded resources returns structured error/fallback
	// rather than crashing or causing cascading unhandled 500 panics.
	badURL := fmt.Sprintf("%s/events/non-existent-event-uuid-99999/pricing", gatewayURL)
	resp, err := http.Get(badURL)
	if err != nil {
		t.Fatalf("Request to bad URL failed unexpectedly: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected graceful response status (200 fallback or 404), got %d", resp.StatusCode)
	}

	// Verify that critical core services remain 100% operational (isolated from downstream failures)
	healthResp, err := http.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("Gateway health check failed: %v", err)
	}
	defer healthResp.Body.Close()

	if healthResp.StatusCode != http.StatusOK {
		t.Errorf("Expected gateway /health 200 OK, got %d", healthResp.StatusCode)
	}
}
