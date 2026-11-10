package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestConciergeRuntimeLLMCallSuccess(t *testing.T) {
	// 1. Mock LLM Server simulating Gemini JSON output
	mockLLMServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		geminiResponse := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"text": "```json\n" +
									"{\n" +
									`  "headline": "AI VIP Concierge: Neon Symphony Experience in Berlin",` + "\n" +
									`  "weatherAdvisory": "Clear skies anticipated; ideal terrace transit entry.",` + "\n" +
									`  "recommendedTier": "VIP Lounge & Front Stage Tier",` + "\n" +
									`  "recommendedAddonName": "Fast-Track Priority Parking Pass",` + "\n" +
									`  "recommendedAddonId": "addon-parking-priority",` + "\n" +
									`  "localizedBudgetNote": "180.00 EUR aligns precisely with VIP orchestra allocation.",` + "\n" +
									`  "convertedPrice": 180.0,` + "\n" +
									`  "currency": "EUR",` + "\n" +
									`  "suggestedVoucher": "EARLYBIRD20",` + "\n" +
									`  "insiderTips": [` + "\n" +
									`    "Dedicated fast-track entrance via West Gate.",` + "\n" +
									`    "Complimentary acoustic earplugs available at lounge concierge."` + "\n" +
									`  ]` + "\n" +
									"}\n" +
									"```",
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(geminiResponse)
	}))
	defer mockLLMServer.Close()

	// Configure environment to point to mock LLM server
	os.Setenv("LLM_API_URL", mockLLMServer.URL)
	os.Setenv("GEMINI_API_KEY", "mock-gemini-key")
	defer os.Unsetenv("LLM_API_URL")
	defer os.Unsetenv("GEMINI_API_KEY")

	s := &Server{
		httpClient: &http.Client{Timeout: 3500 * time.Millisecond},
	}

	payload := map[string]interface{}{
		"EventID":     "evt-101",
		"prompt":      "I want an electric concert experience in Berlin",
		"budget":      180,
		"currency":    "EUR",
		"preferences": "vip, parking",
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/concierge/advise", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handleConciergeAdvise(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp ConciergeAdviceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.EventID != "evt-101" {
		t.Errorf("expected EventID 'evt-101', got '%s'", resp.EventID)
	}
	if resp.FallbackUsed {
		t.Errorf("expected FallbackUsed=false on successful LLM call, got true")
	}
	if resp.Engine != "gemini-2.5-flash-runtime" {
		t.Errorf("expected engine 'gemini-2.5-flash-runtime', got '%s'", resp.Engine)
	}
	if resp.RecommendedTier != "VIP Lounge & Front Stage Tier" {
		t.Errorf("expected LLM recommendedTier, got '%s'", resp.RecommendedTier)
	}
	if resp.Currency != "EUR" {
		t.Errorf("expected currency 'EUR', got '%s'", resp.Currency)
	}
	if resp.ConvertedPrice != 180.0 {
		t.Errorf("expected ConvertedPrice 180.0, got %f", resp.ConvertedPrice)
	}
}

func TestConciergeRuntimeLLMFallbackOnTimeoutOrError(t *testing.T) {
	// Mock error LLM Server returning 500 Internal Server Error
	errorLLMServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream LLM quota exhausted", http.StatusInternalServerError)
	}))
	defer errorLLMServer.Close()

	os.Setenv("LLM_API_URL", errorLLMServer.URL)
	os.Setenv("GEMINI_API_KEY", "mock-gemini-key")
	defer os.Unsetenv("LLM_API_URL")
	defer os.Unsetenv("GEMINI_API_KEY")

	s := &Server{
		httpClient: &http.Client{Timeout: 3500 * time.Millisecond},
	}

	payload := map[string]interface{}{
		"EventID":  "evt-101",
		"prompt":   "Electric concert in Berlin",
		"budget":   180,
		"currency": "EUR",
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/concierge/advise", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handleConciergeAdvise(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 graceful fallback, got %d", rr.Code)
	}

	var resp ConciergeAdviceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.EventID != "evt-101" {
		t.Errorf("expected EventID 'evt-101', got '%s'", resp.EventID)
	}
	if !resp.FallbackUsed {
		t.Errorf("expected FallbackUsed=true when LLM returns 500 error, got false")
	}
	if resp.Engine != "runtime-heuristic-fallback" {
		t.Errorf("expected engine 'runtime-heuristic-fallback', got '%s'", resp.Engine)
	}
	if resp.RecommendedTier == "" {
		t.Errorf("expected non-empty recommendedTier from fallback")
	}
}
