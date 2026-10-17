package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

type ReservationDetailsResponse struct {
	ReservationID    string   `json:"reservationId"`
	EventID          string   `json:"eventId"`
	TierID           string   `json:"tierId"`
	SeatIDs          []string `json:"seatIds"`
	Quantity         int      `json:"quantity"`
	Status           string   `json:"status"`
	ExpiresAt        string   `json:"expiresAt"`
	CreatedAt        string   `json:"createdAt"`
	RemainingSeconds int      `json:"remainingSeconds"`
	IsExpired        bool     `json:"isExpired"`
	Error            string   `json:"error,omitempty"`
}

type JanitorSweepResponse struct {
	Status         string `json:"status"`
	ReclaimedCount int    `json:"reclaimedCount"`
	Message        string `json:"message"`
}

func createShortHoldReservation(t *testing.T, seatID string, holdSeconds int) map[string]interface{} {
	eventID := getBaseEvent(t)
	payload := map[string]interface{}{
		"eventId":     eventID,
		"tierId":      "tier-vip",
		"quantity":    1,
		"seats":       []string{seatID},
		"holdSeconds": holdSeconds,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(gatewayURL+"/inventory/reserve", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Reserve request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected reserve status 200 OK, got %d: %s", resp.StatusCode, string(respBody))
	}

	var res map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&res)
	return res
}

func TestReservationTimeoutAutomaticRelease(t *testing.T) {
	eventID := getBaseEvent(t)
	seat := getFreshAvailableSeat(t)

	// 1. Check initial available count
	initialSeats := getAvailableSeats(t, eventID)
	initialCount := len(initialSeats)

	// 2. Reserve seat with a 1-second timeout lease
	res := createShortHoldReservation(t, seat, 1)
	resID := res["reservationId"].(string)
	t.Logf("Created short-lived reservation %s holding seat %s for 1 second", resID, seat)

	// 3. Immediately verify seat is in HOLD status
	resp, err := http.Get(fmt.Sprintf("%s/inventory/reservations/%s", gatewayURL, resID))
	if err != nil {
		t.Fatalf("Failed to fetch reservation: %v", err)
	}
	var details ReservationDetailsResponse
	json.NewDecoder(resp.Body).Decode(&details)
	resp.Body.Close()

	if details.Status != "HOLD" && details.Status != "EXPIRED" {
		t.Fatalf("Expected reservation status HOLD, got %s", details.Status)
	}

	// 4. Wait for the lease window to expire and the background janitor to sweep (runs every 3s)
	t.Logf("Waiting up to 5s for lease expiration and automatic background janitor sweep...")
	var detailsAfter ReservationDetailsResponse
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		respAfter, err := http.Get(fmt.Sprintf("%s/inventory/reservations/%s", gatewayURL, resID))
		if err == nil {
			json.NewDecoder(respAfter.Body).Decode(&detailsAfter)
			respAfter.Body.Close()
			if detailsAfter.Status == "EXPIRED" {
				break
			}
		}
	}

	if detailsAfter.Status != "EXPIRED" {
		t.Fatalf("Expected reservation status to be EXPIRED, got %s", detailsAfter.Status)
	}
	t.Logf("Reservation %s successfully transitioned to EXPIRED", resID)

	// 6. Verify seat is restored to AVAILABLE in event seat map
	seatsAfter := getAvailableSeats(t, eventID)
	seatFound := false
	for _, s := range seatsAfter {
		if s == seat {
			seatFound = true
			break
		}
	}
	if !seatFound {
		t.Fatalf("Expected seat %s to be released back to AVAILABLE, but was not in available seats", seat)
	}

	if len(seatsAfter) != initialCount {
		t.Fatalf("Expected available seats count to be restored to %d, got %d", initialCount, len(seatsAfter))
	}
	t.Logf("Seat %s confirmed AVAILABLE and inventory count restored to %d", seat, len(seatsAfter))
}

func TestExpiredSeatCanBeReservedByAnotherUser(t *testing.T) {
	seat := getFreshAvailableSeat(t)

	// 1. User 1 reserves seat with 1-second timeout
	res1 := createShortHoldReservation(t, seat, 1)
	resID1 := res1["reservationId"].(string)

	// 2. Wait for expiration and sweep
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		rCheck, err := http.Get(fmt.Sprintf("%s/inventory/reservations/%s", gatewayURL, resID1))
		if err == nil {
			var dt ReservationDetailsResponse
			json.NewDecoder(rCheck.Body).Decode(&dt)
			rCheck.Body.Close()
			if dt.Status == "EXPIRED" {
				break
			}
		}
	}

	// 3. User 2 immediately reserves the expired seat
	res2 := createShortHoldReservation(t, seat, 600)
	resID2 := res2["reservationId"].(string)

	if resID2 == resID1 {
		t.Fatalf("Expected distinct reservation IDs, got duplicate %s", resID2)
	}
	if res2["status"] != "HOLD" {
		t.Fatalf("Expected User 2 reservation to be HOLD, got %v", res2["status"])
	}
	t.Logf("User 2 successfully claimed expired seat %s under reservation %s", seat, resID2)

	// Cleanup User 2 reservation
	cancelPayload := map[string]interface{}{"reservationId": resID2}
	cBody, _ := json.Marshal(cancelPayload)
	cResp, _ := http.Post(gatewayURL+"/inventory/cancel", "application/json", bytes.NewReader(cBody))
	if cResp != nil {
		cResp.Body.Close()
	}
}

func TestExpiredReservationCannotBeCommitted(t *testing.T) {
	seat := getFreshAvailableSeat(t)

	// 1. Reserve seat with 1-second timeout
	res := createShortHoldReservation(t, seat, 1)
	resID := res["reservationId"].(string)

	// 2. Wait for lease expiration
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		rCheck, err := http.Get(fmt.Sprintf("%s/inventory/reservations/%s", gatewayURL, resID))
		if err == nil {
			var dt ReservationDetailsResponse
			json.NewDecoder(rCheck.Body).Decode(&dt)
			rCheck.Body.Close()
			if dt.Status == "EXPIRED" {
				break
			}
		}
	}

	// 3. Attempt to commit the expired reservation
	commitPayload := map[string]string{
		"reservationId": resID,
	}
	body, _ := json.Marshal(commitPayload)
	resp, err := http.Post(gatewayURL+"/inventory/commit", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Commit request failed: %v", err)
	}
	defer resp.Body.Close()

	// 4. Must reject with 409 Conflict
	if resp.StatusCode != http.StatusConflict {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected HTTP 409 Conflict on expired reservation commit, got %d: %s", resp.StatusCode, string(respBody))
	}

	var errRes map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errRes)
	if errRes["error"] != "RESERVATION_EXPIRED" && errRes["error"] != "RESERVATION_INVALID_STATE" {
		t.Fatalf("Expected RESERVATION_EXPIRED or RESERVATION_INVALID_STATE, got %v", errRes["error"])
	}
	t.Logf("Commit on expired reservation was correctly blocked with HTTP 409: %v", errRes["error"])
}

func TestReservationCountdownStatus(t *testing.T) {
	seat := getFreshAvailableSeat(t)

	// Reserve seat for 300 seconds (5 minutes)
	res := createShortHoldReservation(t, seat, 300)
	resID := res["reservationId"].(string)

	// Query reservation status
	resp, err := http.Get(fmt.Sprintf("%s/inventory/reservations/%s", gatewayURL, resID))
	if err != nil {
		t.Fatalf("Get reservation failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var details ReservationDetailsResponse
	json.NewDecoder(resp.Body).Decode(&details)

	if details.ReservationID != resID {
		t.Fatalf("Expected reservation ID %s, got %s", resID, details.ReservationID)
	}
	if details.Status != "HOLD" {
		t.Fatalf("Expected status HOLD, got %s", details.Status)
	}
	if details.RemainingSeconds < 250 || details.RemainingSeconds > 300 {
		t.Fatalf("Expected remaining seconds ~300, got %d", details.RemainingSeconds)
	}
	if details.IsExpired {
		t.Fatalf("Expected isExpired to be false for active hold")
	}
	t.Logf("Verified reservation countdown timer: %d remaining seconds", details.RemainingSeconds)

	// Cleanup
	cPayload := map[string]interface{}{"reservationId": resID}
	cBody, _ := json.Marshal(cPayload)
	cResp, _ := http.Post(gatewayURL+"/inventory/cancel", "application/json", bytes.NewReader(cBody))
	if cResp != nil {
		cResp.Body.Close()
	}
}

func TestManualJanitorSweepEndpoint(t *testing.T) {
	resp, err := http.Post(gatewayURL+"/inventory/janitor/sweep", "application/json", nil)
	if err != nil {
		t.Fatalf("Sweep request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK from manual sweep, got %d", resp.StatusCode)
	}

	var sweepRes JanitorSweepResponse
	json.NewDecoder(resp.Body).Decode(&sweepRes)

	if sweepRes.Status != "SUCCESS" {
		t.Fatalf("Expected status SUCCESS, got %s", sweepRes.Status)
	}
	t.Logf("Manual janitor sweep verified successfully: %s", sweepRes.Message)
}
