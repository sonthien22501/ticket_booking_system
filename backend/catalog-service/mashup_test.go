package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResolveCityCoords(t *testing.T) {
	tests := []struct {
		name         string
		location     string
		expectedCity string
		expectedLat  float64
		expectedLon  float64
	}{
		{
			name:         "San Francisco with state",
			location:     "San Francisco, CA",
			expectedCity: "San Francisco",
			expectedLat:  37.7749,
			expectedLon:  -122.4194,
		},
		{
			name:         "New York with state",
			location:     "New York, NY",
			expectedCity: "New York",
			expectedLat:  40.7128,
			expectedLon:  -74.0060,
		},
		{
			name:         "London with country",
			location:     "London, UK",
			expectedCity: "London",
			expectedLat:  51.5074,
			expectedLon:  -0.1278,
		},
		{
			name:         "Tokyo plain",
			location:     "Tokyo, Japan",
			expectedCity: "Tokyo",
			expectedLat:  35.6762,
			expectedLon:  139.6503,
		},
		{
			name:         "Unknown city fallback",
			location:     "Atlantis Deep Sea",
			expectedCity: "Atlantis Deep Sea",
			expectedLat:  40.7128,
			expectedLon:  -74.0060,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			city, lat, lon := resolveCityCoords(tt.location)
			if city != tt.expectedCity {
				t.Errorf("expected city %q, got %q", tt.expectedCity, city)
			}
			if lat != tt.expectedLat || lon != tt.expectedLon {
				t.Errorf("expected coords (%f, %f), got (%f, %f)", tt.expectedLat, tt.expectedLon, lat, lon)
			}
		})
	}
}

func TestFetchOpenMeteoWeather_LiveAndFallback(t *testing.T) {
	// 1. Success mock
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"current": map[string]interface{}{
				"temperature_2m": 19.5,
				"weather_code":   0,
				"wind_speed_10m": 14.2,
			},
			"hourly": map[string]interface{}{
				"precipitation_probability": []int{5, 10, 15},
			},
		})
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 2 * time.Second}

	weather, fallback := fetchOpenMeteoWeather(ctx, client, mockServer.URL, 37.7749, -122.4194)
	if fallback {
		t.Errorf("expected fallback to be false for successful API call")
	}
	if weather.TemperatureC != 19.5 {
		t.Errorf("expected temperature 19.5, got %f", weather.TemperatureC)
	}
	if !weather.IsOutdoorFriendly {
		t.Errorf("expected outdoor friendly to be true for code 0")
	}
	if weather.Source != "open-meteo" {
		t.Errorf("expected source 'open-meteo', got %s", weather.Source)
	}

	// 2. Error fallback mock
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	fallbackWeather, fallbackUsed := fetchOpenMeteoWeather(ctx, client, errorServer.URL, 37.7749, -122.4194)
	if !fallbackUsed {
		t.Errorf("expected fallback to be true on error response")
	}
	if fallbackWeather.Source != "fallback-static" {
		t.Errorf("expected source 'fallback-static', got %s", fallbackWeather.Source)
	}
	if fallbackWeather.TemperatureC == 0 {
		t.Errorf("expected non-zero fallback temperature")
	}
}

func TestFetchITunesPreview_LiveAndFallback(t *testing.T) {
	// 1. Success mock
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"resultCount": 1,
			"results": []map[string]interface{}{
				{
					"trackName":        "Midnight City",
					"artistName":       "M83",
					"collectionName":   "Hurry Up, We're Dreaming",
					"previewUrl":       "https://audio-ssl.itunes.apple.com/preview.m4a",
					"artworkUrl100":    "https://is1-ssl.mzstatic.com/image.jpg",
					"primaryGenreName": "Electronic",
					"releaseDate":      "2011-10-18T07:00:00Z",
				},
			},
		})
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client := &http.Client{Timeout: 2 * time.Second}

	track, fallback := fetchITunesPreview(ctx, client, mockServer.URL, "M83 Midnight")
	if fallback {
		t.Errorf("expected fallback to be false for successful iTunes call")
	}
	if track.TrackName != "Midnight City" {
		t.Errorf("expected track 'Midnight City', got %q", track.TrackName)
	}
	if track.ArtistName != "M83" {
		t.Errorf("expected artist 'M83', got %q", track.ArtistName)
	}
	if track.PreviewURL != "https://audio-ssl.itunes.apple.com/preview.m4a" {
		t.Errorf("unexpected previewUrl %s", track.PreviewURL)
	}

	// 2. Error / empty fallback mock
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer errorServer.Close()

	fallbackTrack, fallbackUsed := fetchITunesPreview(ctx, client, errorServer.URL, "Nonexistent Band")
	if !fallbackUsed {
		t.Errorf("expected fallback to be true on iTunes error")
	}
	if fallbackTrack.Source != "fallback-static" {
		t.Errorf("expected source 'fallback-static', got %s", fallbackTrack.Source)
	}
	if fallbackTrack.PreviewURL == "" {
		t.Errorf("expected fallback preview URL to be non-empty")
	}
}

func TestOpenAPISpecAndDocsEndpoints(t *testing.T) {
	s := &Server{db: nil}

	// 1. GET /api/openapi.json
	reqJSON := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	rrJSON := httptest.NewRecorder()
	s.handleOpenAPISpec(rrJSON, reqJSON)

	if rrJSON.Code != http.StatusOK {
		t.Fatalf("expected 200 for openapi.json, got %d", rrJSON.Code)
	}
	var spec map[string]interface{}
	if err := json.NewDecoder(rrJSON.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode OpenAPI JSON: %v", err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi 3.0.3, got %v", spec["openapi"])
	}
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing paths in openapi spec")
	}
	expectedPaths := []string{
		"/api/events",
		"/api/events/{id}/mashup",
		"/api/bookings",
		"/api/currencies/rates",
		"/api/concierge/advise",
	}
	for _, ep := range expectedPaths {
		if _, exists := paths[ep]; !exists {
			t.Errorf("expected path %q in OpenAPI spec", ep)
		}
	}

	// 2. GET /api/docs
	reqDocs := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rrDocs := httptest.NewRecorder()
	s.handleSwaggerDocs(rrDocs, reqDocs)

	if rrDocs.Code != http.StatusOK {
		t.Fatalf("expected 200 for docs, got %d", rrDocs.Code)
	}
	body := rrDocs.Body.String()
	if !strings.Contains(body, "swagger-ui") {
		t.Errorf("expected swagger-ui in docs HTML")
	}
	if !strings.Contains(body, "/api/openapi.json") {
		t.Errorf("expected reference to /api/openapi.json in docs HTML")
	}
}

func TestHandleEventMashup_Endpoint(t *testing.T) {
	s := &Server{db: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/events/evt-101/mashup", nil)
	rr := httptest.NewRecorder()

	s.handleEventMashup(rr, req, "evt-101")

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for mashup, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp EventMashupResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode mashup response: %v", err)
	}
	if resp.EventID != "evt-101" {
		t.Errorf("expected eventId 'evt-101', got %q", resp.EventID)
	}
	if resp.Weather.Condition == "" {
		t.Errorf("expected weather condition to be populated")
	}
	if resp.MediaPreview.TrackName == "" {
		t.Errorf("expected media preview track name to be populated")
	}
}
