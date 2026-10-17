package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type WaitlistEntryResponse struct {
	ID            string  `json:"id"`
	EventID       string  `json:"eventId"`
	TierID        string  `json:"tierId,omitempty"`
	SeatID        string  `json:"seatId,omitempty"`
	UserID        string  `json:"userId"`
	CustomerName  string  `json:"customerName"`
	CustomerEmail string  `json:"customerEmail"`
	Quantity      int     `json:"quantity"`
	Status        string  `json:"status"`
	ReservationID string  `json:"reservationId,omitempty"`
	PromotedAt    *string `json:"promotedAt,omitempty"`
	ExpiresAt     *string `json:"expiresAt,omitempty"`
	Position      int     `json:"position,omitempty"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
	Error         string  `json:"error,omitempty"`
	Message       string  `json:"message,omitempty"`
}

func joinWaitlist(t *testing.T, payload map[string]interface{}) WaitlistEntryResponse {
	body, _ := json.Marshal(payload)
	resp, err := http.Post(gatewayURL+"/waitlist", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Join waitlist request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected waitlist join status 201 Created, got %d: %s", resp.StatusCode, string(respBytes))
	}

	var entry WaitlistEntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatalf("Failed to decode waitlist response: %v", err)
	}
	return entry
}

func getWaitlistEntryE2E(t *testing.T, id string) WaitlistEntryResponse {
	resp, err := http.Get(fmt.Sprintf("%s/waitlist/%s", gatewayURL, id))
	if err != nil {
		t.Fatalf("Get waitlist entry request failed: %v", err)
	}
	defer resp.Body.Close()

	var entry WaitlistEntryResponse
	_ = json.NewDecoder(resp.Body).Decode(&entry)
	return entry
}

func cancelWaitlistEntryE2E(t *testing.T, id string) {
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/waitlist/%s", gatewayURL, id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

func seatToTier(seat string) string {
	if strings.HasPrefix(seat, "B") {
		return "tier-gen"
	}
	return "tier-vip"
}

func createWaitlistTestBooking(t *testing.T, seatID string) map[string]interface{} {
	eventID := getBaseEvent(t)
	tierID := seatToTier(seatID)
	payload := map[string]interface{}{
		"eventId":       eventID,
		"tierId":        tierID,
		"quantity":      1,
		"seats":         []string{seatID},
		"customerName":  "Waitlist Cancellation Tester",
		"customerEmail": "waitlist-cancel@example.com",
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create booking request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Booking request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected booking status 201 Created, got %d: %s", resp.StatusCode, string(respBody))
	}

	var res map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&res)
	return res
}

func createWaitlistHold(t *testing.T, seatID string, holdSeconds int) map[string]interface{} {
	eventID := getBaseEvent(t)
	tierID := seatToTier(seatID)
	payload := map[string]interface{}{
		"eventId":     eventID,
		"tierId":      tierID,
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

func claimWaitlistEntryE2E(t *testing.T, id string) (int, map[string]interface{}) {
	resp, err := http.Post(fmt.Sprintf("%s/waitlist/%s/claim", gatewayURL, id), "application/json", nil)
	if err != nil {
		t.Fatalf("Claim waitlist request failed: %v", err)
	}
	defer resp.Body.Close()

	var res map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	return resp.StatusCode, res
}

func TestWaitlistJoinAndQueuePosition(t *testing.T) {
	eventID := getBaseEvent(t)
	prefix := fmt.Sprintf("wl-user-%d", time.Now().UnixNano())

	// Join 3 users in order
	u1 := joinWaitlist(t, map[string]interface{}{
		"eventId":       eventID,
		"tierId":        "tier-vip",
		"userId":        prefix + "-1",
		"customerName":  "FIFO User 1",
		"customerEmail": prefix + "-1@test.com",
	})
	defer cancelWaitlistEntryE2E(t, u1.ID)

	time.Sleep(20 * time.Millisecond)
	u2 := joinWaitlist(t, map[string]interface{}{
		"eventId":       eventID,
		"tierId":        "tier-vip",
		"userId":        prefix + "-2",
		"customerName":  "FIFO User 2",
		"customerEmail": prefix + "-2@test.com",
	})
	defer cancelWaitlistEntryE2E(t, u2.ID)

	time.Sleep(20 * time.Millisecond)
	u3 := joinWaitlist(t, map[string]interface{}{
		"eventId":       eventID,
		"tierId":        "tier-vip",
		"userId":        prefix + "-3",
		"customerName":  "FIFO User 3",
		"customerEmail": prefix + "-3@test.com",
	})
	defer cancelWaitlistEntryE2E(t, u3.ID)

	// Verify relative FIFO queue positions
	f1 := getWaitlistEntryE2E(t, u1.ID)
	f2 := getWaitlistEntryE2E(t, u2.ID)
	f3 := getWaitlistEntryE2E(t, u3.ID)

	if !(f1.Position < f2.Position && f2.Position < f3.Position) {
		t.Fatalf("Expected strictly increasing positions (f1 < f2 < f3), got: f1=%d, f2=%d, f3=%d", f1.Position, f2.Position, f3.Position)
	}

	// Verify list endpoint
	resp, err := http.Get(fmt.Sprintf("%s/waitlist?eventId=%s&userId=%s", gatewayURL, eventID, prefix+"-1"))
	if err != nil {
		t.Fatalf("List waitlist failed: %v", err)
	}
	defer resp.Body.Close()

	var list []WaitlistEntryResponse
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) == 0 || list[0].ID != u1.ID {
		t.Fatalf("Expected list to contain user 1's entry")
	}
}

func TestWaitlistAutoPromotionOnCancellation(t *testing.T) {
	eventID := getBaseEvent(t)
	seat := getFreshAvailableSeat(t)
	tierID := seatToTier(seat)

	// 1. Create a confirmed booking for the seat
	booking := createWaitlistTestBooking(t, seat)
	bookingID := booking["id"].(string)
	t.Logf("Created booking %s holding seat %s in tier %s", bookingID, seat, tierID)

	// 2. User joins waitlist for that event and tier
	prefix := fmt.Sprintf("wl-cancel-%d", time.Now().UnixNano())
	wlEntry := joinWaitlist(t, map[string]interface{}{
		"eventId":       eventID,
		"tierId":        tierID,
		"userId":        prefix,
		"customerName":  "Cancel Waitlist User",
		"customerEmail": prefix + "@example.com",
	})
	defer cancelWaitlistEntryE2E(t, wlEntry.ID)

	if wlEntry.Status != "PENDING" {
		t.Fatalf("Expected entry to start in PENDING, got %s", wlEntry.Status)
	}

	// 3. Cancel the confirmed booking
	cancelURL := fmt.Sprintf("%s/bookings/%s/cancel", gatewayURL, bookingID)
	cReq, _ := http.NewRequest(http.MethodPost, cancelURL, bytes.NewReader([]byte(`{"reason":"CUSTOMER_REQUEST"}`)))
	cReq.Header.Set("Content-Type", "application/json")
	cResp, err := http.DefaultClient.Do(cReq)
	if err != nil {
		t.Fatalf("Cancel booking request failed: %v", err)
	}
	defer cResp.Body.Close()

	if cResp.StatusCode != http.StatusOK {
		cBody, _ := io.ReadAll(cResp.Body)
		t.Fatalf("Expected cancel status 200 OK, got %d: %s", cResp.StatusCode, string(cBody))
	}
	t.Logf("Booking %s cancelled successfully", bookingID)

	// 4. Poll waitlist entry for up to 8s for automatic promotion
	var promoted WaitlistEntryResponse
	for i := 0; i < 32; i++ {
		time.Sleep(250 * time.Millisecond)
		promoted = getWaitlistEntryE2E(t, wlEntry.ID)
		if promoted.Status == "PROMOTED" && promoted.ReservationID != "" {
			break
		}
	}

	if promoted.Status != "PROMOTED" {
		t.Fatalf("Expected waitlist entry %s to be PROMOTED, got status: %s", wlEntry.ID, promoted.Status)
	}
	if promoted.ReservationID == "" {
		t.Fatalf("Expected non-empty ReservationID on promoted waitlist entry")
	}
	t.Logf("Waitlist entry %s successfully promoted with reservation %s", wlEntry.ID, promoted.ReservationID)

	// 5. Claim the promoted seat
	statusCode, claimResp := claimWaitlistEntryE2E(t, wlEntry.ID)
	if statusCode != http.StatusOK {
		t.Fatalf("Expected claim status 200 OK, got %d: %+v", statusCode, claimResp)
	}
	if claimResp["status"] != "CONFIRMED" {
		t.Fatalf("Expected claimed booking status CONFIRMED, got %v", claimResp["status"])
	}

	// 6. Test Idempotency: Claiming again should succeed and return the same booking
	statusCode2, claimResp2 := claimWaitlistEntryE2E(t, wlEntry.ID)
	if statusCode2 != http.StatusOK {
		t.Fatalf("Expected idempotent claim status 200 OK, got %d", statusCode2)
	}
	if claimResp2["id"] != claimResp["id"] {
		t.Fatalf("Expected idempotent replay to return same booking ID %v, got %v", claimResp["id"], claimResp2["id"])
	}

	// 7. Check waitlist entry is now CLAIMED
	claimedEntry := getWaitlistEntryE2E(t, wlEntry.ID)
	if claimedEntry.Status != "CLAIMED" {
		t.Fatalf("Expected waitlist entry status CLAIMED, got %s", claimedEntry.Status)
	}
}

func TestWaitlistAutoPromotionOnTimeout(t *testing.T) {
	eventID := getBaseEvent(t)
	seat := getFreshAvailableSeat(t)
	tierID := seatToTier(seat)

	// 1. Reserve seat with a 1-second timeout
	res := createWaitlistHold(t, seat, 1)
	resID := res["reservationId"].(string)
	t.Logf("Created short hold %s for seat %s in tier %s", resID, seat, tierID)

	// 2. User joins waitlist for that event and tier
	prefix := fmt.Sprintf("wl-timeout-%d", time.Now().UnixNano())
	wlEntry := joinWaitlist(t, map[string]interface{}{
		"eventId":       eventID,
		"tierId":        tierID,
		"userId":        prefix,
		"customerName":  "Timeout Waitlist User",
		"customerEmail": prefix + "@example.com",
	})
	defer cancelWaitlistEntryE2E(t, wlEntry.ID)

	// 3. Wait for the 1s lease to expire and background janitor sweep to trigger promotion
	var promoted WaitlistEntryResponse
	for i := 0; i < 32; i++ {
		time.Sleep(250 * time.Millisecond)
		promoted = getWaitlistEntryE2E(t, wlEntry.ID)
		if promoted.Status == "PROMOTED" && promoted.ReservationID != "" {
			break
		}
	}

	if promoted.Status != "PROMOTED" {
		t.Fatalf("Expected waitlist entry to be promoted on timeout sweep, got status %s", promoted.Status)
	}
	t.Logf("Waitlist entry %s promoted on hold expiration with reservation %s", wlEntry.ID, promoted.ReservationID)
}

func TestWaitlistConcurrentFIFOOrdering(t *testing.T) {
	eventID := getBaseEvent(t)
	prefix := fmt.Sprintf("wl-conc-%d", time.Now().UnixNano())
	numUsers := 5

	var entries []WaitlistEntryResponse
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < numUsers; i++ {
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := joinWaitlist(t, map[string]interface{}{
				"eventId":       eventID,
				"tierId":        "tier-vip",
				"userId":        fmt.Sprintf("%s-%d", prefix, idx),
				"customerName":  fmt.Sprintf("Concurrent User %d", idx),
				"customerEmail": fmt.Sprintf("%s-%d@test.com", prefix, idx),
			})
			mu.Lock()
			entries = append(entries, e)
			mu.Unlock()
		}()
	}
	wg.Wait()

	for _, e := range entries {
		defer cancelWaitlistEntryE2E(t, e.ID)
	}

	if len(entries) != numUsers {
		t.Fatalf("Expected %d created waitlist entries, got %d", numUsers, len(entries))
	}

	// Verify all entries have valid positions >= 1
	for _, e := range entries {
		fetched := getWaitlistEntryE2E(t, e.ID)
		if fetched.Position < 1 {
			t.Errorf("Expected position >= 1 for entry %s, got %d", e.ID, fetched.Position)
		}
		if fetched.Status != "PENDING" {
			t.Errorf("Expected status PENDING for entry %s, got %s", e.ID, fetched.Status)
		}
	}
}

func TestWaitlistReviewFocusEdgeCases(t *testing.T) {
	eventID := getBaseEvent(t)
	prefix := fmt.Sprintf("wl-edge-%d", time.Now().UnixNano())

	// Edge Case 1: Unpromoted user attempting to claim should fail with 400 Bad Request
	e1 := joinWaitlist(t, map[string]interface{}{
		"eventId":       eventID,
		"tierId":        "tier-vip",
		"userId":        prefix + "-unpromoted",
		"customerName":  "Edge Unpromoted",
		"customerEmail": prefix + "-unprom@test.com",
	})
	defer cancelWaitlistEntryE2E(t, e1.ID)

	code, claimErr := claimWaitlistEntryE2E(t, e1.ID)
	if code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request when claiming unpromoted entry, got %d: %+v", code, claimErr)
	}

	// Edge Case 2: Cancellation of waitlist entry
	cancelWaitlistEntryE2E(t, e1.ID)
	afterCancel := getWaitlistEntryE2E(t, e1.ID)
	if afterCancel.Status != "CANCELLED" {
		t.Errorf("Expected cancelled entry status CANCELLED, got %s", afterCancel.Status)
	}

	// Edge Case 3: Cancelled entry cannot be claimed
	codeCancelled, _ := claimWaitlistEntryE2E(t, e1.ID)
	if codeCancelled != http.StatusBadRequest {
		t.Errorf("Expected status 400 when claiming cancelled entry, got %d", codeCancelled)
	}
}
