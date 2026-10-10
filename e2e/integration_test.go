package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

const gatewayURL = "http://localhost:8080/api"

func getBaseEvent(t *testing.T) string {
	resp, err := http.Get(gatewayURL + "/events")
	if err != nil {
		t.Fatalf("Failed to fetch events: %v", err)
	}
	defer resp.Body.Close()

	var events []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatalf("Failed to parse events: %v", err)
	}

	if len(events) == 0 {
		t.Fatalf("No events found")
	}

	return events[0]["id"].(string)
}

func getAvailableSeats(t *testing.T, eventID string) []string {
	resp, err := http.Get(gatewayURL + "/inventory/" + eventID + "/seats")
	if err != nil {
		t.Fatalf("Failed to fetch seats: %v", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	seatsRaw, ok := data["seats"].([]interface{})
	if !ok {
		t.Fatalf("Failed to parse seats array")
	}

	var available []string
	for _, sr := range seatsRaw {
		s := sr.(map[string]interface{})
		if s["status"] == "AVAILABLE" {
			available = append(available, s["id"].(string))
		}
	}
	return available
}

func TestBookingSagaIntegration(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 2 {
		t.Skip("Not enough available seats to test")
	}
	targetSeats := seats[0:2]

	payload := map[string]interface{}{
		"eventId":       eventID,
		"seats":         targetSeats,
		"customerName":  "Integration Test User",
		"customerEmail": "test@integration.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Booking request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	var bookingRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&bookingRes)

	bookingID := bookingRes["bookingId"].(string)
	t.Logf("Booking created successfully: %s", bookingID)

	resp2, err := http.Get(gatewayURL + "/bookings/" + bookingID)
	if err != nil {
		t.Fatalf("Failed to fetch booking: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp2.StatusCode)
	}
	
	t.Log("Integration test passed!")
}

func TestBookingConcurrency(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 1 {
		t.Skip("Not enough seats")
	}
	seatID := seats[len(seats)-1] // Pick from the end

	payload := map[string]interface{}{
		"eventId":       eventID,
		"seats":         []string{seatID},
		"customerName":  "Concurrency User",
		"customerEmail": "concurrent@test.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	const numRequests = 50
	var wg sync.WaitGroup
	wg.Add(numRequests)

	successCount := 0
	conflictCount := 0
	errorCount := 0

	var mu sync.Mutex

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer wg.Done()

			req, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(req)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errorCount++
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusCreated {
				successCount++
			} else if resp.StatusCode == http.StatusConflict {
				conflictCount++
			} else {
				errorCount++
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrency Test Results - Success: %d, Conflict: %d, Errors: %d", successCount, conflictCount, errorCount)

	if successCount != 1 {
		t.Errorf("Expected exactly 1 success, got %d", successCount)
	}
	if conflictCount != 49 {
		t.Errorf("Expected exactly 49 conflicts, got %d", conflictCount)
	}
	if errorCount > 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}
}

func TestSagaPaymentFailure(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 1 {
		t.Skip("Not enough seats")
	}
	seatID := seats[len(seats)-2]

	payload := map[string]interface{}{
		"eventId":       eventID,
		"seats":         []string{seatID},
		"customerName":  "Payment Fail User",
		"customerEmail": "fail@test.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_fail",
		},
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Booking request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("Expected status 409 Conflict due to payment failure, got %d", resp.StatusCode)
	}

	t.Log("Saga Payment Failure compensated correctly.")
}
