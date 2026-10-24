package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleNotificationsValidation(t *testing.T) {
	s := &Server{db: nil}

	// 1. Method Not Allowed on /api/notifications
	req := httptest.NewRequest(http.MethodDelete, "/api/notifications", nil)
	rr := httptest.NewRecorder()
	s.handleNotifications(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
	}

	// 2. Missing email and userId query params
	req = httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	rr = httptest.NewRecorder()
	s.handleNotifications(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when no user filter provided, got %d", rr.Code)
	}
}

func TestHandleNotificationsEndpoints(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		t.Skip("Postgres not available, skipping db integration test")
		return
	}

	// Ensure table exists for test
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS notifications (
			id VARCHAR(64) PRIMARY KEY,
			user_id VARCHAR(64),
			recipient_email VARCHAR(255) NOT NULL,
			type VARCHAR(64) NOT NULL,
			title VARCHAR(255) NOT NULL,
			message TEXT NOT NULL,
			data JSONB DEFAULT '{}'::jsonb,
			is_read BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("failed to create test notifications table: %v", err)
	}

	s := &Server{db: db}

	testEmail := fmt.Sprintf("notif.test.%d@example.com", time.Now().UnixNano())

	// 1. Create a notification
	createPayload, _ := json.Marshal(map[string]interface{}{
		"recipientEmail": testEmail,
		"type":           "WAITLIST_PROMOTED",
		"title":          "You are off the waitlist!",
		"message":        "A seat opened up. You have 15 minutes to claim your hold.",
		"data": map[string]interface{}{
			"eventId":    "evt-106",
			"eventTitle": "Stand-Up Comedy All-Stars",
			"tier":       "Front Row",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/notifications", bytes.NewBuffer(createPayload))
	rr := httptest.NewRecorder()
	s.handleNotifications(rr, req)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("expected 201/200 on notification create, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var createdNotif Notification
	if err := json.Unmarshal(rr.Body.Bytes(), &createdNotif); err != nil {
		t.Fatalf("failed to decode created notification: %v", err)
	}
	if createdNotif.ID == "" {
		t.Fatalf("expected non-empty notification ID")
	}

	// 2. Fetch notifications for testEmail
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/notifications?email=%s", testEmail), nil)
	rr = httptest.NewRecorder()
	s.handleNotifications(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get notifications, got %d", rr.Code)
	}

	var listResp NotificationListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal list response: %v", err)
	}
	if len(listResp.Notifications) == 0 {
		t.Fatalf("expected at least 1 notification, found 0")
	}
	if listResp.UnreadCount != 1 {
		t.Errorf("expected unreadCount 1, got %d", listResp.UnreadCount)
	}

	// 3. Mark notification as read
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/notifications/%s/read", createdNotif.ID), nil)
	rr = httptest.NewRecorder()
	s.handleNotificationByID(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on mark as read, got %d, body: %s", rr.Code, rr.Body.String())
	}

	// 4. Verify unread count is now 0
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/notifications?email=%s", testEmail), nil)
	rr = httptest.NewRecorder()
	s.handleNotifications(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get notifications, got %d", rr.Code)
	}
	json.Unmarshal(rr.Body.Bytes(), &listResp)
	if listResp.UnreadCount != 0 {
		t.Errorf("expected unreadCount 0 after mark read, got %d", listResp.UnreadCount)
	}

	// 5. Test mark all as read
	// Add another notification
	s.handleNotifications(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/notifications", bytes.NewBuffer(createPayload)))
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/notifications/read-all?email=%s", testEmail), nil)
	rr = httptest.NewRecorder()
	s.handleNotificationReadAll(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on mark all read, got %d", rr.Code)
	}

	// Verify unread count is 0
	req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/notifications?email=%s", testEmail), nil)
	rr = httptest.NewRecorder()
	s.handleNotifications(rr, req)
	json.Unmarshal(rr.Body.Bytes(), &listResp)
	if listResp.UnreadCount != 0 {
		t.Errorf("expected unreadCount 0 after mark all read, got %d", listResp.UnreadCount)
	}
}

func TestCalendarICSGeneration(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		t.Skip("Postgres not available, skipping db integration test")
		return
	}

	s := &Server{db: db}

	// 1. Non-existent booking 404
	req := httptest.NewRequest(http.MethodGet, "/api/bookings/nonexistent-bkg-99999/calendar.ics", nil)
	rr := httptest.NewRecorder()
	s.handleBookingCalendarICS(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for missing booking calendar, got %d", rr.Code)
	}

	// 2. Fetch pre-seeded demo booking: bkg-demo-001 (Neon Symphony 2026)
	var existingBkgID string
	err := db.QueryRowContext(context.Background(), "SELECT id FROM bookings WHERE id = 'bkg-demo-001'").Scan(&existingBkgID)
	if err != nil || existingBkgID == "" {
		t.Skip("bkg-demo-001 not present in DB, skipping live ICS test")
		return
	}

	req = httptest.NewRequest(http.MethodGet, "/api/bookings/bkg-demo-001/calendar.ics", nil)
	rr = httptest.NewRecorder()
	s.handleBookingCalendarICS(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on calendar export, got %d, body: %s", rr.Code, rr.Body.String())
	}

	// Check headers
	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/calendar") {
		t.Errorf("expected Content-Type containing 'text/calendar', got %s", contentType)
	}
	contentDisposition := rr.Header().Get("Content-Disposition")
	if !strings.Contains(contentDisposition, "attachment") || !strings.Contains(contentDisposition, ".ics") {
		t.Errorf("expected attachment Content-Disposition with .ics, got %s", contentDisposition)
	}

	// Check RFC 5545 body components
	body := rr.Body.String()
	requiredTokens := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//TicketVerse//Event Ticketing System//EN",
		"BEGIN:VEVENT",
		"UID:bkg-demo-001@ticketverse.com",
		"SUMMARY:Neon Symphony 2026",
		"LOCATION:",
		"DTSTART:",
		"DTEND:",
		"DESCRIPTION:",
		"END:VEVENT",
		"END:VCALENDAR",
	}

	for _, token := range requiredTokens {
		if !strings.Contains(body, token) {
			t.Errorf("expected calendar ICS to contain '%s', but missing in:\n%s", token, body)
		}
	}

	// Check CRLF endings (\r\n) per RFC 5545
	if !strings.Contains(body, "\r\n") {
		t.Errorf("expected RFC 5545 CRLF line endings, but none found")
	}
}
