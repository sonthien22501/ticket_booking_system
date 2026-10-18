package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandleTransferTicketValidation(t *testing.T) {
	s := &Server{db: nil}

	// 1. Method Not Allowed
	req := httptest.NewRequest(http.MethodGet, "/api/tickets/transfer", nil)
	rr := httptest.NewRecorder()
	s.handleTransferTicket(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// 2. Empty Body
	req = httptest.NewRequest(http.MethodPost, "/api/tickets/transfer", bytes.NewBuffer([]byte("")))
	rr = httptest.NewRecorder()
	s.handleTransferTicket(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on empty body, got %d", rr.Code)
	}

	// 3. Missing ticketIdentifier
	badPayload, _ := json.Marshal(map[string]string{
		"recipientEmail": "friend@example.com",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/tickets/transfer", bytes.NewBuffer(badPayload))
	rr = httptest.NewRecorder()
	s.handleTransferTicket(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing ticket identifier, got %d", rr.Code)
	}

	// 4. Missing recipientEmail
	badPayload2, _ := json.Marshal(map[string]string{
		"ticketCode": "TKT-12345",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/tickets/transfer", bytes.NewBuffer(badPayload2))
	rr = httptest.NewRecorder()
	s.handleTransferTicket(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on missing recipientEmail, got %d", rr.Code)
	}
}

func TestHandleTransferTicketWithDB(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		t.Skip("Postgres not available, skipping db integration test")
		return
	}
	defer db.Close()

	s := &Server{db: db}
	if err := s.initTicketTransferSchema(context.Background()); err != nil {
		t.Fatalf("failed to init ticket transfer schema: %v", err)
	}

	testBookingID := "bk-xfer-" + time.Now().Format("20060102150405.000")
	testTicketCode := "TKT-XFER-SRC-" + time.Now().Format("150405000")
	testTicketID := "tkt-" + testBookingID

	var testEventID string
	if err := db.QueryRow("SELECT id FROM events LIMIT 1").Scan(&testEventID); err != nil || testEventID == "" {
		testEventID = "evt-101"
	}

	_, err := db.Exec(`
		INSERT INTO bookings (id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, payment_status, created_at, updated_at)
		VALUES ($1, $2, 'usr-sender', 'Sender Customer', 'sender@example.com', $3, 'Neon Symphony', 'tier-vip', 1, '["A-1"]', 150.0, 'CONFIRMED', 'PAID', NOW(), NOW())
	`, testBookingID, "REF-"+testBookingID, testEventID)
	if err != nil {
		t.Fatalf("failed to insert test booking: %v", err)
	}
	defer db.Exec(`DELETE FROM ticket_transfers WHERE booking_id = $1`, testBookingID)
	defer db.Exec(`DELETE FROM tickets WHERE booking_id = $1`, testBookingID)
	defer db.Exec(`DELETE FROM bookings WHERE id = $1`, testBookingID)

	_, err = db.Exec(`
		INSERT INTO tickets (id, booking_id, seat_id, seat_label, ticket_code, status, created_at)
		VALUES ($1, $2, 'seat-a1', 'Row A, Seat 1', $3, 'VALID', NOW())
	`, testTicketID, testBookingID, testTicketCode)
	if err != nil {
		t.Fatalf("failed to insert test ticket: %v", err)
	}

	// 1. Successful ticket transfer
	transferReq := TicketTransferRequest{
		TicketCode:     testTicketCode,
		RecipientEmail: "recipient@example.com",
		RecipientName:  "Lucky Friend",
		FromUserID:     "usr-sender",
		Note:           "Have fun tonight!",
	}
	reqBody, _ := json.Marshal(transferReq)
	req := httptest.NewRequest(http.MethodPost, "/api/tickets/transfer", bytes.NewBuffer(reqBody))
	rr := httptest.NewRecorder()
	s.handleTransferTicket(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ticket transfer, got %d: %s", rr.Code, rr.Body.String())
	}

	var res TicketTransferResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Status != "COMPLETED" || res.NewTicketCode == "" {
		t.Errorf("expected COMPLETED transfer with newTicketCode, got %+v", res)
	}

	// Clean up transferred booking and ticket created for recipient
	if res.NewBookingID != "" {
		defer db.Exec(`DELETE FROM tickets WHERE booking_id = $1`, res.NewBookingID)
		defer db.Exec(`DELETE FROM bookings WHERE id = $1`, res.NewBookingID)
	}

	// 2. Old ticket must be marked TRANSFERRED in DB
	var oldStatus string
	err = db.QueryRow(`SELECT status FROM tickets WHERE ticket_code = $1`, testTicketCode).Scan(&oldStatus)
	if err != nil {
		t.Fatalf("failed to query old ticket status: %v", err)
	}
	if oldStatus != "TRANSFERRED" {
		t.Errorf("expected old ticket status to be TRANSFERRED, got %s", oldStatus)
	}

	// 3. Gate verification on old ticket must fail
	gateReqOld := VerifyTicketRequest{
		TicketCode: testTicketCode,
		Action:     "VERIFY",
	}
	gateBodyOld, _ := json.Marshal(gateReqOld)
	req = httptest.NewRequest(http.MethodPost, "/api/bookings/verify-ticket", bytes.NewBuffer(gateBodyOld))
	rr = httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)

	var gateResOld VerifyTicketResponse
	_ = json.NewDecoder(rr.Body).Decode(&gateResOld)
	if gateResOld.Valid || gateResOld.Status != "TRANSFERRED" {
		t.Errorf("expected gate verification to reject transferred ticket, got %+v", gateResOld)
	}

	// 4. Gate verification on NEW ticket must succeed
	gateReqNew := VerifyTicketRequest{
		TicketCode: res.NewTicketCode,
		Action:     "VERIFY",
	}
	gateBodyNew, _ := json.Marshal(gateReqNew)
	req = httptest.NewRequest(http.MethodPost, "/api/bookings/verify-ticket", bytes.NewBuffer(gateBodyNew))
	rr = httptest.NewRecorder()
	s.handleVerifyTicket(rr, req)

	var gateResNew VerifyTicketResponse
	_ = json.NewDecoder(rr.Body).Decode(&gateResNew)
	if !gateResNew.Valid || gateResNew.Status != "VALID" {
		t.Errorf("expected gate verification to approve new recipient ticket, got %+v", gateResNew)
	}

	// 5. Attempting to re-transfer the old ticket must fail with 409 Conflict
	req = httptest.NewRequest(http.MethodPost, "/api/tickets/transfer", bytes.NewBuffer(reqBody))
	rr = httptest.NewRecorder()
	s.handleTransferTicket(rr, req)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict when transferring already transferred ticket, got %d", rr.Code)
	}
}
