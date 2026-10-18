package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleVerifyTicketValidation(t *testing.T) {
	s := &Server{db: nil}

	// 1. Method Not Allowed
	req := httptest.NewRequest(http.MethodGet, "/api/bookings/verify-ticket", nil)
	rr := httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// 2. Empty Body
	req = httptest.NewRequest(http.MethodPost, "/api/bookings/verify-ticket", bytes.NewBuffer([]byte("")))
	rr = httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty payload, got %d", rr.Code)
	}

	// 3. Missing ticketCode
	badPayload, _ := json.Marshal(map[string]string{"eventId": "evt-123"})
	req = httptest.NewRequest(http.MethodPost, "/api/bookings/verify-ticket", bytes.NewBuffer(badPayload))
	rr = httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing ticketCode, got %d", rr.Code)
	}
}

func TestHandleVerifyTicketWithDB(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		t.Skip("Postgres not available, skipping db integration verification")
		return
	}
	defer db.Close()

	s := &Server{db: db}

	// Insert test booking and ticket
	testBookingID := "bk-verify-" + time.Now().Format("20060102150405.000")
	testTicketCode := "TKT-VERIFY-" + time.Now().Format("150405000")

	var testEventID string
	if err := db.QueryRow("SELECT id FROM events LIMIT 1").Scan(&testEventID); err != nil || testEventID == "" {
		testEventID = "evt-101"
	}

	_, err := db.Exec(`
		INSERT INTO bookings (id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, payment_status, created_at, updated_at)
		VALUES ($1, $2, 'usr-test', 'Alice Verifier', 'alice@example.com', $3, 'Cinema Premiere', 'tier-vip', 1, '["A-1"]', 50.0, 'CONFIRMED', 'PAID', NOW(), NOW())
	`, testBookingID, "REF-"+testBookingID, testEventID)
	if err != nil {
		t.Fatalf("failed to insert test booking: %v", err)
	}
	defer db.Exec(`DELETE FROM tickets WHERE booking_id = $1`, testBookingID)
	defer db.Exec(`DELETE FROM bookings WHERE id = $1`, testBookingID)

	_, err = db.Exec(`
		INSERT INTO tickets (id, booking_id, seat_id, seat_label, ticket_code, status, created_at)
		VALUES ($1, $2, 'seat-a1', 'A-1', $3, 'VALID', NOW())
	`, "tkt-"+testBookingID, testBookingID, testTicketCode)
	if err != nil {
		t.Fatalf("failed to insert test ticket: %v", err)
	}

	// 1. Verify valid ticket
	verifyReq := VerifyTicketRequest{
		TicketCode: testTicketCode,
		Action:     "VERIFY",
	}
	reqBody, _ := json.Marshal(verifyReq)
	req := httptest.NewRequest(http.MethodPost, "/api/bookings/verify-ticket", bytes.NewBuffer(reqBody))
	rr := httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid ticket, got %d: %s", rr.Code, rr.Body.String())
	}

	var res VerifyTicketResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !res.Valid || res.Status != "VALID" {
		t.Errorf("expected valid ticket with status VALID, got %+v", res)
	}

	// 2. Perform CHECK_IN
	checkInReq := VerifyTicketRequest{
		TicketCode: testTicketCode,
		Action:     "CHECK_IN",
	}
	reqBody, _ = json.Marshal(checkInReq)
	req = httptest.NewRequest(http.MethodPost, "/api/bookings/verify-ticket", bytes.NewBuffer(reqBody))
	rr = httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for check-in, got %d: %s", rr.Code, rr.Body.String())
	}
	var checkInRes VerifyTicketResponse
	_ = json.NewDecoder(rr.Body).Decode(&checkInRes)
	if checkInRes.Status != "USED" {
		t.Errorf("expected ticket status to transition to USED, got %s", checkInRes.Status)
	}

	// 3. Second CHECK_IN must fail with 409 Conflict
	req = httptest.NewRequest(http.MethodPost, "/api/bookings/verify-ticket", bytes.NewBuffer(reqBody))
	rr = httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for double check-in, got %d", rr.Code)
	}
}
