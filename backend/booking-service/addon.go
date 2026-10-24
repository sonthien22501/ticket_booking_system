package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type EventAddon struct {
	ID                 string    `json:"id"`
	EventID            string    `json:"eventId"`
	Name               string    `json:"name"`
	Description        string    `json:"description,omitempty"`
	Price              float64   `json:"price"`
	Category           string    `json:"category"` // PARKING, MERCHANDISE, HOSPITALITY, CONCESSIONS
	TotalInventory     int       `json:"totalInventory"`
	AvailableInventory int       `json:"availableInventory"`
	Active             bool      `json:"active"`
	CreatedAt          time.Time `json:"createdAt"`
}

type BookingAddon struct {
	ID         string    `json:"id"`
	BookingID  string    `json:"bookingId"`
	AddonID    string    `json:"addonId"`
	Name       string    `json:"name"`
	UnitPrice  float64   `json:"unitPrice"`
	Quantity   int       `json:"quantity"`
	TotalPrice float64   `json:"totalPrice"`
	CreatedAt  time.Time `json:"createdAt"`
}

type AddonItemRequest struct {
	AddonID  string `json:"addonId"`
	Quantity int    `json:"quantity"`
}

var (
	memAddonsMu sync.RWMutex
	memAddons   = make(map[string][]EventAddon)
)

func generateAddonID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("addon-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

func (s *Server) handleEventAddons(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Path: /api/events/{id}/addons or /events/{id}/addons
	path := strings.TrimPrefix(r.URL.Path, "/api/events/")
	path = strings.TrimPrefix(path, "/events/")
	parts := strings.Split(path, "/")
	eventID := parts[0]

	if eventID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "MISSING_EVENT_ID",
			"message": "Event ID is required",
		})
		return
	}

	addons := s.fetchAddonsForEvent(r.Context(), eventID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(addons)
}

func (s *Server) fetchAddonsForEvent(ctx context.Context, eventID string) []EventAddon {
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, event_id, name, COALESCE(description, ''), price, category, total_inventory, available_inventory, active, created_at
			FROM event_addons
			WHERE event_id = $1 AND active = true AND available_inventory > 0
			ORDER BY price ASC
		`, eventID)
		if err == nil {
			defer rows.Close()
			var list []EventAddon
			for rows.Next() {
				var a EventAddon
				if err := rows.Scan(&a.ID, &a.EventID, &a.Name, &a.Description, &a.Price, &a.Category, &a.TotalInventory, &a.AvailableInventory, &a.Active, &a.CreatedAt); err == nil {
					list = append(list, a)
				}
			}
			if len(list) > 0 {
				return list
			}
		}
	}

	// Memory fallback
	memAddonsMu.RLock()
	defer memAddonsMu.RUnlock()
	return memAddons[eventID]
}

func (s *Server) handleAddons(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var req struct {
			EventID        string  `json:"eventId"`
			Name           string  `json:"name"`
			Description    string  `json:"description"`
			Price          float64 `json:"price"`
			Category       string  `json:"category"`
			TotalInventory int     `json:"totalInventory"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EventID == "" || req.Name == "" || req.Price <= 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "VALIDATION_ERROR",
				"message": "eventId, name, and valid price > 0 required",
			})
			return
		}

		if req.Category == "" {
			req.Category = "MERCHANDISE"
		}
		if req.TotalInventory <= 0 {
			req.TotalInventory = 100
		}

		addonID := generateAddonID()
		now := time.Now().UTC()
		addon := EventAddon{
			ID:                 addonID,
			EventID:            req.EventID,
			Name:               req.Name,
			Description:        req.Description,
			Price:              req.Price,
			Category:           req.Category,
			TotalInventory:     req.TotalInventory,
			AvailableInventory: req.TotalInventory,
			Active:             true,
			CreatedAt:          now,
		}

		if s.db != nil {
			_, _ = s.db.ExecContext(r.Context(), `
				INSERT INTO event_addons (id, event_id, name, description, price, category, total_inventory, available_inventory, active, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`, addon.ID, addon.EventID, addon.Name, addon.Description, addon.Price, addon.Category, addon.TotalInventory, addon.AvailableInventory, addon.Active, addon.CreatedAt)
		}

		memAddonsMu.Lock()
		memAddons[req.EventID] = append(memAddons[req.EventID], addon)
		memAddonsMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(addon)

	case http.MethodGet:
		eventID := r.URL.Query().Get("eventId")
		addons := s.fetchAddonsForEvent(r.Context(), eventID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(addons)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) processBookingAddons(ctx context.Context, eventID, bookingID string, items []AddonItemRequest) ([]BookingAddon, float64, error) {
	if len(items) == 0 {
		return nil, 0, nil
	}

	var result []BookingAddon
	var total float64

	for _, item := range items {
		if item.Quantity <= 0 {
			continue
		}

		var name string
		var unitPrice float64
		var avail int

		if s.db != nil {
			err := s.db.QueryRowContext(ctx, `
				SELECT name, price, available_inventory
				FROM event_addons
				WHERE id = $1 AND event_id = $2 AND active = true
				FOR UPDATE
			`, item.AddonID, eventID).Scan(&name, &unitPrice, &avail)

			if err != nil {
				return nil, 0, fmt.Errorf("addon %s not found for event %s", item.AddonID, eventID)
			}

			if avail < item.Quantity {
				return nil, 0, fmt.Errorf("addon '%s' has insufficient stock (requested %d, available %d)", name, item.Quantity, avail)
			}

			// Decrement inventory
			_, _ = s.db.ExecContext(ctx, `
				UPDATE event_addons
				SET available_inventory = available_inventory - $1
				WHERE id = $2
			`, item.Quantity, item.AddonID)

			bkgAddonID := fmt.Sprintf("ba-%d-%s", time.Now().UnixNano(), item.AddonID)
			lineTotal := unitPrice * float64(item.Quantity)

			_, _ = s.db.ExecContext(ctx, `
				INSERT INTO booking_addons (id, booking_id, addon_id, name, unit_price, quantity, total_price, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			`, bkgAddonID, bookingID, item.AddonID, name, unitPrice, item.Quantity, lineTotal)

			bkgAddon := BookingAddon{
				ID:         bkgAddonID,
				BookingID:  bookingID,
				AddonID:    item.AddonID,
				Name:       name,
				UnitPrice:  unitPrice,
				Quantity:   item.Quantity,
				TotalPrice: lineTotal,
				CreatedAt:  time.Now().UTC(),
			}
			result = append(result, bkgAddon)
			total += lineTotal
		}
	}

	return result, total, nil
}

func (s *Server) fetchAddonsForBooking(ctx context.Context, bookingID string) []BookingAddon {
	if s.db == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, booking_id, addon_id, name, unit_price, quantity, total_price, created_at
		FROM booking_addons
		WHERE booking_id = $1
		ORDER BY created_at ASC
	`, bookingID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var list []BookingAddon
	for rows.Next() {
		var a BookingAddon
		if err := rows.Scan(&a.ID, &a.BookingID, &a.AddonID, &a.Name, &a.UnitPrice, &a.Quantity, &a.TotalPrice, &a.CreatedAt); err == nil {
			list = append(list, a)
		}
	}
	return list
}
