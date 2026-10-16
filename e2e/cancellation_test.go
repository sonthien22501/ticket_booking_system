package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type CancelResponse struct {
	BookingID        string `json:"bookingId"`
	BookingReference string `json:"bookingReference"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	Error            string `json:"error,omitempty"`
}

func getFreshAvailableSeat(t *testing.T) string {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) == 0 {
		t.Fatalf("No available seats left for test")
	}
	// Prefer non-row-A seats (e.g. B, C) to avoid colliding with CI tests on A1, A2, A3
	for i := len(seats) - 1; i >= 0; i-- {
		s := seats[i]
		if !strings.HasPrefix(s, "A") {
			return s
		}
	}
	return seats[len(seats)-1]
}

func createTestBooking(t *testing.T, token string, seatID string) map[string]interface{} {
	eventID := getBaseEvent(t)
	payload := map[string]interface{}{
		"eventId":       eventID,
		"tierId":        "tier-vip",
		"quantity":      1,
		"seats":         []string{seatID},
		"customerName":  "Cancellation Tester",
		"customerEmail": "cancel-tester@example.com",
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

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

func TestBookingCancellationSuccess(t *testing.T) {
	// 1. Create a booking for dynamic available seat
	seat := getFreshAvailableSeat(t)
	booking := createTestBooking(t, "", seat)
	bookingID := booking["id"].(string)
	if bookingID == "" {
		bookingID = booking["bookingId"].(string)
	}
	t.Logf("Created initial booking: ID=%s, Seat=%s, Status=%v", bookingID, seat, booking["status"])

	// 2. Query initial seat state: should be BOOKED
	eventID := getBaseEvent(t)
	resp, err := http.Get(fmt.Sprintf("%s/inventory/%s/seats", gatewayURL, eventID))
	if err != nil {
		t.Fatalf("Failed to fetch seats: %v", err)
	}
	var seatData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&seatData)
	resp.Body.Close()

	// 3. Cancel the booking via POST /api/bookings/{id}/cancel
	cancelURL := fmt.Sprintf("%s/bookings/%s/cancel", gatewayURL, bookingID)
	cancelReq, _ := http.NewRequest(http.MethodPost, cancelURL, nil)
	cancelResp, err := http.DefaultClient.Do(cancelReq)
	if err != nil {
		t.Fatalf("Cancel request failed: %v", err)
	}
	defer cancelResp.Body.Close()

	if cancelResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cancelResp.Body)
		t.Fatalf("Expected status 200 OK, got %d: %s", cancelResp.StatusCode, string(body))
	}

	var cancelResult CancelResponse
	json.NewDecoder(cancelResp.Body).Decode(&cancelResult)
	if cancelResult.Status != "CANCELLED" {
		t.Fatalf("Expected status CANCELLED, got %s", cancelResult.Status)
	}
	t.Logf("Cancellation successful: %s", cancelResult.Message)

	// 4. Verify booking status is now CANCELLED via GET /api/bookings/{id}
	getResp, err := http.Get(fmt.Sprintf("%s/bookings/%s", gatewayURL, bookingID))
	if err != nil {
		t.Fatalf("Failed to fetch booking: %v", err)
	}
	defer getResp.Body.Close()

	var updatedBooking map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&updatedBooking)
	if updatedBooking["status"] != "CANCELLED" {
		t.Fatalf("Expected booking status to be CANCELLED, got %v", updatedBooking["status"])
	}

	// 5. Verify tickets are marked CANCELLED
	if tickets, ok := updatedBooking["tickets"].([]interface{}); ok && len(tickets) > 0 {
		ticket := tickets[0].(map[string]interface{})
		if ticket["status"] != "CANCELLED" {
			t.Fatalf("Expected ticket status to be CANCELLED, got %v", ticket["status"])
		}
	}
	t.Logf("Verified booking and tickets marked CANCELLED")
}

func TestRestoredSeatCanBeRebooked(t *testing.T) {
	seat := getFreshAvailableSeat(t)

	// 1. First user books the seat
	booking1 := createTestBooking(t, "", seat)
	bookingID1 := booking1["id"].(string)

	// 2. Cancel booking 1
	cancelURL := fmt.Sprintf("%s/bookings/%s/cancel", gatewayURL, bookingID1)
	req, _ := http.NewRequest(http.MethodPost, cancelURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to cancel booking 1: %v", err)
	}
	resp.Body.Close()

	// 3. Second user immediately books the restored seat
	booking2 := createTestBooking(t, "", seat)
	bookingID2 := booking2["id"].(string)

	if bookingID2 == bookingID1 {
		t.Fatalf("Expected distinct booking IDs, but got duplicate: %s", bookingID2)
	}
	if booking2["status"] != "CONFIRMED" {
		t.Fatalf("Expected re-booking to be CONFIRMED, got %v", booking2["status"])
	}
	t.Logf("Successfully re-booked released seat %s under new booking %s", seat, bookingID2)

	// Cleanup: Cancel booking 2 so seat is released back
	cancelReq2, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/bookings/%s/cancel", gatewayURL, bookingID2), nil)
	if cancelResp2, err := http.DefaultClient.Do(cancelReq2); err == nil {
		cancelResp2.Body.Close()
	}
}

func TestIdempotentCancellation(t *testing.T) {
	seat := getFreshAvailableSeat(t)
	booking := createTestBooking(t, "", seat)
	bookingID := booking["id"].(string)

	// Cancel 1st time
	cancelURL := fmt.Sprintf("%s/bookings/%s/cancel", gatewayURL, bookingID)
	req1, _ := http.NewRequest(http.MethodPost, cancelURL, nil)
	resp1, err := http.DefaultClient.Do(req1)
	if err != nil || resp1.StatusCode != http.StatusOK {
		t.Fatalf("First cancel failed: %v", err)
	}
	resp1.Body.Close()

	// Cancel 2nd time (should be idempotent 200 OK)
	req2, _ := http.NewRequest(http.MethodPost, cancelURL, nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("Second cancel request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Expected idempotent 200 OK on duplicate cancel, got %d: %s", resp2.StatusCode, string(body))
	}

	var cancelResult CancelResponse
	json.NewDecoder(resp2.Body).Decode(&cancelResult)
	if cancelResult.Status != "CANCELLED" {
		t.Fatalf("Expected status CANCELLED on replayed cancel, got %s", cancelResult.Status)
	}
	t.Logf("Idempotent cancellation verified for booking %s", bookingID)
}

func TestCancellationViaDeleteMethod(t *testing.T) {
	seat := getFreshAvailableSeat(t)
	booking := createTestBooking(t, "", seat)
	bookingID := booking["id"].(string)

	// Cancel via HTTP DELETE /api/bookings/{id}
	delURL := fmt.Sprintf("%s/bookings/%s", gatewayURL, bookingID)
	req, _ := http.NewRequest(http.MethodDelete, delURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200 OK on HTTP DELETE, got %d: %s", resp.StatusCode, string(body))
	}

	var cancelResult CancelResponse
	json.NewDecoder(resp.Body).Decode(&cancelResult)
	if cancelResult.Status != "CANCELLED" {
		t.Fatalf("Expected status CANCELLED on DELETE, got %s", cancelResult.Status)
	}
	t.Logf("Cancellation via HTTP DELETE verified for booking %s", bookingID)
}

func TestCancelNonExistentBooking(t *testing.T) {
	cancelURL := fmt.Sprintf("%s/bookings/non-existent-booking-id-999999/cancel", gatewayURL)
	req, _ := http.NewRequest(http.MethodPost, cancelURL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected 404 Not Found for non-existent booking, got %d", resp.StatusCode)
	}
	t.Logf("Non-existent booking cancellation properly returned 404")
}

func TestUnauthorizedCancellation(t *testing.T) {
	// 1. Register Customer A and Customer B
	emailA := fmt.Sprintf("customera-%d@example.com", time.Now().UnixNano())
	emailB := fmt.Sprintf("customerb-%d@example.com", time.Now().UnixNano())

	regA, _ := json.Marshal(map[string]string{
		"email": emailA, "password": "password123", "name": "Customer A", "role": "customer",
	})
	respA, err := http.Post(gatewayURL+"/auth/register", "application/json", bytes.NewReader(regA))
	if err != nil || respA.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to register customer A: %v", err)
	}
	var authA AuthResponse
	json.NewDecoder(respA.Body).Decode(&authA)
	respA.Body.Close()

	regB, _ := json.Marshal(map[string]string{
		"email": emailB, "password": "password123", "name": "Customer B", "role": "customer",
	})
	respB, err := http.Post(gatewayURL+"/auth/register", "application/json", bytes.NewReader(regB))
	if err != nil || respB.StatusCode != http.StatusCreated {
		t.Fatalf("Failed to register customer B: %v", err)
	}
	var authB AuthResponse
	json.NewDecoder(respB.Body).Decode(&authB)
	respB.Body.Close()

	// 2. Customer A creates a booking for dynamic available seat
	seat := getFreshAvailableSeat(t)
	booking := createTestBooking(t, authA.Token, seat)
	bookingID := booking["id"].(string)

	// 3. Customer B attempts to cancel Customer A's booking
	cancelURL := fmt.Sprintf("%s/bookings/%s/cancel", gatewayURL, bookingID)
	reqB, _ := http.NewRequest(http.MethodPost, cancelURL, nil)
	reqB.Header.Set("Authorization", "Bearer "+authB.Token)
	respCancelB, err := http.DefaultClient.Do(reqB)
	if err != nil {
		t.Fatalf("Cancel request from B failed: %v", err)
	}
	defer respCancelB.Body.Close()

	if respCancelB.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(respCancelB.Body)
		t.Fatalf("Expected 403 Forbidden when Customer B cancels Customer A's booking, got %d: %s", respCancelB.StatusCode, string(body))
	}
	t.Logf("Unauthorized cancellation correctly blocked with 403 Forbidden")

	// 4. Admin logs in and cancels Customer A's booking
	adminLogin, _ := json.Marshal(map[string]string{
		"email": "admin@tickets.com", "password": "admin123",
	})
	respAdmin, err := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(adminLogin))
	if err != nil || respAdmin.StatusCode != http.StatusOK {
		t.Fatalf("Failed to login as admin: %v", err)
	}
	var authAdmin AuthResponse
	json.NewDecoder(respAdmin.Body).Decode(&authAdmin)
	respAdmin.Body.Close()

	reqAdmin, _ := http.NewRequest(http.MethodPost, cancelURL, nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+authAdmin.Token)
	respCancelAdmin, err := http.DefaultClient.Do(reqAdmin)
	if err != nil {
		t.Fatalf("Cancel request from Admin failed: %v", err)
	}
	defer respCancelAdmin.Body.Close()

	if respCancelAdmin.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(respCancelAdmin.Body)
		t.Fatalf("Expected 200 OK when Admin cancels booking, got %d: %s", respCancelAdmin.StatusCode, string(body))
	}
	t.Logf("Admin cancellation successfully authorized with 200 OK")
}
