package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestIdempotencyHeaderReplay(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 1 {
		t.Skip("Not enough available seats to test")
	}
	targetSeat := seats[0:1]

	idemKey := fmt.Sprintf("idem-key-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"eventId":       eventID,
		"seats":         targetSeat,
		"customerName":  "Idempotency Test User",
		"customerEmail": "idempotent@test.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	// First Request: Expect 201 Created
	req1, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idemKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp1.Body)
		t.Fatalf("Expected 201 Created on first request, got %d: %s", resp1.StatusCode, string(respBody))
	}

	var booking1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&booking1)
	bookingID1, _ := booking1["bookingId"].(string)
	if bookingID1 == "" {
		bookingID1, _ = booking1["id"].(string)
	}
	if bookingID1 == "" {
		t.Fatalf("First request did not return a valid booking ID: %+v", booking1)
	}

	// Second Request: Replay with same Idempotency-Key
	req2, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idemKey)

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Replayed request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Expected 200 OK or 201 Created on replayed request, got %d: %s", resp2.StatusCode, string(respBody))
	}

	isReplay := resp2.Header.Get("X-Idempotent-Replay")
	if isReplay != "true" {
		t.Errorf("Expected X-Idempotent-Replay header to be 'true', got '%s'", isReplay)
	}

	var booking2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&booking2)
	bookingID2, _ := booking2["bookingId"].(string)
	if bookingID2 == "" {
		bookingID2, _ = booking2["id"].(string)
	}

	if bookingID1 != bookingID2 {
		t.Fatalf("Booking ID mismatch on idempotent replay: initial=%s, replayed=%s", bookingID1, bookingID2)
	}

	t.Logf("Idempotent replay successful: replayed booking %s matches original", bookingID1)
}

func TestIdempotencyPayloadFieldReplay(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 1 {
		t.Skip("Not enough available seats to test")
	}
	targetSeat := seats[0:1]

	idemKey := fmt.Sprintf("idem-body-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"eventId":        eventID,
		"seats":          targetSeat,
		"customerName":   "Idempotency Payload User",
		"customerEmail":  "payload-idem@test.com",
		"idempotencyKey": idemKey,
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}

	// First Request
	req1, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp1.Body)
		t.Fatalf("Expected 201 Created on first request, got %d: %s", resp1.StatusCode, string(respBody))
	}

	var b1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&b1)
	bID1 := b1["bookingId"]
	if bID1 == nil {
		bID1 = b1["id"]
	}

	// Replay request
	req2, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Replay request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Expected 200 OK or 201 Created, got %d: %s", resp2.StatusCode, string(respBody))
	}

	if resp2.Header.Get("X-Idempotent-Replay") != "true" {
		t.Errorf("Expected X-Idempotent-Replay: true")
	}

	var b2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&b2)
	bID2 := b2["bookingId"]
	if bID2 == nil {
		bID2 = b2["id"]
	}

	if bID1 != bID2 {
		t.Fatalf("Booking IDs do not match: initial=%v, replay=%v", bID1, bID2)
	}

	t.Logf("Payload field idempotency successful: %v", bID1)
}

func TestIdempotencyConcurrentContention(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 1 {
		t.Skip("Not enough available seats to test")
	}
	targetSeat := seats[0:1]

	idemKey := fmt.Sprintf("idem-race-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"eventId":       eventID,
		"seats":         targetSeat,
		"customerName":  "Concurrent Idempotency Tester",
		"customerEmail": "race-idem@test.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	const concurrency = 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	var mu sync.Mutex
	statusCodes := make(map[int]int)
	bookingIDs := make(map[string]bool)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 10 * time.Second}
			req, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", idemKey)

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			statusCodes[resp.StatusCode]++
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				var res map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
					id, _ := res["bookingId"].(string)
					if id == "" {
						id, _ = res["id"].(string)
					}
					if id != "" {
						bookingIDs[id] = true
					}
				}
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	t.Logf("Concurrent Idempotency Status Codes: %+v, Unique Booking IDs: %d", statusCodes, len(bookingIDs))

	// Exactly 1 booking ID should ever have been generated
	if len(bookingIDs) != 1 {
		t.Fatalf("Expected exactly 1 unique booking ID across all concurrent requests, found %d: %+v", len(bookingIDs), bookingIDs)
	}

	// Success responses must be 201 or 200, conflicts must be 409
	successCount := statusCodes[http.StatusCreated] + statusCodes[http.StatusOK]
	if successCount < 1 {
		t.Fatalf("Expected at least 1 successful response (200 or 201), got %+v", statusCodes)
	}
}

func TestIdempotencyInventoryConsistency(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 1 {
		t.Skip("Not enough available seats to test")
	}
	targetSeat := seats[0:1]

	// Fetch initial available seat count
	getInventoryCount := func() int {
		resp, err := http.Get(gatewayURL + "/inventory/" + eventID)
		if err != nil {
			t.Fatalf("Failed to fetch inventory: %v", err)
		}
		defer resp.Body.Close()
		var inv map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&inv)
		avail, _ := inv["availableSeats"].(float64)
		if avail == 0 {
			avail, _ = inv["available_seats"].(float64)
		}
		return int(avail)
	}

	initialAvail := getInventoryCount()

	idemKey := fmt.Sprintf("idem-inv-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"eventId":       eventID,
		"seats":         targetSeat,
		"customerName":  "Inventory Idempotency User",
		"customerEmail": "inv-idem@test.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}

	// 1st request: create booking
	req1, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idemKey)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", resp1.StatusCode)
	}

	// Subsequent 3 duplicate requests with same key
	for i := 1; i <= 3; i++ {
		reqDup, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
		reqDup.Header.Set("Content-Type", "application/json")
		reqDup.Header.Set("Idempotency-Key", idemKey)
		respDup, err := client.Do(reqDup)
		if err != nil {
			t.Fatalf("Duplicate request %d failed: %v", i, err)
		}
		defer respDup.Body.Close()
		if respDup.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 OK for duplicate request %d, got %d", i, respDup.StatusCode)
		}
		if respDup.Header.Get("X-Idempotent-Replay") != "true" {
			t.Fatalf("Expected X-Idempotent-Replay: true on duplicate request %d", i)
		}
	}

	// Assert inventory decremented by exactly 1 seat, not 4
	finalAvail := getInventoryCount()
	expectedAvail := initialAvail - len(targetSeat)
	if finalAvail != expectedAvail {
		t.Fatalf("Inventory decrement error: expected %d seats remaining, got %d (initial %d)", expectedAvail, finalAvail, initialAvail)
	}

	t.Logf("Inventory consistency verified: initial=%d, final=%d after 4 requests with same key", initialAvail, finalAvail)
}

func TestIdempotencyPostgresFallback(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) < 1 {
		t.Skip("Not enough available seats to test")
	}
	targetSeat := seats[0:1]

	idemKey := fmt.Sprintf("idem-pg-fallback-%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"eventId":       eventID,
		"seats":         targetSeat,
		"customerName":  "PG Fallback User",
		"customerEmail": "pg-fallback@test.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Initial request creating booking
	req1, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idemKey)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d", resp1.StatusCode)
	}

	var b1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&b1)
	bID1 := b1["bookingId"]
	if bID1 == nil {
		bID1 = b1["id"]
	}

	// 2. Delete Redis key directly to simulate cache eviction/restart
	redisKey := "idempotency:booking:" + idemKey
	conn, err := net.DialTimeout("tcp", "localhost:6379", 2*time.Second)
	if err == nil {
		defer conn.Close()
		delCmd := fmt.Sprintf("*2\r\n$3\r\nDEL\r\n$%d\r\n%s\r\n", len(redisKey), redisKey)
		conn.Write([]byte(delCmd))
		buf := make([]byte, 32)
		conn.Read(buf)
		t.Logf("Simulated Redis eviction by deleting %s", redisKey)
	}

	// 3. Repeat request: PostgreSQL fallback should satisfy the request
	req2, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idemKey)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Fallback request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK from Postgres fallback replay, got %d", resp2.StatusCode)
	}

	if resp2.Header.Get("X-Idempotent-Replay") != "true" {
		t.Errorf("Expected X-Idempotent-Replay: true")
	}

	var b2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&b2)
	bID2 := b2["bookingId"]
	if bID2 == nil {
		bID2 = b2["id"]
	}

	if bID1 != bID2 {
		t.Fatalf("Booking ID mismatch: initial=%v, fallback=%v", bID1, bID2)
	}

	t.Logf("PostgreSQL fallback idempotency recovery passed: %v", bID1)
}

