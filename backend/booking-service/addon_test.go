package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEventAddonsEndpoints(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		t.Skip("Postgres not available, skipping db integration test")
		return
	}

	// Ensure tables exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS event_addons (
			id VARCHAR(64) PRIMARY KEY,
			event_id VARCHAR(64) NOT NULL,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price NUMERIC(10,2) NOT NULL,
			category VARCHAR(64) NOT NULL,
			total_inventory INT NOT NULL DEFAULT 100,
			available_inventory INT NOT NULL DEFAULT 100,
			active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS booking_addons (
			id VARCHAR(64) PRIMARY KEY,
			booking_id VARCHAR(64) NOT NULL,
			addon_id VARCHAR(64) NOT NULL,
			name VARCHAR(255) NOT NULL,
			unit_price NUMERIC(10,2) NOT NULL,
			quantity INT NOT NULL DEFAULT 1,
			total_price NUMERIC(10,2) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		);
	`)
	if err != nil {
		t.Fatalf("failed to create test tables: %v", err)
	}

	// Insert test addon
	testAddonID := fmt.Sprintf("addon-test-%d", 101)
	_, _ = db.Exec(`
		INSERT INTO event_addons (id, event_id, name, description, price, category, total_inventory, available_inventory, active)
		VALUES ($1, 'evt-101', 'VIP Lounge Pass', 'Exclusive access to VIP lounge and open bar', 45.00, 'HOSPITALITY', 50, 50, true)
		ON CONFLICT (id) DO UPDATE SET available_inventory = 50, active = true
	`, testAddonID)

	s := &Server{db: db}

	// 1. Test GET /api/events/evt-101/addons
	req := httptest.NewRequest(http.MethodGet, "/api/events/evt-101/addons", nil)
	rr := httptest.NewRecorder()
	s.handleEventAddons(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get addons, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var addons []EventAddon
	if err := json.Unmarshal(rr.Body.Bytes(), &addons); err != nil {
		t.Fatalf("failed to unmarshal addons response: %v", err)
	}
	if len(addons) == 0 {
		t.Fatalf("expected at least 1 addon, got 0")
	}

	found := false
	for _, a := range addons {
		if a.ID == testAddonID {
			found = true
			if a.Price != 45.00 {
				t.Errorf("expected price 45.00, got %f", a.Price)
			}
			break
		}
	}
	if !found {
		t.Errorf("test addon %s not found in response", testAddonID)
	}

	// 2. Test Addon Inventory Deduction
	ctx := context.Background()
	bkgAddons, addonTotal, err := s.processBookingAddons(ctx, "evt-101", "bkg-test-123", []AddonItemRequest{
		{AddonID: testAddonID, Quantity: 2},
	})
	if err != nil {
		t.Fatalf("processBookingAddons failed: %v", err)
	}
	if addonTotal != 90.00 {
		t.Errorf("expected addonTotal 90.00, got %f", addonTotal)
	}
	if len(bkgAddons) != 1 || bkgAddons[0].Quantity != 2 {
		t.Errorf("expected 1 booking addon with quantity 2")
	}

	// 3. Test Addon Out of Stock Rejection
	_, _, err = s.processBookingAddons(ctx, "evt-101", "bkg-test-123", []AddonItemRequest{
		{AddonID: testAddonID, Quantity: 999},
	})
	if err == nil {
		t.Errorf("expected error when requesting 999 addons, but succeeded")
	}
}

func TestHandleAddonsValidation(t *testing.T) {
	s := &Server{db: nil}

	// Method not allowed on /api/addons
	req := httptest.NewRequest(http.MethodDelete, "/api/addons", nil)
	rr := httptest.NewRecorder()
	s.handleAddons(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// Bad Request on empty create payload
	req = httptest.NewRequest(http.MethodPost, "/api/addons", bytes.NewBuffer([]byte("{}")))
	rr = httptest.NewRecorder()
	s.handleAddons(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty addon payload, got %d", rr.Code)
	}
}
