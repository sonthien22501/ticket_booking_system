package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchFrankfurterRates_SuccessAndFallback(t *testing.T) {
	// 1. Success mock
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"amount": 1.0,
			"base":   "USD",
			"date":   "2026-10-27",
			"rates": map[string]float64{
				"EUR": 0.925,
				"GBP": 0.775,
				"JPY": 154.2,
				"CAD": 1.390,
				"AUD": 1.530,
			},
		})
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 2 * time.Second}

	rates, fallbackUsed := fetchFrankfurterRates(ctx, client, mockServer.URL)
	if fallbackUsed {
		t.Errorf("expected fallbackUsed to be false for successful mock server")
	}
	if rates["EUR"] != 0.925 {
		t.Errorf("expected EUR rate 0.925, got %f", rates["EUR"])
	}
	if rates["JPY"] != 154.2 {
		t.Errorf("expected JPY rate 154.2, got %f", rates["JPY"])
	}

	// 2. Error fallback mock
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer errorServer.Close()

	fallbackRates, fallback := fetchFrankfurterRates(ctx, client, errorServer.URL)
	if !fallback {
		t.Errorf("expected fallback to be true on 503 Service Unavailable")
	}
	if fallbackRates["EUR"] <= 0 {
		t.Errorf("expected positive static fallback rate for EUR, got %f", fallbackRates["EUR"])
	}
	if fallbackRates["USD"] != 1.0 {
		t.Errorf("expected USD rate to be 1.0, got %f", fallbackRates["USD"])
	}
}

func TestConciergeAdvise_ValidationError(t *testing.T) {
	s := &Server{}

	// 1. Missing eventId
	reqPayload := ConciergeAdviceRequest{
		EventID:    "",
		EventTitle: "Concert",
	}
	payloadBytes, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/concierge/advise", bytes.NewReader(payloadBytes))
	rr := httptest.NewRecorder()

	s.handleConciergeAdvise(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for missing eventId, got %d", rr.Code)
	}

	var errResp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to parse validation error response: %v", err)
	}
	if errResp["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected code 'VALIDATION_ERROR', got %v", errResp["code"])
	}

	// 2. Malformed JSON
	reqBad := httptest.NewRequest(http.MethodPost, "/api/concierge/advise", bytes.NewReader([]byte("{invalid-json")))
	rrBad := httptest.NewRecorder()
	s.handleConciergeAdvise(rrBad, reqBad)
	if rrBad.Code != http.StatusBadRequest {
		t.Errorf("expected HTTP 400 for malformed json, got %d", rrBad.Code)
	}
}

func TestConciergeAdvise_CompositionAndHeuristicEngine(t *testing.T) {
	s := &Server{}

	reqPayload := ConciergeAdviceRequest{
		EventID:          "evt-101",
		EventTitle:       "Neon Symphony 2026",
		Category:         "CONCERT",
		VenueName:        "Grand Hall",
		VenueLocation:    "San Francisco, CA",
		Currency:         "USD",
		BudgetMax:        250.0,
		WeatherCondition: "Rainy",
		RainProbability:  80,
		Preferences:      "vip, parking",
	}
	payloadBytes, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/concierge/advise", bytes.NewReader(payloadBytes))
	rr := httptest.NewRecorder()

	s.handleConciergeAdvise(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ConciergeAdviceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode concierge response: %v", err)
	}

	if resp.EventID != "evt-101" {
		t.Errorf("expected eventId 'evt-101', got %s", resp.EventID)
	}
	if resp.Headline == "" {
		t.Errorf("expected headline to be populated")
	}
	if resp.RecommendedTier == "" {
		t.Errorf("expected recommended tier")
	}
	if resp.RecommendedAddonID == "" {
		t.Errorf("expected recommended addon ID")
	}
	if len(resp.InsiderTips) == 0 {
		t.Errorf("expected non-empty insider tips")
	}
	if len(resp.ComposedSources) == 0 {
		t.Errorf("expected non-empty composed sources")
	}
	if resp.Engine == "" {
		t.Errorf("expected engine identifier")
	}
}
