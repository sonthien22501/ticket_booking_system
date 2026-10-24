package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// SearchParams represents the multi-facet filter criteria
type SearchParams struct {
	Q        string  `json:"q"`
	City     string  `json:"city"`
	Category string  `json:"category"`
	MinPrice float64 `json:"minPrice"`
	MaxPrice float64 `json:"maxPrice"`
	SortBy   string  `json:"sortBy"` // date_asc, date_desc, price_asc, price_desc, title_asc
}

// SearchFacets stores aggregated counts across dimensions
type SearchFacets struct {
	Cities     map[string]int `json:"cities"`
	Categories map[string]int `json:"categories"`
	MinPrice   float64        `json:"minPrice"`
	MaxPrice   float64        `json:"maxPrice"`
}

// SearchResponse is returned by /api/events/search
type SearchResponse struct {
	Events     []Event      `json:"events"`
	Facets     SearchFacets `json:"facets"`
	TotalCount int          `json:"totalCount"`
}

// VenueFloorplanSection defines a spatial zone within the venue
type VenueFloorplanSection struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Category    string  `json:"category"` // STAGE, VIP_PIT, ORCHESTRA, MEZZANINE, BALCONY
	TierID      string  `json:"tierId,omitempty"`
	TierName    string  `json:"tierName,omitempty"`
	Capacity    int     `json:"capacity"`
	BasePrice   float64 `json:"basePrice"`
	RowRange    string  `json:"rowRange"`
	Color       string  `json:"color"`
	Coordinates string  `json:"coordinates"`
}

// VenueFloorplan represents the architectural layout for an event
type VenueFloorplan struct {
	EventID          string                  `json:"eventId"`
	VenueName        string                  `json:"venueName"`
	VenueLocation    string                  `json:"venueLocation"`
	StageOrientation string                  `json:"stageOrientation"` // NORTH, SOUTH
	TotalCapacity    int                     `json:"totalCapacity"`
	Sections         []VenueFloorplanSection `json:"sections"`
}

// extractCity extracts city name from location string (e.g., "San Francisco, CA" -> "San Francisco")
func extractCity(location string) string {
	parts := strings.Split(location, ",")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		return strings.TrimSpace(parts[0])
	}
	return "General"
}

// getEventLowestPrice finds lowest tier price for an event
func getEventLowestPrice(e *Event) float64 {
	if len(e.Tiers) == 0 {
		return 0.0
	}
	min := e.Tiers[0].Price
	for _, t := range e.Tiers[1:] {
		if t.Price < min {
			min = t.Price
		}
	}
	return min
}

// computeSearchFacets tallies categories, cities, and price bounds
func computeSearchFacets(events []Event) SearchFacets {
	categories := make(map[string]int)
	cities := make(map[string]int)
	var minPrice float64 = -1
	var maxPrice float64 = 0

	for _, e := range events {
		// Category tally
		if e.Category != "" {
			categories[e.Category]++
		}

		// City tally
		city := extractCity(e.VenueLocation)
		if city != "" {
			cities[city]++
		}

		// Price range tally
		for _, t := range e.Tiers {
			if minPrice < 0 || t.Price < minPrice {
				minPrice = t.Price
			}
			if t.Price > maxPrice {
				maxPrice = t.Price
			}
		}
	}

	if minPrice < 0 {
		minPrice = 0
	}

	return SearchFacets{
		Cities:     cities,
		Categories: categories,
		MinPrice:   minPrice,
		MaxPrice:   maxPrice,
	}
}

// filterEventsInMemory executes keyword and facet filtering in-memory
func filterEventsInMemory(events []Event, params SearchParams) []Event {
	var filtered []Event

	qLower := strings.ToLower(strings.TrimSpace(params.Q))
	cityLower := strings.ToLower(strings.TrimSpace(params.City))
	catUpper := strings.ToUpper(strings.TrimSpace(params.Category))

	for _, e := range events {
		// 1. Keyword search (title, description, venue, location)
		if qLower != "" {
			combined := strings.ToLower(fmt.Sprintf("%s %s %s %s", e.Title, e.Description, e.VenueName, e.VenueLocation))
			if !strings.Contains(combined, qLower) {
				continue
			}
		}

		// 2. City filter
		if cityLower != "" && cityLower != "all" {
			evtCity := strings.ToLower(extractCity(e.VenueLocation))
			if !strings.Contains(evtCity, cityLower) && !strings.Contains(strings.ToLower(e.VenueLocation), cityLower) {
				continue
			}
		}

		// 3. Category filter
		if catUpper != "" && catUpper != "ALL" {
			if strings.ToUpper(e.Category) != catUpper {
				continue
			}
		}

		// 4. Price range filter
		lowestPrice := getEventLowestPrice(&e)
		if params.MinPrice > 0 && lowestPrice < params.MinPrice {
			continue
		}
		if params.MaxPrice > 0 && lowestPrice > params.MaxPrice {
			continue
		}

		filtered = append(filtered, e)
	}

	// 5. Sorting
	switch params.SortBy {
	case "date_desc":
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].StartTime.After(filtered[j].StartTime)
		})
	case "price_asc":
		sort.Slice(filtered, func(i, j int) bool {
			return getEventLowestPrice(&filtered[i]) < getEventLowestPrice(&filtered[j])
		})
	case "price_desc":
		sort.Slice(filtered, func(i, j int) bool {
			return getEventLowestPrice(&filtered[i]) > getEventLowestPrice(&filtered[j])
		})
	case "title_asc":
		sort.Slice(filtered, func(i, j int) bool {
			return strings.ToLower(filtered[i].Title) < strings.ToLower(filtered[j].Title)
		})
	case "date_asc":
		fallthrough
	default:
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].StartTime.Before(filtered[j].StartTime)
		})
	}

	if filtered == nil {
		filtered = []Event{}
	}
	return filtered
}

// buildVenueFloorplan synthesizes spatial sections and seat rows
func buildVenueFloorplan(e *Event) VenueFloorplan {
	sections := []VenueFloorplanSection{}
	totalCap := 0

	// 1. Stage section (focal point)
	sections = append(sections, VenueFloorplanSection{
		ID:          "sec-stage",
		Name:        "Main Center Stage",
		Category:    "STAGE",
		Capacity:    0,
		BasePrice:   0,
		RowRange:    "Center Podium",
		Color:       "#6366f1",
		Coordinates: "0,0,100,20",
	})

	if len(e.Tiers) == 0 {
		// Default standard sections
		sections = append(sections,
			VenueFloorplanSection{
				ID:          "sec-vip",
				Name:        "VIP Floor Front",
				Category:    "VIP_PIT",
				Capacity:    20,
				BasePrice:   150.0,
				RowRange:    "Rows A - B",
				Color:       "#f59e0b",
				Coordinates: "10,25,80,30",
			},
			VenueFloorplanSection{
				ID:          "sec-orch",
				Name:        "Orchestra Center",
				Category:    "ORCHESTRA",
				Capacity:    48,
				BasePrice:   60.0,
				RowRange:    "Rows C - F",
				Color:       "#3b82f6",
				Coordinates: "5,60,90,40",
			},
		)
		totalCap = 68
	} else {
		colors := []string{"#f59e0b", "#3b82f6", "#10b981", "#8b5cf6", "#ec4899"}
		categories := []string{"VIP_PIT", "ORCHESTRA", "MEZZANINE", "BALCONY"}

		for i, tier := range e.Tiers {
			secCategory := categories[i%len(categories)]
			color := colors[i%len(colors)]
			cap := tier.Capacity
			if cap <= 0 {
				cap = 30
			}
			totalCap += cap

			rowRange := fmt.Sprintf("Tier %d (Rows %c - %c)", i+1, 'A'+i*2, 'B'+i*2)

			sections = append(sections, VenueFloorplanSection{
				ID:          fmt.Sprintf("sec-%s", tier.ID),
				Name:        tier.Name,
				Category:    secCategory,
				TierID:      tier.ID,
				TierName:    tier.Name,
				Capacity:    cap,
				BasePrice:   tier.Price,
				RowRange:    rowRange,
				Color:       color,
				Coordinates: fmt.Sprintf("%d,%d,%d,%d", 10+i*5, 25+i*30, 80-i*5, 25),
			})
		}
	}

	return VenueFloorplan{
		EventID:          e.ID,
		VenueName:        e.VenueName,
		VenueLocation:    e.VenueLocation,
		StageOrientation: "NORTH",
		TotalCapacity:    totalCap,
		Sections:         sections,
	}
}

// handleSearchEvents processes GET /api/events/search and /events/search
func (s *Server) handleSearchEvents(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	q := r.URL.Query().Get("q")
	city := r.URL.Query().Get("city")
	category := r.URL.Query().Get("category")
	sortBy := r.URL.Query().Get("sort_by")

	var minPrice, maxPrice float64
	if minStr := r.URL.Query().Get("min_price"); minStr != "" {
		minPrice, _ = strconv.ParseFloat(minStr, 64)
	}
	if maxStr := r.URL.Query().Get("max_price"); maxStr != "" {
		maxPrice, _ = strconv.ParseFloat(maxStr, 64)
	}

	params := SearchParams{
		Q:        q,
		City:     city,
		Category: category,
		MinPrice: minPrice,
		MaxPrice: maxPrice,
		SortBy:   sortBy,
	}

	allEvents := s.fetchAllEventsFromDBOrMock(r.Context())
	facets := computeSearchFacets(allEvents)
	filtered := filterEventsInMemory(allEvents, params)

	resp := SearchResponse{
		Events:     filtered,
		Facets:     facets,
		TotalCount: len(filtered),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleVenueFloorplan processes GET /api/events/{id}/floorplan
func (s *Server) handleVenueFloorplan(w http.ResponseWriter, r *http.Request, eventID string) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	allEvents := s.fetchAllEventsFromDBOrMock(r.Context())
	var targetEvent *Event
	for _, e := range allEvents {
		if e.ID == eventID {
			targetEvent = &e
			break
		}
	}

	if targetEvent == nil {
		// Mock event for floorplan preview if not found
		targetEvent = &Event{
			ID:            eventID,
			Title:         "Event Arena Floorplan",
			VenueName:     "Grand Arena Stadium",
			VenueLocation: "San Francisco, CA",
		}
	}

	floorplan := buildVenueFloorplan(targetEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(floorplan)
}

// fetchAllEventsFromDBOrMock returns full event list for search aggregation
func (s *Server) fetchAllEventsFromDBOrMock(ctx context.Context) []Event {
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, title, description, category, venue_name, venue_location,
			       start_time, end_time, status, image_url, created_at, updated_at
			FROM events
			ORDER BY start_time ASC
		`)
		if err == nil {
			defer rows.Close()
			var events []Event
			for rows.Next() {
				var e Event
				var desc, img sql.NullString
				if err := rows.Scan(
					&e.ID, &e.Title, &desc, &e.Category,
					&e.VenueName, &e.VenueLocation,
					&e.StartTime, &e.EndTime,
					&e.Status, &img,
					&e.CreatedAt, &e.UpdatedAt,
				); err == nil {
					if desc.Valid {
						e.Description = desc.String
					}
					if img.Valid {
						e.ImageURL = img.String
					}
					e.EventID = e.ID
					e.Tiers = s.fetchTiersForEvent(e.ID)
					events = append(events, e)
				}
			}
			if len(events) > 0 {
				return events
			}
		}
	}

	// Mock fallback events if DB is disconnected
	now := time.Now()
	return []Event{
		{
			ID:            "evt-101",
			Title:         "Neon Symphony 2026",
			Description:   "A breathtaking fusion of classical strings and analog synthesizers.",
			Category:      "CONCERT",
			VenueName:     "Grand Hall",
			VenueLocation: "San Francisco, CA",
			StartTime:     now.Add(24 * time.Hour),
			EndTime:       now.Add(27 * time.Hour),
			Status:        "ACTIVE",
			Tiers: []TicketTier{
				{ID: "tier-vip-101", Name: "VIP Stage Front", Price: 150.0, Capacity: 20},
				{ID: "tier-gen-101", Name: "General Admission", Price: 60.0, Capacity: 48},
			},
		},
		{
			ID:            "evt-102",
			Title:         "Global Tech Summit 2026",
			Description:   "The world's premier gathering of software architects and AI innovators.",
			Category:      "CONFERENCE",
			VenueName:     "Moscone Center",
			VenueLocation: "San Francisco, CA",
			StartTime:     now.Add(72 * time.Hour),
			EndTime:       now.Add(80 * time.Hour),
			Status:        "ACTIVE",
			Tiers: []TicketTier{
				{ID: "tier-keynote-102", Name: "Full Conference Pass", Price: 299.0, Capacity: 50},
			},
		},
		{
			ID:            "evt-103",
			Title:         "UEFA Champions Gala Match: Berlin vs Madrid",
			Description:   "Championship match with live broadcast and trophy presentation.",
			Category:      "SPORTS",
			VenueName:     "Olympiastadion",
			VenueLocation: "Berlin, Germany",
			StartTime:     now.Add(120 * time.Hour),
			EndTime:       now.Add(123 * time.Hour),
			Status:        "ACTIVE",
			Tiers: []TicketTier{
				{ID: "tier-stands-103", Name: "Grandstand Upper", Price: 95.0, Capacity: 100},
			},
		},
	}
}
