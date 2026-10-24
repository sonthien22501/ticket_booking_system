package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchFacetedFilterLogic(t *testing.T) {
	now := time.Now()
	sampleEvents := []Event{
		{
			ID:            "evt-1",
			Title:         "Neon Symphony 2026",
			Description:   "Cyberpunk electronic orchestra",
			Category:      "CONCERT",
			VenueName:     "Grand Hall",
			VenueLocation: "San Francisco, CA",
			StartTime:     now.Add(24 * time.Hour),
			Tiers: []TicketTier{
				{ID: "t-1", Name: "VIP", Price: 150.0},
				{ID: "t-2", Name: "General", Price: 60.0},
			},
		},
		{
			ID:            "evt-2",
			Title:         "Global Tech Summit",
			Description:   "Keynote conference for developers",
			Category:      "CONFERENCE",
			VenueName:     "Moscone Center",
			VenueLocation: "San Francisco, CA",
			StartTime:     now.Add(48 * time.Hour),
			Tiers: []TicketTier{
				{ID: "t-3", Name: "Pass", Price: 299.0},
			},
		},
		{
			ID:            "evt-3",
			Title:         "Hamilton: Musical",
			Description:   "Award-winning Broadway theater show",
			Category:      "THEATER",
			VenueName:     "Victoria Palace",
			VenueLocation: "London, UK",
			StartTime:     now.Add(72 * time.Hour),
			Tiers: []TicketTier{
				{ID: "t-4", Name: "Orchestra", Price: 85.0},
			},
		},
	}

	// 1. Test Text Keyword Search
	resQ := filterEventsInMemory(sampleEvents, SearchParams{Q: "Cyberpunk"})
	if len(resQ) != 1 || resQ[0].ID != "evt-1" {
		t.Errorf("expected 1 result for 'Cyberpunk', got %d", len(resQ))
	}

	// 2. Test City Filter
	resCity := filterEventsInMemory(sampleEvents, SearchParams{City: "San Francisco"})
	if len(resCity) != 2 {
		t.Errorf("expected 2 SF events, got %d", len(resCity))
	}

	// 3. Test Category Filter
	resCat := filterEventsInMemory(sampleEvents, SearchParams{Category: "THEATER"})
	if len(resCat) != 1 || resCat[0].ID != "evt-3" {
		t.Errorf("expected 1 THEATER event, got %d", len(resCat))
	}

	// 4. Test Price Range Filter
	resPrice := filterEventsInMemory(sampleEvents, SearchParams{MaxPrice: 100.0})
	if len(resPrice) != 2 { // evt-1 (starting $60) and evt-3 (starting $85)
		t.Errorf("expected 2 events with min tier <= $100, got %d", len(resPrice))
	}

	// 5. Test Facet Aggregation
	facets := computeSearchFacets(sampleEvents)
	if facets.Categories["CONCERT"] != 1 {
		t.Errorf("expected 1 CONCERT facet, got %d", facets.Categories["CONCERT"])
	}
	if facets.Cities["San Francisco"] != 2 {
		t.Errorf("expected 2 San Francisco facet count, got %d", facets.Cities["San Francisco"])
	}
	if facets.MinPrice != 60.0 {
		t.Errorf("expected min price 60.0, got %.2f", facets.MinPrice)
	}
	if facets.MaxPrice != 299.0 {
		t.Errorf("expected max price 299.0, got %.2f", facets.MaxPrice)
	}
}

func TestVenueFloorplanGeneration(t *testing.T) {
	evt := Event{
		ID:            "evt-101",
		Title:         "Neon Symphony",
		VenueName:     "Grand Arena",
		VenueLocation: "San Francisco, CA",
		Tiers: []TicketTier{
			{ID: "tier-1", Name: "VIP Stage Front", Price: 150.0, Capacity: 20},
			{ID: "tier-2", Name: "General Admission", Price: 60.0, Capacity: 48},
		},
	}

	floorplan := buildVenueFloorplan(&evt)
	if floorplan.EventID != "evt-101" {
		t.Errorf("expected floorplan eventId 'evt-101', got %s", floorplan.EventID)
	}
	if len(floorplan.Sections) < 2 {
		t.Fatalf("expected at least 2 floorplan sections, got %d", len(floorplan.Sections))
	}
	if floorplan.Sections[0].Category != "STAGE" && floorplan.Sections[0].Category != "VIP_PIT" {
		t.Errorf("expected VIP or STAGE section first, got %s", floorplan.Sections[0].Category)
	}
	if floorplan.TotalCapacity <= 0 {
		t.Errorf("expected positive total capacity, got %d", floorplan.TotalCapacity)
	}
}

func TestSearchHTTPHandlerFallback(t *testing.T) {
	srv := &Server{}

	// Test GET /events/search?city=San+Francisco
	req := httptest.NewRequest(http.MethodGet, "/events/search?city=San+Francisco", nil)
	w := httptest.NewRecorder()
	srv.handleSearchEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", w.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}

	if resp.Facets.Cities == nil {
		t.Errorf("expected cities facet map, got nil")
	}
}
