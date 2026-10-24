package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatEmailMessage(t *testing.T) {
	evt := NotificationEvent{
		EventType:     "booking.confirmed",
		BookingRef:    "REF-NEON-2026-001",
		EventTitle:    "Neon Symphony 2026",
		Recipient:     "user@tickets.com",
		CustomerName:  "Alex Morgan",
		Seats:         []string{"A1", "A2"},
		TotalPrice:    245.0,
		Currency:      "USD",
		TicketPassURL: "https://tickets.example.com/passes/REF-NEON-2026-001",
	}

	subject, htmlBody := formatEmailMessage(evt)
	if !strings.Contains(subject, "Booking Confirmed") || !strings.Contains(subject, "Neon Symphony 2026") {
		t.Errorf("unexpected email subject: %s", subject)
	}
	if !strings.Contains(htmlBody, "Alex Morgan") {
		t.Errorf("email body missing customer name: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "REF-NEON-2026-001") {
		t.Errorf("email body missing booking reference: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "A1, A2") {
		t.Errorf("email body missing reserved seats: %s", htmlBody)
	}
}

func TestFormatSMSMessage(t *testing.T) {
	evt := NotificationEvent{
		EventType:    "waitlist.promoted",
		BookingRef:   "WL-101-PROMOTE",
		EventTitle:   "Cyberpunk Symphony",
		Recipient:    "+15551234567",
		CustomerName: "Elena",
		Seats:        []string{"B4"},
	}

	sms := formatSMSMessage(evt)
	if !strings.Contains(sms, "Waitlist hold available") {
		t.Errorf("expected waitlist notice in SMS, got: %s", sms)
	}
	if !strings.Contains(sms, "Cyberpunk Symphony") {
		t.Errorf("expected event title in SMS, got: %s", sms)
	}
	// Transactional SMS should be concise (< 160 characters ideally, definitely < 250)
	if len(sms) > 250 {
		t.Errorf("SMS message is too long: %d characters", len(sms))
	}
}

func TestWorkerDispatchLogic(t *testing.T) {
	worker := NewNotificationWorker(nil, nil)

	evt := NotificationEvent{
		EventType:    "ticket.transferred",
		BookingRef:   "TKT-XFER-999",
		EventTitle:   "Grand Arena Summit",
		Recipient:    "friend@example.com",
		CustomerName: "David",
		Seats:        []string{"VIP-1"},
	}

	// Dispatch email
	delivery, err := worker.processEvent(evt, ChannelEmail)
	if err != nil {
		t.Fatalf("unexpected error processing event: %v", err)
	}
	if delivery.Status != StatusDelivered {
		t.Errorf("expected status DELIVERED, got %s", delivery.Status)
	}
	if delivery.Channel != ChannelEmail {
		t.Errorf("expected channel EMAIL, got %s", delivery.Channel)
	}

	// Dispatch SMS
	smsEvt := evt
	smsEvt.Recipient = "+14155550199"
	smsDelivery, err := worker.processEvent(smsEvt, ChannelSMS)
	if err != nil {
		t.Fatalf("unexpected error processing SMS event: %v", err)
	}
	if smsDelivery.Status != StatusDelivered {
		t.Errorf("expected status DELIVERED, got %s", smsDelivery.Status)
	}
	if smsDelivery.Channel != ChannelSMS {
		t.Errorf("expected channel SMS, got %s", smsDelivery.Channel)
	}

	// Check delivery history retrieval
	deliveries := worker.GetDeliveries()
	if len(deliveries) < 2 {
		t.Errorf("expected at least 2 recorded deliveries, got %d", len(deliveries))
	}
}

func TestNotificationHTTPHandlers(t *testing.T) {
	worker := NewNotificationWorker(nil, nil)
	srv := NewServer(nil, worker)

	// 1. GET /health
	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	wHealth := httptest.NewRecorder()
	srv.handleHealth(wHealth, reqHealth)
	if wHealth.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 on health, got %d", wHealth.Code)
	}

	// 2. POST /dispatch-test
	testPayload := map[string]interface{}{
		"eventType":    "booking.confirmed",
		"recipient":    "alice@example.com",
		"customerName": "Alice Smith",
		"eventTitle":   "Live Stadium Concert",
		"bookingRef":   "REF-STADIUM-001",
		"seats":        []string{"A1", "A2"},
	}
	payloadBytes, _ := json.Marshal(testPayload)
	reqPost := httptest.NewRequest(http.MethodPost, "/dispatch-test", bytes.NewReader(payloadBytes))
	reqPost.Header.Set("Content-Type", "application/json")
	wPost := httptest.NewRecorder()
	srv.handleDispatchTest(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on dispatch-test, got %d (body: %s)", wPost.Code, wPost.Body.String())
	}

	// 3. GET /deliveries
	reqGet := httptest.NewRequest(http.MethodGet, "/deliveries", nil)
	wGet := httptest.NewRecorder()
	srv.handleDeliveries(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on deliveries, got %d", wGet.Code)
	}

	var deliveries []NotificationDelivery
	if err := json.NewDecoder(wGet.Body).Decode(&deliveries); err != nil {
		t.Fatalf("failed to decode deliveries: %v", err)
	}
	if len(deliveries) == 0 {
		t.Errorf("expected deliveries list to have records, got 0")
	}
}
